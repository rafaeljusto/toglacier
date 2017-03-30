package archive_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/archive"
)

func TestError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         archive.Error
		expected    string
	}{
		{
			description: "it should show the message with filename and low level error",
			err: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			expected: "archive: filename “example.txt”, error creating the tar file. details: low level error",
		},
		{
			description: "it should show the message only with the filename",
			err: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
			},
			expected: "archive: filename “example.txt”, error creating the tar file",
		},
		{
			description: "it should show the message only with the low level error",
			err: archive.Error{
				Code: archive.ErrorCodeTARCreation,
				Err:  errors.New("low level error"),
			},
			expected: "archive: error creating the tar file. details: low level error",
		},
		{
			description: "it should show the correct error message for TAR creation problem",
			err:         archive.Error{Code: archive.ErrorCodeTARCreation},
			expected:    "archive: error creating the tar file",
		},
		{
			description: "it should show the correct error message for TAR generation problem",
			err:         archive.Error{Code: archive.ErrorCodeTARGeneration},
			expected:    "archive: error generating tar file",
		},
		{
			description: "it should show the correct error message for opening file problem",
			err:         archive.Error{Code: archive.ErrorCodeOpeningFile},
			expected:    "archive: error opening file",
		},
		{
			description: "it should show the correct error message for creating temporary file problem",
			err:         archive.Error{Code: archive.ErrorCodeTmpFileCreation},
			expected:    "archive: error creating temporary file",
		},
		{
			description: "it should show the correct error message for HMAC-SHA256 calculation problem",
			err:         archive.Error{Code: archive.ErrorCodeCalculateHMACSHA256},
			expected:    "archive: error calculating hmac-sha256",
		},
		{
			description: "it should show the correct error message for IV random numbers problem",
			err:         archive.Error{Code: archive.ErrorCodeGenerateRandomNumbers},
			expected:    "archive: error filling iv with random numbers",
		},
		{
			description: "it should show the correct error message for writing encrypted label problem",
			err:         archive.Error{Code: archive.ErrorCodeWritingLabel},
			expected:    "archive: error writing label to encrypted file",
		},
		{
			description: "it should show the correct error message for reading encrypted label problem",
			err:         archive.Error{Code: archive.ErrorCodeReadingLabel},
			expected:    "archive: error reading encrypted file label",
		},
		{
			description: "it should show the correct error message for writing authentication problem",
			err:         archive.Error{Code: archive.ErrorCodeWritingAuth},
			expected:    "archive: error writing authentication to encrypted file",
		},
		{
			description: "it should show the correct error message for reading authentication problem",
			err:         archive.Error{Code: archive.ErrorCodeReadingAuth},
			expected:    "archive: error reading encrypted authentication",
		},
		{
			description: "it should show the correct error message for writing IV problem",
			err:         archive.Error{Code: archive.ErrorCodeWritingIV},
			expected:    "archive: error writing iv to encrypt file",
		},
		{
			description: "it should show the correct error message for reading IV problem",
			err:         archive.Error{Code: archive.ErrorCodeReadingIV},
			expected:    "archive: error reading iv to decrypt file",
		},
		{
			description: "it should show the correct error message for initializing cipher problem",
			err:         archive.Error{Code: archive.ErrorCodeInitCipher},
			expected:    "archive: error initializing cipher",
		},
		{
			description: "it should show the correct error message for encrypting file problem",
			err:         archive.Error{Code: archive.ErrorCodeEncryptingFile},
			expected:    "archive: error encrypting file",
		},
		{
			description: "it should show the correct error message for decrypting file problem",
			err:         archive.Error{Code: archive.ErrorCodeDecryptingFile},
			expected:    "archive: error decrypting file",
		},
		{
			description: "it should show the correct error message for authentication failed problem",
			err:         archive.Error{Code: archive.ErrorCodeAuthFailed},
			expected:    "archive: encrypted content authentication failed",
		},
		{
			description: "it should show the correct error message for rewinding file problem",
			err:         archive.Error{Code: archive.ErrorCodeRewindingFile},
			expected:    "archive: error moving to the beggining of the file",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         archive.Error{Code: archive.ErrorCode("i-dont-exist")},
			expected:    "archive: unknown error code",
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
			err1: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			err2: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the filename is different",
			err1: archive.Error{
				Filename: "example1.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			err2: archive.Error{
				Filename: "example2.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			err2: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeDecryptingFile,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error 1"),
			},
			err2: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error 2"),
			},
			expected: false,
		},
		{
			description: "it should detect when both errors are undefined",
			expected:    true,
		},
		{
			description: "it should detect when only one error is undefined",
			err1: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't Error type",
			err1: archive.Error{
				Filename: "example.txt",
				Code:     archive.ErrorCodeTARCreation,
				Err:      errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := archive.ErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}

func TestPathError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         archive.PathError
		expected    string
	}{
		{
			description: "it should show the message with path and low level error",
			err: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			expected: "archive: path “/tmp/data”, error retrieving information. details: low level error",
		},
		{
			description: "it should show the message only with the path",
			err: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
			},
			expected: "archive: path “/tmp/data”, error retrieving information",
		},
		{
			description: "it should show the message only with the low level error",
			err: archive.PathError{
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			expected: "archive: error retrieving information. details: low level error",
		},
		{
			description: "it should show the correct error message for path information problem",
			err:         archive.PathError{Code: archive.PathErrorCodeInfo},
			expected:    "archive: error retrieving information",
		},
		{
			description: "it should show the correct error message for TAR header creation problem",
			err:         archive.PathError{Code: archive.PathErrorCodeCreateTARHeader},
			expected:    "archive: error creating tar header",
		},
		{
			description: "it should show the correct error message for writing TAR header problem",
			err:         archive.PathError{Code: archive.PathErrorCodeWritingTARHeader},
			expected:    "archive: error writing header in tar",
		},
		{
			description: "it should show the correct error message for opening file problem",
			err:         archive.PathError{Code: archive.PathErrorCodeOpeningFile},
			expected:    "archive: error opening file",
		},
		{
			description: "it should show the correct error message for writing file problem",
			err:         archive.PathError{Code: archive.PathErrorCodeWritingFile},
			expected:    "archive: error writing content in tar",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         archive.PathError{Code: archive.PathErrorCode("i-dont-exist")},
			expected:    "archive: unknown error code",
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

func TestPathErrorEqual(t *testing.T) {
	scenarios := []struct {
		description string
		err1        error
		err2        error
		expected    bool
	}{
		{
			description: "it should detect equal PathError instances",
			err1: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			err2: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the path is different",
			err1: archive.PathError{
				Path: "/tmp/data1",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			err2: archive.PathError{
				Path: "/tmp/data2",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			err2: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeWritingTARHeader,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error 1"),
			},
			err2: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
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
			err1: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't PathError type",
			err1: archive.PathError{
				Path: "/tmp/data",
				Code: archive.PathErrorCodeInfo,
				Err:  errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := archive.PathErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
