// Package log connects to a local or remote syslog server with fallback to
// stderr output.
package log

import (
	"errors"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/registrobr/gostk/path"
)

// pathDeep defines the number of directories that are visible when logging a
// message with the logging location.
const pathDeep = 3

// Syslog level message, defined in RFC 5424, section 6.2.1
const (
	// LevelEmergency sets a high priority level of problem advising that system
	// is unusable.
	LevelEmergency Level = 0

	// LevelAlert sets a high priority level of problem advising to correct
	// immediately.
	LevelAlert Level = 1

	// LevelCritical sets a medium priority level of problem indicating a failure
	// in a primary system.
	LevelCritical Level = 2

	// LevelError sets a medium priority level of problem indicating a non-urgent
	// failure.
	LevelError Level = 3

	// LevelWarning sets a low priority level indicating that an error will occur
	// if action is not taken.
	LevelWarning Level = 4

	// LevelNotice sets a low priority level indicating events that are unusual,
	// but not error conditions.
	LevelNotice Level = 5

	// LevelInfo sets a very low priority level indicating normal operational
	// messages that require no action.
	LevelInfo Level = 6

	// LevelDebug sets a very low priority level indicating information useful to
	// developers for debugging the application.
	LevelDebug Level = 7
)

var (
	// ErrDialTimeout returned when a syslog dial connection takes too long to be
	// established.
	ErrDialTimeout = errors.New("dial timeout")
)

// Level defines the severity of an error. For example, if a custom error is
// created as bellow:
//
//    import "github.com/registrobr/gostk/log"
//
//    type ErrDatabaseFailure struct {
//    }
//
//    func (e ErrDatabaseFailure) Error() string {
//      return "database failure!"
//    }
//
//    func (e ErrDatabaseFailure) Level() log.Level {
//      return log.LevelEmergency
//    }
//
//  When used with the Logger type will be written in the syslog in the
//  corresponding log level.
type Level int

type leveler interface {
	Level() Level
}

// syslogWriter is useful to mock a low level syslog writer for unit tests.
type syslogWriter interface {
	Close() error
	Emerg(m string) (err error)
	Alert(m string) (err error)
	Crit(m string) (err error)
	Err(m string) (err error)
	Warning(m string) (err error)
	Notice(m string) (err error)
	Info(m string) (err error)
	Debug(m string) (err error)
}

var (
	// remoteLogger connection with a remote syslog server.
	remoteLogger syslogWriter

	// LocalLogger is the fallback log used when the remote logger isn't
	// available.
	LocalLogger *log.Logger
)

func init() {
	LocalLogger = log.New(os.Stderr, "", log.LstdFlags)
}

// Dial establishes a connection to a log daemon by connecting to
// address raddr on the specified network.  Each write to the returned
// writer sends a log message with the given facility, severity and
// tag. If network is empty, Dial will connect to the local syslog server. A
// connection timeout defines how long it will wait for the connection until a
// timeout error is raised.
func Dial(network, raddr, tag string, timeout time.Duration) error {
	// The channels has size of 1 (buffered) to avoid keeping an unnecessary goroutine blocked in
	// memory. For example: a goroutine is spawn, and it returns via channel a new transaction or
	// an error. After spawning a goroutine the program blocks in the select statement waiting
	// until the first channel message. In case of a timeout message, the spawned goroutine will
	// put a message in one of this two channels (ch and chErr) and simply returns (die), the
	// program don't care about the messages, because it has already timed out. If the channels
	// were not buffered the goroutine would be blocked trying to put a message into the channel
	// until the program dies.
	ch := make(chan *syslog.Writer, 1)
	chErr := make(chan error, 1)

	go func() {
		w, err := syslog.Dial(network, raddr, syslog.LOG_INFO|syslog.LOG_LOCAL0, tag)
		if err != nil {
			chErr <- err
			return
		}

		ch <- w
	}()

	select {
	case remoteLogger = <-ch:
		return nil
	case err := <-chErr:
		return err
	case <-time.After(timeout):
		return ErrDialTimeout
	}
}

// Close closes a connection to the syslog daemon. It is declared as a variable
// to allow an easy mocking.
var Close = func() error {
	if remoteLogger == nil {
		return nil
	}

	err := remoteLogger.Close()
	if err == nil {
		remoteLogger = nil
	}
	return err
}

// Logger allows logging messages in all different level types. As it is an
// interface it can be replaced by mocks for test purposes.
type Logger interface {
	Emerg(m ...interface{})
	Emergf(m string, a ...interface{})
	Alert(m ...interface{})
	Alertf(m string, a ...interface{})
	Crit(m ...interface{})
	Critf(m string, a ...interface{})
	Error(e error)
	Errorf(m string, a ...interface{})
	Warning(m ...interface{})
	Warningf(m string, a ...interface{})
	Notice(m ...interface{})
	Noticef(m string, a ...interface{})
	Info(m ...interface{})
	Infof(m string, a ...interface{})
	Debug(m ...interface{})
	Debugf(m string, a ...interface{})

	// SetCaller defines the number of invocations to follow-up to retrieve the
	// actual caller of the log entry. For now is only used by the package easy
	// functions.
	SetCaller(n int)
}

type logger struct {
	identifier string
	caller     int
}

// NewLogger returns a internal instance of the Logger type tagging an
// identifier to every message logged. This identifier is useful to group many
// messages to one related transaction id.
var NewLogger = func(id string) Logger {
	return &logger{
		identifier: "[" + id + "] ",
		caller:     3,
	}
}

func (l logger) Emerg(a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Emerg
	}
	l.logWithSourceInfo(f, a...)
}

