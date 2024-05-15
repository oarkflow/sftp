// Package oarklog provides a No-Operation logger
package oarklog

import (
	"fmt"
	"os"
	"time"
	
	oarkLog "github.com/oarkflow/log"
	
	"github.com/oarkflow/sftp/pkg/log"
)

// New creates a no-op logger
func New(logr oarkLog.Logger) log.Logger {
	return &OarkLog{
		logger: logr,
	}
}

// Default creates a no-op logger
func Default() log.Logger {
	w := []oarkLog.Writer{
		// &oarkLog.ConsoleWriter{ColorOutput: true, EndWithMessage: true},
		&oarkLog.IOWriter{Writer: os.Stdout},
	}
	writer := oarkLog.MultiEntryWriter(w)
	oarkLog.DefaultLogger.Writer = &writer
	oarkLog.DefaultLogger.EnableTracing = false
	oarkLog.DefaultLogger.TimeLocation = time.UTC
	oarkLog.DefaultLogger.TimeFormat = time.RFC3339
	return &OarkLog{
		logger: oarkLog.DefaultLogger,
	}
}

type OarkLog struct {
	logger oarkLog.Logger
}

func addLog(event *oarkLog.Entry, msg string, keyvals ...interface{}) {
	addEvents(event, keyvals...).Msg(msg)
}

// Debug logs key-values at debug level
func (logger *OarkLog) Debug(msg string, keyvals ...interface{}) {
	addLog(logger.logger.Debug(), msg, keyvals...)
}

// Info logs key-values at info level
func (logger *OarkLog) Info(msg string, keyvals ...interface{}) {
	addLog(logger.logger.Info(), msg, keyvals...)
}

// Warn logs key-values at warn level
func (logger *OarkLog) Warn(msg string, keyvals ...interface{}) {
	addLog(logger.logger.Warn(), msg, keyvals...)
}

// Error logs key-values at error level
func (logger *OarkLog) Error(msg string, keyvals ...interface{}) {
	addLog(logger.logger.Error(), msg, keyvals...)
}

func (logger *OarkLog) Panic(msg string, keyvals ...interface{}) {
	addLog(logger.logger.Panic(), msg, keyvals...)
}

// With adds key-values
func (logger *OarkLog) With(keyvals ...interface{}) log.Logger {
	event := oarkLog.With(&logger.logger)
	return New(addEvents(event, keyvals...).Copy())
}

func addEvents(event *oarkLog.Entry, keyvals ...interface{}) *oarkLog.Entry {
	for i := 0; i < len(keyvals)-1; i += 2 {
		event = event.Any(fmt.Sprint(keyvals[i]), keyvals[i+1])
	}
	return event
}
