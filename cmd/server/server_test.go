package main_test

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	main "github.com/locktopus-project/locktopus/cmd/server"
	internal "github.com/locktopus-project/locktopus/internal/utils"
)

var serverAddress = os.Getenv("SERVER_ADDRESS")

var defaultHostname = "localhost"

const connTimeoutMs = 5000
const connPollIntervalMs = 100

func TestMain(m *testing.M) {
	if serverAddress == "" {
		freePort, err := internal.FindFreePort()
		if err != nil {
			log.Fatalf("Cannot find free port: %s", err)
		}

		serverAddress = fmt.Sprintf("%s:%s", defaultHostname, freePort)

		server := main.MakeServer(main.ServerParameters{
			Hostname:              defaultHostname,
			Port:                  freePort,
			DefaultAbandonTimeout: 60 * time.Second,
		})

		main.StartListening(server)

		defer server.Close()
	}

	err := internal.EnsureServerAvailability(fmt.Sprintf("http://%s", serverAddress), connTimeoutMs, connPollIntervalMs)
	if err != nil {
		log.Fatalf("Cannot ensure server availability: %s", err)
	}

	m.Run()
}
