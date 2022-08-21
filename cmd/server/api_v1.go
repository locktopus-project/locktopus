package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	internal "github.com/xshkut/gearlock/internal/utils"

	ml "github.com/xshkut/gearlock/pkg/multilocker"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const invalidInput = 3000

func apiV1Handler(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")

	if namespace == "" {
		w.Write([]byte("URL parameter 'namespace' is required"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		internal.WrapErrorPrint(err, "upgrade error")
		return
	}
	defer conn.Close()

	internal.Logger.Infof("New connection from %s", conn.RemoteAddr())

	err = handleCommunication(conn, getMultilockerInstance(namespace))

	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(internal.WrapError(err, "Communication error").Error()))
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(invalidInput, ""), time.Now().Add(time.Second))

		internal.Logger.Infof("Connection errored: %s", err)

		return
	}

	internal.Logger.Infof("Closing errored connection: %s", err)

	conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
}

type action string

const (
	actionLock    action = "lock"
	actionRelease action = "release"
)

type requestMessage struct {
	Action    action     `json:"action"` // enqueue, unlock
	Resources []resource `json:"resources,omitempty"`
}

type resource struct {
	T    string   `json:"type"`
	Path []string `json:"path"`
}

type responseMessage struct {
	ID     string `json:"id"`
	Action action `json:"action"` // enqueue, acquire, unlock
	State  string `json:"state"`
}

type ClientState int

const (
	clientStateEnqueued ClientState = iota
	clientStateAcquired
	clientStateReady
)

var states = [3]string{"enqueued", "acquired", "ready"}

func (cs ClientState) String() string {
	return states[cs]
}

func handleCommunication(conn *websocket.Conn, ls *ml.MultiLocker) (err error) {
	var l *ml.Lock
	state := clientStateReady
	ch := make(chan requestMessage)

	var readErr error
	go func() {
		for {
			cm := &requestMessage{}

			if readErr = conn.ReadJSON(cm); readErr != nil {
				close(ch)
				break
			}

			ch <- *cm
		}
	}()

	for {
		incm := requestMessage{}
		opened := true
		received := false

		if state == clientStateEnqueued {
			select {
			case <-l.Ready():
				state = clientStateAcquired

				if err != conn.WriteJSON(responseMessage{ID: fmt.Sprintf("%d", l.ID()), Action: actionLock, State: state.String()}) {
					err = internal.WrapError(err, "Cannot send JSON message")
				}

			case incm, opened = <-ch:
				received = true
			}
		}

		if !received {
			incm, opened = <-ch
			if !opened {
				break
			}
		}

		if !opened || err != nil || readErr != nil {
			break
		}

		if err = assertCorrectAction(incm.Action, state); err != nil {
			break
		}

		if incm.Action == actionLock {
			var resourceLocks []ml.ResourceLock
			resourceLocks, err = makeResourceLocks(incm.Resources)
			if err != nil {
				break
			}

			newLock := ls.Lock(resourceLocks)
			l = &newLock

			select {
			case <-l.Ready():
				state = clientStateAcquired
			default:
				state = clientStateEnqueued
			}

			if err != conn.WriteJSON(responseMessage{ID: fmt.Sprintf("%d", l.ID()), Action: incm.Action, State: state.String()}) {
				err = internal.WrapError(err, "Cannot send JSON message")
				break
			}

			continue
		}

		// actionRelease

		go l.Acquire().Unlock()

		state = clientStateReady

		if err != conn.WriteJSON(responseMessage{ID: fmt.Sprintf("%d", l.ID()), Action: incm.Action, State: state.String()}) {
			err = internal.WrapError(err, "Cannot send JSON message")
			break
		}

	}

	if l != nil {
		l.Acquire().Unlock()
	}

	if readErr != nil {
		err = readErr
	}

	return err
}

func parseLockType(input string) (ml.LockType, error) {
	lt := ml.LockTypeRead

	switch strings.ToLower(input) {
	case "r":
		lt = ml.LockTypeRead
	case "read":
		lt = ml.LockTypeRead
	case "w":
		lt = ml.LockTypeWrite
	case "write":
		lt = ml.LockTypeWrite
	default:
		return ml.LockTypeRead, fmt.Errorf("invalid lock type: %s", input)
	}

	return lt, nil
}

func makeResourceLocks(resources []resource) ([]ml.ResourceLock, error) {
	resourceLocks := make([]ml.ResourceLock, len(resources))

	for i, r := range resources {
		lt, err := parseLockType(r.T)
		if err != nil {
			return nil, internal.WrapError(err, "Cannot build resource lock")
		}

		resourceLocks[i] = ml.NewResourceLock(lt, r.Path)
	}

	return resourceLocks, nil
}

func assertCorrectAction(action action, state ClientState) error {
	if action != actionLock && action != actionRelease {
		return fmt.Errorf("invalid action: %s", action)
	}

	if action == actionLock && state == clientStateReady {
		return nil
	}

	if action == actionRelease && state != clientStateReady {
		return nil
	}

	return fmt.Errorf("invalid action [%s] in state [%s]", action, state)
}
