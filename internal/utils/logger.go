package internal

import (
	"os"

	"github.com/withmandala/go-log"
)

var Logger *log.Logger = log.New(os.Stdout)
