package chshare

import (
	"errors"
	"fmt"
	"log"
	"os"
)

// Logger is a logical log output stream with a level filter
// and a prefix added to each output record.
type Logger struct {
	prefix      string
	logger      *log.Logger
	Info, Debug bool
}

// NewLogger creates a new Logger with a given prefix and Default flags,
// emitting output to os.Stderr
func NewLogger(prefix string) *Logger {
	return NewLoggerFlag(prefix, log.Ldate|log.Ltime)
}

// NewLoggerFlag creates a new Logger with a given prefix flags, emitting output
// to os.Stderr
func NewLoggerFlag(prefix string, flag int) *Logger {
	l := &Logger{
		prefix: prefix,
		logger: log.New(os.Stderr, "", flag),
		Info:   false,
		Debug:  false,
	}
	return l
}

// Infof outputs to a Logger iff INFO logging level is enabled
func (l *Logger) Infof(f string, args ...interface{}) {
	if l.Info {
		l.logger.Print(l.Sprintf(f, args...))
	}
}

// Debugf outputs to a Logger iff DEBUG logging level is enabled
func (l *Logger) Debugf(f string, args ...interface{}) {
	if l.Debug {
		l.logger.Printf(l.Sprintf(f, args...))
	}
}

// Errorf returns an error object with a description string that has the
// Logger's prefix
func (l *Logger) Errorf(f string, args ...interface{}) error {
	return errors.New(l.Sprintf(f, args...))
}

// Sprintf returns a string that has the Logger's prefix
func (l *Logger) Sprintf(f string, args ...interface{}) string {
	return l.prefix + ": " + fmt.Sprintf(f, args...)
}

// DebugErrorf outputs an error message to a Logger iff DEBUG logging level is enabled,
// and returns an error object with a description string that has the
// logger's prefix
func (l *Logger) DebugErrorf(f string, args ...interface{}) error {
	s := l.Sprintf(f, args...)
	if l.Debug {
		l.logger.Print(s)
	}
	return errors.New(s)
}

// InfoErrorf outputs an error message to a Logger iff ERROR logging level is enabled,
// and returns an error object with a description string that has the
// logger's prefix
func (l *Logger) InfoErrorf(f string, args ...interface{}) error {
	s := l.Sprintf(f, args...)
	if l.Info {
		l.logger.Print("Error: " + s)
	}
	return errors.New(s)
}

// Fork creates a new Logger that has an additional formatted string appended onto
// an existing logger's prefix (with ": " added between)
func (l *Logger) Fork(prefix string, args ...interface{}) *Logger {
	//slip the parent prefix at the front
	args = append([]interface{}{l.prefix}, args...)
	ll := NewLogger(fmt.Sprintf("%s: "+prefix, args...))
	ll.Info = l.Info
	ll.Debug = l.Debug
	return ll
}

// Prefix returns the Logger's prefix string (does not include ": " trailer)
func (l *Logger) Prefix() string {
	return l.prefix
}
