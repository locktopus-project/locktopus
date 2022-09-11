package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"

	internal "github.com/xshkut/gearlock/internal/utils"
	gearlockclient "github.com/xshkut/gearlock/pkg/gearlock_client/v1"
)

var serverHost = os.Getenv("SERVER_HOST")
var serverPort = os.Getenv("SERVER_PORT")

const connTimeoutMs = 5000
const connPollIntervalMs = 100

const statsNamespaceName = "stats_namespace"

func init() {
	if serverHost == "" {
		serverHost = "localhost"
	}

	if serverPort == "" {
		serverPort = "9009"
	}
}

func TestMain(m *testing.M) {
	err := internal.CheckServerAvailability(fmt.Sprintf("http://%s:%s", serverHost, serverPort), connTimeoutMs, connPollIntervalMs)
	if err != nil {
		log.Fatalf("Cannot ensure server availability: %s", err)
	}

	m.Run()
}

func TestStats_BeforeInitiatingNamespace(t *testing.T) {
	url := fmt.Sprintf("http://%s:%s/stats_v1?namespace=%s", serverHost, serverPort, statsNamespaceName)

	// make http get request to url
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("cannot query Gearlock server: %s", err)
		return
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Status code is not 404")
		return
	}
}

func TestStats_AfterInitiatingNamespace(t *testing.T) {
	url := fmt.Sprintf("http://%s:%s/stats_v1?namespace=%s", serverHost, serverPort, statsNamespaceName)

	_, err := gearlockclient.MakeGearlockClient(gearlockclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s:%s/v1?namespace=%s", serverHost, serverPort, statsNamespaceName),
	})
	if err != nil {
		t.Fatalf("cannot connect to Gearlock server: %s", err)
		return
	}

	// make http get request to url
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("cannot query Gearlock server: %s", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status code is not 200")
		return
	}

	defer resp.Body.Close()

	// check for successfully parsing json
	err = json.NewDecoder(resp.Body).Decode(&map[string]interface{}{})
	if err != nil {
		t.Fatalf("cannot parse response body: %s", err)
		return
	}
}
