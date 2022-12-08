package main_test

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/locktopus-project/locktopus/internal/constants"
	locktopusclient "github.com/locktopus-project/locktopus/pkg/client/v1"
)

const v1NamespaceName = "v1_namespace"

func TestClient_MakeLocktopusClient_ByUrl(t *testing.T) {
	client, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%v=123&%s=1000", serverAddress, constants.NamespaceQueryParameterName, constants.AbandonTimeoutQueryParameterName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	client.Close()
}

func TestClient_MakeLocktopusClient_ByParams(t *testing.T) {
	serverHost := strings.Split(serverAddress, ":")[0]
	serverPort := strings.Split(serverAddress, ":")[1]

	portNumber, err := strconv.Atoi(serverPort)
	if err != nil {
		t.Fatalf("cannot convert port number to int: %s", err)
	}

	timeoutMs := 1000

	client, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Namespace:           "123",
		Host:                serverHost,
		Port:                portNumber,
		Secure:              false,
		ForceCloseTimeoutMs: &timeoutMs,
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	client.Close()
}

func TestClient_ImmediateAcquireOnLock(t *testing.T) {
	client, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	client.AddLockResource(locktopusclient.LockTypeWrite, "test1")

	err = client.Lock()
	if err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if !client.IsAcquired() {
		t.Fatalf("client should have acquired the lock")
	}

	client.Close()
}

func TestClient_LockID(t *testing.T) {
	client, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	client.AddLockResource(locktopusclient.LockTypeWrite, "testLockId")

	err = client.Lock()
	if err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if !client.IsAcquired() {
		t.Fatalf("client should have acquired the lock")
	}

	if client.LockID() == "" {
		t.Fatalf("client should have lock id")
	}

	client.Close()
}

func TestClient_SequentialAcquire(t *testing.T) {
	locker, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	waiter, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test2")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test2")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if !locker.IsAcquired() {
		t.Fatalf("locker's lock should be acquired")
	}

	if err = waiter.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if waiter.IsAcquired() {
		t.Fatalf("waiter's lock should not be acquired")
	}

	if err = locker.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
	}

	if err = waiter.Acquire(); err != nil {
		t.Fatalf("cannot acquire: %s", err)
	}

	if !waiter.IsAcquired() {
		t.Fatalf("waiter's lock should be acquired")
	}

	locker.Close()
	waiter.Close()
}

func TestClient_AfterReleaseShouldLock(t *testing.T) {
	locker, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test4")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if !locker.IsAcquired() {
		t.Fatalf("locker's lock should be acquired")
	}

	if err = locker.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
	}

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if !locker.IsAcquired() {
		t.Fatalf("locker's lock should be acquired")
	}

	locker.Close()
}

func TestClient_ReleaseWhenAcquiring_AcquireFirst(t *testing.T) {
	locker, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	waiter, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test5")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test5")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if err = waiter.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	ch := make(chan error)
	go func() {
		ch <- waiter.Acquire()
	}()

	runtime.Gosched()
	if err = waiter.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
	}

	if err = <-ch; err == nil {
		t.Fatalf("Expected error in acquire()")
	}

	if !errors.Is(err, locktopusclient.ErrReleasedBeforeAcquired) {
		t.Fatalf("Expected ErrReleasedBeforeAcquired error")
	}

	locker.Close()
	waiter.Close()
}

func TestClient_ReleaseWhenAcquiring_ReleaseFirst(t *testing.T) {
	locker, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	waiter, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test6")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test6")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if err = waiter.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	if err = waiter.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
	}

	err = waiter.Acquire()
	if err == nil {
		t.Fatalf("Expected error in acquire()")
	}

	if !errors.Is(err, locktopusclient.ErrReleasedBeforeAcquired) {
		t.Fatalf("Expected ErrReleasedBeforeAcquired error")
	}

	locker.Close()
	waiter.Close()
}

func TestClient_ClosedConnectionTimeout_BeforeLock(t *testing.T) {
	timeoutMs := 1000

	locker, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s&%s=%s", serverAddress, constants.NamespaceQueryParameterName, constants.AbandonTimeoutQueryParameterName, strconv.Itoa(timeoutMs), v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	waiter, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test7")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test7")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	err = locker.Acquire()
	if err != nil {
		t.Fatalf("cannot acquire")
	}

	err = locker.Release()
	if err != nil {
		t.Fatalf("cannot release")
	}

	locker.Close()

	now := time.Now()

	if err = waiter.Lock(); err != nil {
		t.Fatalf("lock should be possible")
	}

	err = waiter.Acquire()
	if err != nil {
		t.Fatalf("cannot acquire")
	}

	if time.Since(now) >= time.Duration(timeoutMs)*time.Millisecond {
		t.Fatalf("waiter should not wait for timeout")
	}

	err = waiter.Release()
	if err != nil {
		t.Fatalf("cannot release")
	}

	waiter.Close()
}

func TestClient_ClosedConnectionTimeout_AfterLock(t *testing.T) {
	timeoutMs := 100

	locker, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s&%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName, constants.AbandonTimeoutQueryParameterName, strconv.Itoa(timeoutMs)),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	waiter, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test8")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test8")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	err = locker.Acquire()
	if err != nil {
		t.Fatalf("cannot acquire")
	}

	locker.Close()

	now := time.Now()

	if err = waiter.Lock(); err != nil {
		t.Fatalf("lock should be possible")
	}

	err = waiter.Acquire()
	if err != nil {
		t.Fatalf("cannot acquire")
	}

	if time.Since(now) < time.Duration(timeoutMs)*time.Millisecond {
		t.Fatalf("waiter should wait for timeout")
	}

	waiter.Close()
}

func TestClient_ClosedConnectionTimeout_AfterRelease(t *testing.T) {
	timeoutMs := 100

	locker, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s&%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName, constants.AbandonTimeoutQueryParameterName, strconv.Itoa(timeoutMs)),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	waiter, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, v1NamespaceName),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test9")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test9")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
	}

	err = locker.Acquire()
	if err != nil {
		t.Fatalf("cannot acquire")
	}

	err = locker.Release()
	if err != nil {
		t.Fatalf("cannot release")
	}

	locker.Close()

	now := time.Now()

	if err = waiter.Lock(); err != nil {
		t.Fatalf("lock should be possible")
	}

	err = waiter.Acquire()
	if err != nil {
		t.Fatalf("cannot acquire")
	}

	if time.Since(now) > time.Duration(timeoutMs)*time.Millisecond {
		t.Fatalf("waiter should not wait for timeout")
	}

	waiter.Close()
}
