package storage

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// StorageErrorCodeOpeningFile error when opening the file that stores the
	// data locally.
	StorageErrorCodeOpeningFile StorageErrorCode = "opening-file"

	// StorageErrorCodeWritingFile error while writing the data to the local
	// storage file.
	StorageErrorCodeWritingFile StorageErrorCode = "writing-file"

	// StorageErrorCodeReadingFile error while reading the file that stores the
	// data locally.
	StorageErrorCodeReadingFile StorageErrorCode = "reading-file"

	// StorageErrorCodeMovingFile error moving files. We keep backups of the
	// storage file, so we need to move the old one before writing a new storage
	// file.
	StorageErrorCodeMovingFile StorageErrorCode = "moving-file"

	// StorageErrorCodeFormat local storage file contains an unexpected format
	// (corrupted).
	StorageErrorCodeFormat StorageErrorCode = "format"

	// StorageErrorCodeDateFormat strange date format found in the local storage
	// file.
	StorageErrorCodeDateFormat StorageErrorCode = "date-format"
)

// StorageErrorCode stores the error type that occurred while reading
// report parameters.
type StorageErrorCode string

// String translate the error code to a human readable text.
func (s StorageErrorCode) String() string {
	switch s {
	case StorageErrorCodeOpeningFile:
		return "error opening the storage file"
	case StorageErrorCodeWritingFile:
		return "error writing the storage file"
	case StorageErrorCodeReadingFile:
		return "error reading the storage file"
	case StorageErrorCodeMovingFile:
		return "error moving the storage file"
	case StorageErrorCodeFormat:
		return "unexpected storage file format"
	case StorageErrorCodeDateFormat:
		return "invalid date format"
	}

	return "unknown error code"
}

// StorageError stores error details from a problem occurred while reading a
// report file or parsing the environment variables.
type StorageError struct {
	Code StorageErrorCode
	Err  error
}

func newStorageError(code StorageErrorCode, err error) StorageError {
	return StorageError{
		Code: code,
		Err:  errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (s StorageError) Error() string {
	return s.String()
}

// String translate the error to a human readable text.
func (s StorageError) String() string {
	var err string
	if s.Err != nil {
		err = fmt.Sprintf(". details: %s", s.Err)
	}

	return fmt.Sprintf("storage: %s%s", s.Code, err)
}

// StorageErrorEqual compares two StorageError objects. This is useful to
// compare down to the low level errors.
func StorageErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(StorageError)
	err2, ok2 := errors.Cause(second).(StorageError)

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
