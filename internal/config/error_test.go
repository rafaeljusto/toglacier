package config_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/config"
)

func TestError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         *config.Error
		expected    string
	}{
		{
			description: "it should show the message with filename and low level error",
			err: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: "config: filename “example.txt”, error reading the configuration file. details: low level error",
		},
		{
			description: "it should show the message only with the filename",
			err: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
			},
			expected: "config: filename “example.txt”, error reading the configuration file",
		},
		{
			description: "it should show the message only with the low level error",
			err: &config.Error{
				Code: config.ErrorCodeReadingFile,
				Err:  errors.New("low level error"),
			},
			expected: "config: error reading the configuration file. details: low level error",
		},
		{
			description: "it should show the correct error message for reading configuration file problem",
			err:         &config.Error{Code: config.ErrorCodeReadingFile},
			expected:    "config: error reading the configuration file",
		},
		{
			description: "it should show the correct error message for parsing YAML problem",
			err:         &config.Error{Code: config.ErrorCodeParsingYAML},
			expected:    "config: error parsing yaml",
		},
		{
			description: "it should show the correct error message for parsing YAML problem",
			err:         &config.Error{Code: config.ErrorCodeReadingEnvVars},
			expected:    "config: error reading environment variables",
		},
		{
			description: "it should show the correct error message for initializing cipher problem",
			err:         &config.Error{Code: config.ErrorCodeInitCipher},
			expected:    "config: error initializing cipher",
		},
		{
			description: "it should show the correct error message for decoding base64 problem",
			err:         &config.Error{Code: config.ErrorCodeDecodeBase64},
			expected:    "config: error deconding base64",
		},
		{
			description: "it should show the correct error message for password size problem",
			err:         &config.Error{Code: config.ErrorCodePasswordSize},
			expected:    "config: invalid password size",
		},
		{
			description: "it should show the correct error message for filling iv problem",
			err:         &config.Error{Code: config.ErrorCodeFillingIV},
			expected:    "config: error filling iv",
		},
		{
			description: "it should show the correct error message for invalid database type",
			err:         &config.Error{Code: config.ErrorCodeDatabaseType},
			expected:    "config: invalid database type",
		},
		{
			description: "it should show the correct error message for invalid log level",
			err:         &config.Error{Code: config.ErrorCodeLogLevel},
			expected:    "config: invalid log level",
		},
		{
			description: "it should show the correct error message for invalid email format",
			err:         &config.Error{Code: config.ErrorCodeEmailFormat},
			expected:    "config: invalid email format",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         &config.Error{Code: config.ErrorCode("i-dont-exist")},
			expected:    "config: unknown error code",
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
			err1: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the filename is different",
			err1: &config.Error{
				Filename: "example1.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: &config.Error{
				Filename: "example2.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeParsingYAML,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error 1"),
			},
			err2: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
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
			err1: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when both causes of the error are undefined",
			err1: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
			},
			err2: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
			},
			expected: true,
		},
		{
			description: "it should detect when only one causes of the error is undefined",
			err1: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't Error type",
			err1: &config.Error{
				Filename: "example.txt",
				Code:     config.ErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := config.ErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
