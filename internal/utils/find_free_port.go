package internal

import (
	"net"
	"strings"
)

func FindFreePort() (string, error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}

	ln.Close()

	addr := ln.Addr().String()
	s := strings.Split(addr, ":")

	port := s[len(s)-1]

	return port, nil
}
