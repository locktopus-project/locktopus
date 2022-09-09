package logger

import (
	"os"

	golog "github.com/withmandala/go-log"
)

type Logger struct {
	enabled bool
	l       *golog.Logger
}

func NewLogger() Logger {
	l := golog.New(os.Stdout)

	l.WithDebug()

	return Logger{
		enabled: true,
		l:       l,
	}
}

func (l *Logger) Disable() {
	l.enabled = false
}

func (l *Logger) Info(args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Info(args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Infof(format, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Warn(args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Warnf(format, args...)
}

func (l *Logger) Error(args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Error(args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	if !l.enabled {
		return
	}

	l.l.Errorf(format, args...)
}
