package gearlockclient

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	ml "github.com/xshkut/gearlock/pkg/multilocker"
)

// GearlockClient is a client for Gearlock server. Use MakeGearlockClient to instantiate one and connect.
type GearlockClient struct {
	conn     *websocket.Conn
	lr       []resource
	acquired bool
	lockID   string
}

type ConnectionOptions struct {
	Url       string // if provided, other options are ignored
	Host      string
	Port      int
	Namespace string
	Secure    bool
}

type LockType = ml.LockType

const (
	LockTypeRead  = ml.LockTypeRead
	LockTypeWrite = ml.LockTypeWrite
)

const version = "v1"

// MakeGearlockClient establishes a connection to the Gearlock server and returns GearlockClient.
func MakeGearlockClient(options ConnectionOptions) (*GearlockClient, error) {
	url := options.Url

	if url == "" {
		switch true {
		case options.Host == "":
			return nil, fmt.Errorf("host is required")
		case options.Port == 0:
			return nil, fmt.Errorf("port is required")
		case options.Namespace == "":
			return nil, fmt.Errorf("namespace is required")
		}

		s := ""
		if options.Secure {
			s = "s"
		}

		url = fmt.Sprintf("ws%s://%s:%d/%s?namespace=%s", s, options.Host, options.Port, version, options.Namespace)
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot dial to Gearlock server: %s", err)
	}

	return &GearlockClient{
		conn: conn,
	}, nil
}

const closeMessage = "close"

// Close us used to close the connection when it is not needed anymore or after an error.
func (c *GearlockClient) Close() error {
	err := c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, closeMessage), time.Now().Add(time.Second))
	if err != nil {
		return fmt.Errorf("cannot write close message: %s", err)
	}

	return c.conn.Close()

}

// AddLockResource adds resources to be used and flushed within next Lock() call.
func (c *GearlockClient) AddLockResource(lockType LockType, resources ...string) {
	lr := ml.NewResourceLock(lockType, resources)

	c.lr = append(c.lr, resource{
		T:    lr.LockType.String(),
		Path: lr.Path,
	})
}

// Lock locks added resources. Use IsAcquired() to check if lock has been acquired.
func (c *GearlockClient) Lock() (err error) {
	var response responseMessage
	msg := requestMessage{
		Action:    actionLock,
		Resources: c.lr,
	}

	err = c.conn.WriteJSON(msg)
	if err != nil {
		return fmt.Errorf("cannot write request: %s", err)
	}

	if err = c.conn.ReadJSON(&response); err != nil {
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
func (c *GearlockClient) IsAcquired() bool {
	return c.acquired
}

func (c *GearlockClient) LockID() string {
	return c.lockID
}

// Acquire is used to wait until the lock is acquired. If IsAcquired() returns true after calling Lock(), calling Acquire() is no-op.
func (c *GearlockClient) Acquire() (err error) {
	var response responseMessage

	if c.acquired {
		return nil
	}

	if err = c.conn.ReadJSON(&response); err != nil {
		return fmt.Errorf("cannot read response: %s", err)
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
func (c *GearlockClient) Release() (err error) {
	var response responseMessage
	msg := requestMessage{
		Action: actionRelease,
	}

	if err = c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("cannot write request: %s", err)
	}

	if err = c.conn.ReadJSON(&response); err != nil {
		return fmt.Errorf("cannot read response: %s", err)
	}

	if response.Action == actionLock && response.State == "acquired" && response.ID == c.lockID {
		// This is the response to the previous Lock() call and should be ignored.
		if err = c.conn.ReadJSON(&response); err != nil {
			return fmt.Errorf("cannot read response: %s", err)
		}
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
