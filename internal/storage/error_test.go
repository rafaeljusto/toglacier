package storage_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         storage.Error
		expected    string
	}{
		{
			description: "it should show the message with the low level error",
			err: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error"),
			},
			expected: "storage: error opening the storage file. details: low level error",
		},
		{
			description: "it should show the correct error message for opening file problem",
			err:         storage.Error{Code: storage.ErrorCodeOpeningFile},
			expected:    "storage: error opening the storage file",
		},
		{
			description: "it should show the correct error message for writing file problem",
			err:         storage.Error{Code: storage.ErrorCodeWritingFile},
			expected:    "storage: error writing the storage file",
		},
		{
			description: "it should show the correct error message for reading file problem",
			err:         storage.Error{Code: storage.ErrorCodeReadingFile},
			expected:    "storage: error reading the storage file",
		},
		{
			description: "it should show the correct error message for moving file problem",
			err:         storage.Error{Code: storage.ErrorCodeMovingFile},
			expected:    "storage: error moving the storage file",
		},
		{
			description: "it should show the correct error message for wrong format file problem",
			err:         storage.Error{Code: storage.ErrorCodeFormat},
			expected:    "storage: unexpected storage file format",
		},
		{
			description: "it should show the correct error message for invalid date problem",
			err:         storage.Error{Code: storage.ErrorCodeDateFormat},
			expected:    "storage: invalid date format",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         storage.Error{Code: storage.ErrorCode("i-dont-exist")},
			expected:    "storage: unknown error code",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if msg := scenario.err.Error(); msg != scenario.expected {
				t.Errorf("errors don't match. expected “%s” and got “%s”", scenario.expected, msg)
			}
		})
	}
}

func TestErrorEqual(t *testing.T) {
	scenarios := []struct {
		description string
		err1        error
		err2        error
		expected    bool
	}{
		{
			description: "it should detect equal Error instances",
			err1: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error"),
			},
			err2: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the code is different",
			err1: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error"),
			},
			err2: storage.Error{
				Code: storage.ErrorCodeWritingFile,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error 1"),
			},
			err2: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error 2"),
			},
			expected: false,
		},
		{
			description: "it should detect when both errors are undefined",
			expected:    true,
		},
		{
			description: "it should detect when only one error is undefined",
			err1: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when both causes of the error are undefined",
			err1: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
			},
			err2: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
			},
			expected: true,
		},
		{
			description: "it should detect when only one causes of the error is undefined",
			err1: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error"),
			},
			err2: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't Error type",
			err1: storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err:  errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := storage.ErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
