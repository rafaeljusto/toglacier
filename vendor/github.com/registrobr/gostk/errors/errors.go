// Package errors adds location and log levels to errors.
package errors

import (
	"fmt"
	"runtime"

	"github.com/registrobr/gostk/log"
	"github.com/registrobr/gostk/path"
)

// pathDeep defines the number of directories that are visible when printing an
// error location.
const pathDeep = 3

// traceableError stores the low level error with the location.
type traceableError struct {
	err   error
	file  string
	line  int
	level log.Level
}

// New returns an error that encapsulates another low level error, storing the
// file and line location.
func New(err error) error {
	if err == nil {
		return nil
	}

	_, file, line, _ := runtime.Caller(1)
	return traceableError{err: err, file: file, line: line, level: log.LevelError}
}

// NewWithFollowUp works exactly as New but defines the number of invocations to
// follow-up to retrieve the actual caller of the error. Useful when the user
// adds an extra layer over the current Error type.
func NewWithFollowUp(err error, followUp int) error {
	if err == nil {
		return nil
	}

	_, file, line, _ := runtime.Caller(followUp)
	return traceableError{err: err, file: file, line: line, level: log.LevelError}
}

// Error string representation of the error adding the location.
func (e traceableError) Error() string {
	format := "%s:%d: %s"
	if _, ok := e.err.(traceableError); ok {
		// stacktrace format
		format = "%s:%d → %s"
	}

	return fmt.Sprintf(format, path.RelevantPath(e.file, pathDeep), e.line, e.err.Error())
}

// Level returns the priority of this error. Useful when logging with this
// library in a syslog server.
func (e traceableError) Level() log.Level {
	return e.level
}

// Emergf returns an error that formats as the given text with an emergency log
// level.
func Emergf(msg string, a ...interface{}) error {
	_, file, line, _ := runtime.Caller(1)
	err := fmt.Errorf(msg, a...)
	return traceableError{err: err, file: file, line: line, level: log.LevelEmergency}
}

// Alertf returns an error that formats as the given text with an emergency log
// level.
func Alertf(msg string, a ...interface{}) error {
	_, file, line, _ := runtime.Caller(1)
	err := fmt.Errorf(msg, a...)
	return traceableError{err: err, file: file, line: line, level: log.LevelAlert}
}

// Critf returns an error that formats as the given text with an emergency log
// level.
func Critf(msg string, a ...interface{}) error {
	_, file, line, _ := runtime.Caller(1)
	err := fmt.Errorf(msg, a...)
	return traceableError{err: err, file: file, line: line, level: log.LevelCritical}
}

// Errorf returns an error that formats as the given text with an error log
// level.
func Errorf(msg string, a ...interface{}) error {
	_, file, line, _ := runtime.Caller(1)
	err := fmt.Errorf(msg, a...)
	return traceableError{err: err, file: file, line: line, level: log.LevelError}
}
