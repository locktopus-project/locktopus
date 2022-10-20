package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"testing"

	internal "github.com/locktopus-project/locktopus/internal/utils"
	locktopusclient "github.com/locktopus-project/locktopus/pkg/client/v1"
)

var serverHost = os.Getenv("SERVER_HOST")
var serverPort = os.Getenv("SERVER_PORT")

const connTimeoutMs = 5000
const connPollIntervalMs = 100

func TestMain(m *testing.M) {
	err := internal.CheckServerAvailability(fmt.Sprintf("http://%s:%s", serverHost, serverPort), connTimeoutMs, connPollIntervalMs)
	if err != nil {
		log.Fatalf("Cannot ensure server availability: %s", err)
	}

	m.Run()
}

func TestClient_MakeLocktopusClient_ByUrl(t *testing.T) {
	client, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	client.Close()
}

func TestClient_MakeLocktopusClient_ByParams(t *testing.T) {
	portNumber, err := strconv.Atoi(serverPort)
	if err != nil {
		t.Fatalf("cannot convert port number to int: %s", err)
	}

	client, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Namespace: "123",
		Host:      serverHost,
		Port:      portNumber,
		Secure:    false,
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	client.Close()
}

func TestClient_ImmediateAcquireOnLock(t *testing.T) {
	client, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	client.AddLockResource(locktopusclient.LockTypeWrite, "test1")

	err = client.Lock()
	if err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if !client.IsAcquired() {
		t.Fatalf("client should have acquired the lock")
		return
	}

	client.Close()
}

func TestClient_LockID(t *testing.T) {
	client, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	client.AddLockResource(locktopusclient.LockTypeWrite, "testLockId")

	err = client.Lock()
	if err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if !client.IsAcquired() {
		t.Fatalf("client should have acquired the lock")
		return
	}

	if client.LockID() == "" {
		t.Fatalf("client should have lock id")
		return
	}

	client.Close()
}

func TestClient_SequentialAcquire(t *testing.T) {
	locker, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	waiter, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test2")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test2")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if !locker.IsAcquired() {
		t.Fatalf("locker's lock should be acquired")
		return
	}

	if err = waiter.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if waiter.IsAcquired() {
		t.Fatalf("waiter's lock should not be acquired")
		return
	}

	if err = locker.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
		return
	}

	if err = waiter.Acquire(); err != nil {
		t.Fatalf("cannot acquire: %s", err)
		return
	}

	if !waiter.IsAcquired() {
		t.Fatalf("waiter's lock should be acquired")
		return
	}

	locker.Close()
	waiter.Close()
}

func TestClient_CloseShouldRelease(t *testing.T) {
	locker, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	waiter, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test3")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test3")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if !locker.IsAcquired() {
		t.Fatalf("locker's lock should be acquired")
		return
	}

	if err = waiter.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if waiter.IsAcquired() {
		t.Fatalf("waiter's lock should not be acquired")
		return
	}

	locker.Close()

	if err = waiter.Acquire(); err != nil {
		t.Fatalf("cannot acquire: %s", err)
		return
	}

	if !waiter.IsAcquired() {
		t.Fatalf("waiter's lock should be acquired")
		return
	}

	waiter.Close()
}

func TestClient_AfterReleaseShouldLock(t *testing.T) {
	locker, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test4")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if !locker.IsAcquired() {
		t.Fatalf("locker's lock should be acquired")
		return
	}

	if err = locker.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
		return
	}

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if !locker.IsAcquired() {
		t.Fatalf("locker's lock should be acquired")
		return
	}

	locker.Close()
}

func TestClient_ReleaseWhenAcquiring_AcquireFirst(t *testing.T) {
	locker, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	waiter, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test5")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test5")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if err = waiter.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	ch := make(chan error)
	go func() {
		ch <- waiter.Acquire()
	}()

	runtime.Gosched()
	if err = waiter.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
		return
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
	locker, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	waiter, err := locktopusclient.MakeLocktopusClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	locker.AddLockResource(locktopusclient.LockTypeWrite, "test6")
	waiter.AddLockResource(locktopusclient.LockTypeWrite, "test6")

	if err = locker.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if err = waiter.Lock(); err != nil {
		t.Fatalf("cannot lock: %s", err)
		return
	}

	if err = waiter.Release(); err != nil {
		t.Fatalf("cannot release: %s", err)
		return
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
