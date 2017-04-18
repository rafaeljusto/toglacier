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

	// ErrorCodeEncodingBackup failed to encode the backup to a storage
	// representation.
	ErrorCodeEncodingBackup ErrorCode = "encoding-backup"

	// ErrorCodeDecodingBackup failed to decode the backup to the original format.
	ErrorCodeDecodingBackup ErrorCode = "decoding-backup"

	// ErrorCodeDatabaseNotFound database wasn't found.
	ErrorCodeDatabaseNotFound ErrorCode = "database-not-found"

	// ErrorCodeUpdatingDatabase problem while updating the database.
	ErrorCodeUpdatingDatabase ErrorCode = "updating-database"

	// ErrorCodeListingDatabase failed to list the backups from the database.
	ErrorCodeListingDatabase ErrorCode = "listing-database"

	// ErrorCodeSave failed to save the item in the database.
	ErrorCodeSave ErrorCode = "save"

	// ErrorCodeDelete failed to remove the item from the database.
	ErrorCodeDelete ErrorCode = "delete"

	// ErrorCodeIterating error while iterating over the database results.
	ErrorCodeIterating ErrorCode = "iterating"

	// ErrorAccessingBucket failed to open or create a database bucket.
	ErrorAccessingBucket ErrorCode = "accessing-bucket"
)

// ErrorCode stores the error type that occurred while managing the local
// storage.
type ErrorCode string

var errorCodeString = map[ErrorCode]string{
	ErrorCodeOpeningFile:      "error opening the storage file",
	ErrorCodeWritingFile:      "error writing the storage file",
	ErrorCodeReadingFile:      "error reading the storage file",
	ErrorCodeMovingFile:       "error moving the storage file",
	ErrorCodeFormat:           "unexpected storage file format",
	ErrorCodeDateFormat:       "invalid date format",
	ErrorCodeEncodingBackup:   "failed to encode backup to a storage representation",
	ErrorCodeDecodingBackup:   "failed to decode backup to the original representation",
	ErrorCodeDatabaseNotFound: "database not found",
	ErrorCodeUpdatingDatabase: "failed to update database",
	ErrorCodeListingDatabase:  "failed to list backups in the database",
	ErrorCodeSave:             "failed to save the item in the database",
	ErrorCodeDelete:           "failed to remove the item from the database",
	ErrorCodeIterating:        "error while iterating over the database results",
	ErrorAccessingBucket:      "failed to open or create a database bucket",
}

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	if msg, ok := errorCodeString[e]; ok {
		return msg
	}

	return "unknown error code"
}

// Error stores error details from a problem occurred while managing the local
// storage.
type Error struct {
	Code ErrorCode
	Err  error
}

func newError(code ErrorCode, err error) *Error {
	return &Error{
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

	err1, ok1 := errors.Cause(first).(*Error)
	err2, ok2 := errors.Cause(second).(*Error)

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
