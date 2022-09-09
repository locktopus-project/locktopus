package main

import (
	"os"

	golog "github.com/withmandala/go-log"
)

type wrappedLogger struct {
	enabled bool
	l       *golog.Logger
}

func NewWrappedLogger() wrappedLogger {
	l := golog.New(os.Stdout)

	l.WithDebug()

	return wrappedLogger{
		enabled: true,
		l:       l,
	}
}

func (l *wrappedLogger) Disable() {
	l.enabled = false
}

func (l *wrappedLogger) Info(args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Info(args...)
}

func (l *wrappedLogger) Infof(format string, args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Infof(format, args...)
}

func (l *wrappedLogger) Warn(args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Warn(args...)
}

func (l *wrappedLogger) Warnf(format string, args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Warnf(format, args...)
}

func (l *wrappedLogger) Error(args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Error(args...)
}

func (l *wrappedLogger) Errorf(format string, args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Errorf(format, args...)
}

var mainLogger = NewWrappedLogger()
var apiLogger = NewWrappedLogger()
var lockLogger = NewWrappedLogger()
