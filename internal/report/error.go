package report

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// ErrorCodeTemplate error parsing template.
	ErrorCodeTemplate ErrorCode = "template"
)

// ErrorCode stores the error type that occurred while reading report
// parameters.
type ErrorCode string

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	switch e {
	case ErrorCodeTemplate:
		return "error parsing template"
	}

	return "unknown error code"
}

// Error stores error details from a problem occurred while reading a report
// file or parsing the environment variables.
type Error struct {
	Code ErrorCode
	Err  error
}

func newError(code ErrorCode, err error) Error {
	return Error{
		Code: code,
		Err:  errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (e Error) Error() string {
	return e.String()
}

// String translate the error to a human readable text.
func (e Error) String() string {
	var err string
	if e.Err != nil {
		err = fmt.Sprintf(". details: %s", e.Err)
	}

	return fmt.Sprintf("report: %s%s", e.Code, err)
}

// ErrorEqual compares two Error objects. This is useful to compare down to the
// low level errors.
func ErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(Error)
	err2, ok2 := errors.Cause(second).(Error)

	if !ok1 || !ok2 {
		return false
	}

	if err1.Code != err2.Code {
		return false
	}

	errCause1 := errors.Cause(err1.Err)
	errCause2 := errors.Cause(err2.Err)

	if errCause1 == nil || errCause2 == nil {
		return errCause1 == errCause2
	}

	return errCause1.Error() == errCause2.Error()
}
