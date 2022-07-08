package internal

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

func ListenHttp(port string) (err error) {
	http.HandleFunc("/", HTTPHandler)

	err = http.ListenAndServe(fmt.Sprintf(":%s", port), nil)

	return WrapError(err, "ListenAndServe failed")
}

var upgrader = websocket.Upgrader{}

func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		WrapErrorPrint(err, "upgrade error")
		return
	}
	defer conn.Close()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println(err)
			return
		}
	}
}
