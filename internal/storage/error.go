package storage

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// ErrorCodeOpeningFile error when opening the file that stores the data
	// locally.
	ErrorCodeOpeningFile ErrorCode = "opening-file"

	// ErrorCodeWritingFile error while writing the data to the local storage
	// file.
	ErrorCodeWritingFile ErrorCode = "writing-file"

	// ErrorCodeReadingFile error while reading the file that stores the data
	// locally.
	ErrorCodeReadingFile ErrorCode = "reading-file"

	// ErrorCodeMovingFile error moving files. We keep backups of the storage
	// file, so we need to move the old one before writing a new storage file.
	ErrorCodeMovingFile ErrorCode = "moving-file"

	// ErrorCodeFormat local storage file contains an unexpected format
	// (corrupted).
	ErrorCodeFormat ErrorCode = "format"

	// ErrorCodeDateFormat strange date format found in the local storage file.
	ErrorCodeDateFormat ErrorCode = "date-format"
)

// ErrorCode stores the error type that occurred while managing the local
// storage.
type ErrorCode string

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	switch e {
	case ErrorCodeOpeningFile:
		return "error opening the storage file"
	case ErrorCodeWritingFile:
		return "error writing the storage file"
	case ErrorCodeReadingFile:
		return "error reading the storage file"
	case ErrorCodeMovingFile:
		return "error moving the storage file"
	case ErrorCodeFormat:
		return "unexpected storage file format"
	case ErrorCodeDateFormat:
		return "invalid date format"
	}

	return "unknown error code"
}

// Error stores error details from a problem occurred while managing the local
// storage.
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

	return fmt.Sprintf("storage: %s%s", e.Code, err)
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
