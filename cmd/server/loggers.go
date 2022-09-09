package main

import (
	logger "github.com/xshkut/gearlock/internal/logger"
)

var mainLogger = logger.NewLogger()
var apiLogger = logger.NewLogger()
var lockLogger = logger.NewLogger()
