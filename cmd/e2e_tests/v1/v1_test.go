package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"

	internal "github.com/xshkut/gearlock/internal/utils"
	gearlockclient "github.com/xshkut/gearlock/pkg/gearlock_client/v1"
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

func TestClient_MakeGearlockClient_ByUrl(t *testing.T) {
	client, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	client.Close()
}

func TestClient_MakeGearlockClient_ByParams(t *testing.T) {
	portNumber, err := strconv.Atoi(serverPort)
	if err != nil {
		t.Fatalf("cannot convert port number to int: %s", err)
	}

	client, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Namespace: "123",
		Host:      serverHost,
		Port:      portNumber,
		Version:   "v1",
		Secure:    false,
	})

	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	client.Close()
}

func TestClient_ImmediateAcquireOnLock(t *testing.T) {
	client, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	client.AddLockResource(gearlockclient.LockTypeWrite, "test1")

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

func TestClient_SequentialAcquire(t *testing.T) {
	locker, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	waiter, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	locker.AddLockResource(gearlockclient.LockTypeWrite, "test2")
	waiter.AddLockResource(gearlockclient.LockTypeWrite, "test2")

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
	locker, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	waiter, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=123", serverHost, serverPort),
	})

	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	locker.AddLockResource(gearlockclient.LockTypeWrite, "test3")
	waiter.AddLockResource(gearlockclient.LockTypeWrite, "test3")

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