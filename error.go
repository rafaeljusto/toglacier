package toglacier

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

const (
	// ErrorCodeModifyTolerance error when too many files were modified between
	// backups. This is an alert for ransomware infection.
	ErrorCodeModifyTolerance ErrorCode = "modify-tolerance"
)

// ErrorCode stores the error type that occurred while processing commands from
// toglacier.
type ErrorCode string

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	switch e {
	case ErrorCodeModifyTolerance:
		return "too many files modified, aborting for precaution"
	}

	return "unknown error code"
}

// Error stores error details from a problem occurred while executing high level
// commands from toglacier.
type Error struct {
	Paths []string
	Code  ErrorCode
	Err   error
}

func newError(paths []string, code ErrorCode, err error) *Error {
	return &Error{
		Paths: paths,
		Code:  code,
		Err:   errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (e Error) Error() string {
	return e.String()
}

// String translate the error to a human readable text.
func (e Error) String() string {
	var paths string
	if e.Paths != nil {
		paths = fmt.Sprintf("paths [%s], ", strings.Join(e.Paths, ", "))
	}

	var err string
	if e.Err != nil {
		err = fmt.Sprintf(". details: %s", e.Err)
	}

	return fmt.Sprintf("toglacier: %s%s%s", paths, e.Code, err)
}

// ErrorEqual compares two Error objects. This is useful to compare down to the
// low level errors.
func ErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(*Error)
	err2, ok2 := errors.Cause(second).(*Error)

	if !ok1 || !ok2 {
		return false
	}

	if !reflect.DeepEqual(err1.Paths, err2.Paths) || err1.Code != err2.Code {
		return false
	}

	errCause1 := errors.Cause(err1.Err)
	errCause2 := errors.Cause(err2.Err)

	if errCause1 == nil || errCause2 == nil {
		return errCause1 == errCause2
	}

	return errCause1.Error() == errCause2.Error()
}
