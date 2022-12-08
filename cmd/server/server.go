package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	// internal
	ns "github.com/locktopus-project/locktopus/internal/namespace"
)

const numberOfPosixSignals = 28

type apiHandler struct {
	version        string
	handler        func(w http.ResponseWriter, r *http.Request, defaultAbandonTimeout time.Duration)
	connStrExample string
}

func main() {
	parseArguments()

	server := MakeServer(
		ServerParameters{
			Hostname:              hostname,
			Port:                  port,
			DefaultAbandonTimeout: defaultAbandonTimeout,
		},
	)
	listenErr := StartListening(server)

	exitCode := 0

	select {
	case err := <-listenErr:
		mainLogger.Error(fmt.Errorf("HTTP listener error: %w", err))
		exitCode = 1
	case s := <-getSignals():
		mainLogger.Infof("Received signal: %s", s)
	}

	mainLogger.Info("Waiting for existing locks to be released... Send SIGINT or SIGTERM again to force exit")
	select {
	case <-ns.CloseNamespaces():
		mainLogger.Info("All namespaces have been closed")
	case s := <-getSignals():
		mainLogger.Infof("Received signal: %s", s)
		exitCode = 1
	}

	mainLogger.Info("Closing HTTP server...")
	server.Close()

	mainLogger.Info("Exit with code", exitCode)
}

func getSignals() <-chan os.Signal {
	ch := make(chan os.Signal, numberOfPosixSignals)

	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	return ch
}

type ServerParameters struct {
	Hostname              string
	Port                  string
	DefaultAbandonTimeout time.Duration
}

func MakeServer(params ServerParameters) http.Server {
	hostname := params.Hostname
	port := params.Port
	defaultAbandonTimeout := params.DefaultAbandonTimeout

	r := mux.NewRouter()

	for _, apiHandler := range apiHandlers {
		ah := apiHandler

		r.HandleFunc(ah.version, func(w http.ResponseWriter, r *http.Request) {
			ah.handler(w, r, defaultAbandonTimeout)
		})
	}

	r.HandleFunc("/", greetingsHandler)

	server := http.Server{
		Addr:         fmt.Sprintf("%s:%s", hostname, port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      handlers.CORS()(r),
	}

	return server
}

var apiHandlers = []apiHandler{
	{
		version:        "/v1",
		handler:        apiV1Handler,
		connStrExample: "ws://host:port/v1?namespace=default",
	},
	{
		version:        "/stats_v1",
		handler:        statsV1Handler,
		connStrExample: "http://host:port/stats_v1?namespace=default",
	},
}

var lastConnID int64 = -1

func StartListening(server http.Server) <-chan error {
	if statInterval > 0 {
		go func() {
			for {
				time.Sleep(time.Duration(statInterval) * time.Second)

				statsList := ns.GetStatistics()

				for _, stats := range statsList {
					mainLogger.Infof("Multilocker namespace: %s. Statistics: %+v", stats.Name, stats.Stats)
				}
			}
		}()
	}

	mainLogger.Info("Starting listening on", server.Addr)

	ch := make(chan error)

	go func() {
		ch <- fmt.Errorf("HTTP listener error: %w", server.ListenAndServe())
	}()

	return ch
}

func greetingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Welcome to LOCKTOPUS server!\n\nAvailable API versions: \n"))
	for _, apiHandler := range apiHandlers {
		w.Write([]byte(fmt.Sprintf("%s\te.g. %s\n", apiHandler.version, apiHandler.connStrExample)))
	}

	w.Write([]byte("\nServer parameters:\n"))
	w.Write([]byte(fmt.Sprintf("Default abandon timeout: %s\n", hostname)))

	namespaces := ns.GetNamespaces()
	if len(namespaces) == 0 {
		w.Write([]byte("\nNo opened namespaces\n"))
		return
	}

	w.Write([]byte("\nOpened namespaces:\n"))
	for _, ns := range namespaces {
		w.Write([]byte(fmt.Sprintf("%s\n", ns.Name)))
	}
}

func wait(seconds int) <-chan struct{} {
	ch := make(chan struct{})

	if seconds > 0 {
		go func() {
			time.Sleep(time.Duration(seconds) * time.Second)
			close(ch)
		}()
	}

	return ch
}
