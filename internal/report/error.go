package report

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// ReportErrorCodeTemplate error parsing template.
	ReportErrorCodeTemplate ReportErrorCode = "template"
)

// ReportErrorCode stores the error type that occurred while reading
// report parameters.
type ReportErrorCode string

// String translate the error code to a human readable text.
func (c ReportErrorCode) String() string {
	switch c {
	case ReportErrorCodeTemplate:
		return "error parsing template"
	}

	return "unknown error code"
}

// ReportError stores error details from a problem occurred while reading a
// report file or parsing the environment variables.
type ReportError struct {
	Code ReportErrorCode
	Err  error
}

func newReportError(code ReportErrorCode, err error) ReportError {
	return ReportError{
		Code: code,
		Err:  errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (c ReportError) Error() string {
	return c.String()
}

// String translate the error to a human readable text.
func (c ReportError) String() string {
	var err string
	if c.Err != nil {
		err = fmt.Sprintf(". details: %s", c.Err)
	}

	return fmt.Sprintf("report: %s%s", c.Code, err)
}

// ReportErrorEqual compares two ReportError objects. This is useful to
// compare down to the low level errors.
func ReportErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(ReportError)
	err2, ok2 := errors.Cause(second).(ReportError)

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
