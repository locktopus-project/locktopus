package client

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/locktopus-project/locktopus/internal/constants"
	ml "github.com/locktopus-project/locktopus/pkg/multilocker"
)

// LocktopusClient is a client for Locktopus server. Use MakeLocktopusClient to instantiate one and connect.
type LocktopusClient struct {
	conn      *websocket.Conn
	lr        []resource
	acquired  bool
	lockID    string
	responses chan result
	released  chan struct{}
}

type ConnectionOptions struct {
	Url                 string // if provided, other options are ignored
	Host                string
	Port                int
	Namespace           string
	Secure              bool
	ForceCloseTimeoutMs *int // if provided, server will keep the lock for this time after client disconnects without releasing it
}

type LockType = ml.LockType

const (
	LockTypeRead  = ml.LockTypeRead
	LockTypeWrite = ml.LockTypeWrite
)

const version = "v1"

// MakeClient establishes a connection to the Locktopus server and returns LocktopusClient.
func MakeClient(options ConnectionOptions) (*LocktopusClient, error) {
	address := options.Url

	if address == "" {
		switch true {
		case options.Host == "":
			return nil, fmt.Errorf("parameter Host is required")
		case options.Port == 0:
			return nil, fmt.Errorf("parameter Port is required")
		case options.Namespace == "":
			return nil, fmt.Errorf("parameter Namespace is required")
		}

		s := ""
		if options.Secure {
			s = "s"
		}

		values := url.Values{}
		values.Set("namespace", options.Namespace)

		if options.ForceCloseTimeoutMs != nil {
			values.Set(constants.AbandonTimeoutQueryParameterName, fmt.Sprintf("%d", *options.ForceCloseTimeoutMs))
		}

		fmt.Sprintln(values.Encode())

		address = fmt.Sprintf("ws%s://%s:%d/%s?%s", s, options.Host, options.Port, version, values.Encode())
	}

	conn, r, err := websocket.DefaultDialer.Dial(address, nil)
	if err != nil {
		body, readErr := ioutil.ReadAll(r.Body)
		if readErr != nil {
			err = fmt.Errorf("cannot read response body after handshake error: %w", err)
		} else {
			err = fmt.Errorf("handshake error: %s", string(body))
		}

		return nil, err
	}

	lc := LocktopusClient{
		conn: conn,
	}

	lc.responses = make(chan result)
	go lc.readResponses(lc.responses)

	lc.released = make(chan struct{}, 1)

	return &lc, nil
}

const closeMessage = "close"

func (c *LocktopusClient) Close() error {
	err := c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, closeMessage), time.Now().Add(time.Second))
	if err != nil {
		return fmt.Errorf("cannot write close message: %s", err)
	}

	return c.conn.Close()
}

// AddLockResource adds resources to be used and flushed within next Lock() call.
func (c *LocktopusClient) AddLockResource(lockType LockType, resources ...string) {
	lr := ml.NewResourceLock(lockType, resources)

	c.lr = append(c.lr, resource{
		T:    lr.LockType.String(),
		Path: lr.Path,
	})
}

// Lock locks added resources. Use IsAcquired() to check if lock has been acquired.
func (c *LocktopusClient) Lock() (err error) {
	select {
	case <-c.released:
	default:
	}

	var response responseMessage
	msg := requestMessage{
		Action:    actionLock,
		Resources: c.lr,
	}

	err = c.conn.WriteJSON(msg)
	if err != nil {
		return fmt.Errorf("cannot write request: %s", err)
	}

	res := <-c.responses
	response = res.data
	err = res.err
	if err != nil {
		return fmt.Errorf("cannot read response: %s", err)
	}

	if response.State == "ready" {
		return fmt.Errorf("unexpected state 'ready' returned from server after Lock()")
	}

	if response.Action != actionLock {
		return fmt.Errorf("unexpected response action returned from server: %s", response.Action)
	}

	c.acquired = response.State == "acquired"
	c.lockID = response.ID

	return nil
}

// IsAcquired returns true if last Lock() has been acquired, so there is no need to call Acquire()
func (c *LocktopusClient) IsAcquired() bool {
	return c.acquired
}

func (c *LocktopusClient) LockID() string {
	return c.lockID
}

var ErrReleasedBeforeAcquired = errors.New("cannot release lock before it has been locked")
var ErrUnexpectedResponse = errors.New("unexpected response")

// Acquire is used to wait until the lock is acquired. If IsAcquired() returns true after calling Lock(), calling Acquire() is no-op.
func (c *LocktopusClient) Acquire() (err error) {
	var response responseMessage

	if c.acquired {
		return nil
	}

	var res result
	select {
	case res = <-c.responses:
	case <-c.released:
		return ErrReleasedBeforeAcquired
	}

	response = res.data
	err = res.err
	if err != nil {
		return fmt.Errorf("cannot read response: %s", err)
	}

	if response.ID != c.lockID {
		return ErrUnexpectedResponse
	}

	if response.State == "released" && response.Action == "release" {
		return ErrUnexpectedResponse
	}

	if response.State != "acquired" {
		return fmt.Errorf("unexpected state '%s' returned from server when waiting for acquire", response.State)
	}

	if response.Action != actionLock {
		return fmt.Errorf("unexpected response action returned from server: %s", response.Action)
	}

	c.acquired = true

	return nil
}

// Release releases the lock. After that you may call AddLockResource() and Lock() again.
func (c *LocktopusClient) Release() (err error) {
	c.released <- struct{}{}

	var response responseMessage
	msg := requestMessage{
		Action: actionRelease,
	}

	if err = c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("cannot write request: %s", err)
	}

	res := <-c.responses
	if res.err != nil {
		return fmt.Errorf("cannot read response: %s", res.err)
	}

	response = res.data

	if response.Action == actionLock && response.State == "acquired" && response.ID == c.lockID {
		// This is the response to the previous Lock() call and should be ignored.
		res := <-c.responses
		if res.err != nil {
			return fmt.Errorf("cannot read response: %s", res.err)
		}

		response = res.data
	}

	if response.State != "ready" {
		return fmt.Errorf("unexpected state '%s' returned from server when waiting for release", response.State)
	}

	if response.Action != actionRelease {
		return fmt.Errorf("unexpected response action returned from server: %s", response.Action)
	}

	c.acquired = false

	return nil
}

type result struct {
	data responseMessage
	err  error
}

func (c *LocktopusClient) readResponses(ch chan<- result) {
	var response responseMessage
	var err error

	for {
		if err = c.conn.ReadJSON(&response); err != nil {
			err = fmt.Errorf("cannot read JSON message: %s", err)
		}

		ch <- result{
			data: response,
			err:  err,
		}

		if err != nil {
			break
		}
	}
}
