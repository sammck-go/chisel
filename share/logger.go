package chshare

import (
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
		l.logger.Printf(l.prefix+": "+f, args...)
	}
}

// Debugf outputs to a Logger iff DEBUG logging level is enabled
func (l *Logger) Debugf(f string, args ...interface{}) {
	if l.Debug {
		l.logger.Printf(l.prefix+": "+f, args...)
	}
}

// Errorf returns an error object with a description string that has the
// Logger's prefix
func (l *Logger) Errorf(f string, args ...interface{}) error {
	return fmt.Errorf(l.prefix+": "+f, args...)
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
