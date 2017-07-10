package report

import (
	"fmt"

	"github.com/rafaeljusto/toglacier/internal/log"
)

// Log create an extra layer on the log engine to add to the report internal
// library messages.
type Log struct {
	logger log.Logger

	Debugs   []string
	Infos    []string
	Warnings []string
}

// NewLogger initializes the log report encapsulating logger inside it. It will
// store the logged messages before calling logger respective function.
func NewLogger(logger log.Logger) *Log {
	return &Log{
		logger: logger,
	}
}

// Debug add detailed messages for development. It uses the same behavior of the
// fmt.Sprint function.
func (l *Log) Debug(args ...interface{}) {
	l.Debugs = append(l.Debugs, fmt.Sprint(args...))
	l.logger.Debug(args...)
}

// Debugf add detailed messages for development. It uses the same behavior of
// the fmt.Sprintf function.
func (l *Log) Debugf(format string, args ...interface{}) {
	l.Debugs = append(l.Debugs, fmt.Sprintf(format, args...))
	l.logger.Debugf(format, args...)
}

// Info add an informational message. It uses the same behavior of the
// fmt.Sprint function.
func (l *Log) Info(args ...interface{}) {
	l.Infos = append(l.Infos, fmt.Sprint(args...))
	l.logger.Info(args...)
}

// Infof add an informational message. It uses the same behavior of the
// fmt.Sprintf function.
func (l *Log) Infof(format string, args ...interface{}) {
	l.Infos = append(l.Infos, fmt.Sprintf(format, args...))
	l.logger.Infof(format, args...)
}

// Warning reports a problem that it is not critical for the system
// functionality. It uses the same behavior of the fmt.Sprint function.
func (l *Log) Warning(args ...interface{}) {
	l.Warnings = append(l.Warnings, fmt.Sprint(args...))
	l.logger.Warning(args...)
}

// Warningf reports a problem that it is not critical for the system
// functionality. It uses the same behavior of the fmt.Sprintf function.
func (l *Log) Warningf(format string, args ...interface{}) {
	l.Warnings = append(l.Warnings, fmt.Sprintf(format, args...))
	l.logger.Warningf(format, args...)
}