func (l logger) Emergf(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Emerg
	}
	l.logWithSourceInfof(f, m, a...)
}

func (l logger) Alert(a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Alert
	}
	l.logWithSourceInfo(f, a...)
}

func (l logger) Alertf(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Alert
	}
	l.logWithSourceInfof(f, m, a...)
}

func (l logger) Crit(a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Crit
	}
	l.logWithSourceInfo(f, a...)
}

func (l logger) Critf(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Crit
	}
	l.logWithSourceInfof(f, m, a...)
}

// Error converts an Go error into an error message. The responsibility of
// knowing the file and line where the error occurred is from the Error()
// function of the specific error.
func (l logger) Error(e error) {
	if e == nil {
		return
	}

	msg := l.identifier + e.Error()
	if remoteLogger == nil {
		LocalLogger.Println(msg)
		return
	}

	var err error

	if levelError, ok := e.(leveler); ok {
		switch levelError.Level() {
		case LevelEmergency:
			err = remoteLogger.Emerg(msg)
		case LevelAlert:
			err = remoteLogger.Alert(msg)
		case LevelCritical:
			err = remoteLogger.Crit(msg)
		case LevelError:
			err = remoteLogger.Err(msg)
		case LevelWarning:
			err = remoteLogger.Warning(msg)
		case LevelNotice:
			err = remoteLogger.Notice(msg)
		case LevelInfo:
			err = remoteLogger.Info(msg)
		case LevelDebug:
			err = remoteLogger.Debug(msg)
		default:
			l.Warningf("Wrong error level: %d", levelError.Level())
			err = remoteLogger.Err(msg)
		}
	} else {
		err = remoteLogger.Err(msg)
	}

	if err != nil {
		LocalLogger.Println("Error writing to syslog. Details:", err)
		LocalLogger.Println(msg)
	}
}

func (l logger) Errorf(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Err
	}
	l.logWithSourceInfof(f, m, a...)
}

func (l logger) Warning(a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Warning
	}
	l.logWithSourceInfo(f, a...)
}

func (l logger) Warningf(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Warning
	}
	l.logWithSourceInfof(f, m, a...)
}

func (l logger) Notice(a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Notice
	}
	l.logWithSourceInfo(f, a...)
}

func (l logger) Noticef(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Notice
	}
	l.logWithSourceInfof(f, m, a...)
}

func (l logger) Info(a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Info
	}
	l.logWithSourceInfo(f, a...)
}

func (l logger) Infof(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Info
	}
	l.logWithSourceInfof(f, m, a...)
}

func (l logger) Debug(a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Debug
	}
	l.logWithSourceInfo(f, a...)
}

func (l logger) Debugf(m string, a ...interface{}) {
	var f logFunc
	if remoteLogger != nil {
		f = remoteLogger.Debug
	}
	l.logWithSourceInfof(f, m, a...)
}

func (l *logger) SetCaller(n int) {
	l.caller = n
}

// Emerg log an emergency message
func Emerg(a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Emerg(a...)
}

// Emergf log an emergency message with arguments
func Emergf(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Emergf(m, a...)
}

// Alert log an emergency message
func Alert(a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Alert(a...)
}

// Alertf log an emergency message with arguments
func Alertf(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Alertf(m, a...)
}

// Crit log an emergency message
func Crit(a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Crit(a...)
}

// Critf log an emergency message with arguments
func Critf(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Critf(m, a...)
}

// Error log an emergency message
func Error(err error) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Error(err)
}

// Errorf log an emergency message with arguments
func Errorf(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Errorf(m, a...)
}

// Warning log an emergency message
func Warning(a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Warning(a...)
}

// Warningf log an emergency message with arguments
func Warningf(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Warningf(m, a...)
}

// Notice log an emergency message
func Notice(a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Notice(a...)
}

// Noticef log an emergency message with arguments
func Noticef(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Noticef(m, a...)
}

// Info log an emergency message
func Info(a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Info(a...)
}

// Infof log an emergency message with arguments
func Infof(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Infof(m, a...)
}

// Debug log an emergency message
func Debug(a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Debug(a...)
}

// Debugf log an emergency message with arguments
func Debugf(m string, a ...interface{}) {
	l := NewLogger("")
	l.SetCaller(4)
	l.Debugf(m, a...)
}

type logFunc func(string) error

func (l logger) logWithSourceInfo(f logFunc, a ...interface{}) {
	// identify the caller from 3 levels above, as this function is never called
	// directly from the place that logged the message
	_, file, line, _ := runtime.Caller(l.caller)
	file = path.RelevantPath(file, pathDeep)
	doLog(f, l.identifier, fmt.Sprint(a...), file, line)
}

func (l logger) logWithSourceInfof(f logFunc, message string, a ...interface{}) {
	// identify the caller from 3 levels above, as this function is never called
	// directly from the place that logged the message
	_, file, line, _ := runtime.Caller(l.caller)
	file = path.RelevantPath(file, pathDeep)
	doLog(f, l.identifier, fmt.Sprintf(message, a...), file, line)
}

func doLog(f logFunc, prefix, message, file string, line int) {
	// support multiline log message, breaking it in many log entries
	for _, item := range strings.Split(message, "\n") {
		if item == "" {
			continue
		}

		msg := fmt.Sprintf("%s%s:%d: %s", prefix, file, line, item)

		if f == nil {
			LocalLogger.Println(msg)

		} else if err := f(msg); err != nil {
			LocalLogger.Println("Error writing to syslog. Details:", err)
			LocalLogger.Println(msg)
		}
	}
}
