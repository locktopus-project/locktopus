package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	internal "github.com/xshkut/distributed-lock/internal/utils"
)

// handle --port, -p argument
var port = flag.String("port", "8080", "port to listen")

func init() {
	flag.Parse()

	if *port == "" {
		flag.Usage()
		os.Exit(1)
	}
}

type apiHandler struct {
	version string
	handler func(w http.ResponseWriter, r *http.Request)
}

func main() {
	err := listenHTTP(*port)
	if err != nil {
		internal.WrapErrorPrint(err, "listening failed")
	}
}

var apiHandlers = []apiHandler{
	{
		version: "/v1",
		handler: apiV1Handler,
	},
}

func listenHTTP(port string) (err error) {
	r := mux.NewRouter()

	for _, apiHandler := range apiHandlers {
		r.HandleFunc(apiHandler.version, apiHandler.handler)
	}

	r.HandleFunc("/", greetingsHandler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	fmt.Println("Listening on port", port)

	return internal.WrapError(server.ListenAndServe(), "HTTP listener error")
}

var documentationLink = os.Getenv("DOCUMENTATION_LINK")

func greetingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Welcome to GearLock service!\n\nAvailable API versions: \n"))
	for _, apiHandler := range apiHandlers {
		w.Write([]byte(fmt.Sprintf("%s\n", apiHandler.version)))
	}

	if documentationLink != "" {
		w.Write([]byte(fmt.Sprintf("\nDocs: %s\n", documentationLink)))
	}
}
