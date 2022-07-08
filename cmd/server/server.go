package main

import (
	"os"

	internal "github.com/xshkut/distributed-lock/internal/utils"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	err := internal.ListenHttp(port)
	if err != nil {
		internal.WrapErrorPrint(err, "listening failed")
	}
}
