// Package log defines an interface for the library be able to log what is
// happening on each stage.
package log

// Logger contains all log actions that the library can do.
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
}
