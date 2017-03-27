package config_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/config"
)

func TestConfigError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         config.ConfigError
		expected    string
	}{
		{
			description: "it should show the message with filename and low level error",
			err: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: "config: filename “example.txt”, error reading the configuration file. details: low level error",
		},
		{
			description: "it should show the message only with the filename",
			err: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
			},
			expected: "config: filename “example.txt”, error reading the configuration file",
		},
		{
			description: "it should show the message only with the low level error",
			err: config.ConfigError{
				Code: config.ConfigErrorCodeReadingFile,
				Err:  errors.New("low level error"),
			},
			expected: "config: error reading the configuration file. details: low level error",
		},
		{
			description: "it should show the correct error message for reading configuration file problem",
			err:         config.ConfigError{Code: config.ConfigErrorCodeReadingFile},
			expected:    "config: error reading the configuration file",
		},
		{
			description: "it should show the correct error message for parsing YAML problem",
			err:         config.ConfigError{Code: config.ConfigErrorCodeParsingYAML},
			expected:    "config: error parsing yaml",
		},
		{
			description: "it should show the correct error message for parsing YAML problem",
			err:         config.ConfigError{Code: config.ConfigErrorCodeReadingEnvVars},
			expected:    "config: error reading environment variables",
		},
		{
			description: "it should show the correct error message for initializing cipher problem",
			err:         config.ConfigError{Code: config.ConfigErrorCodeInitCipher},
			expected:    "config: error initializing cipher",
		},
		{
			description: "it should show the correct error message for decoding base64 problem",
			err:         config.ConfigError{Code: config.ConfigErrorCodeDecodeBase64},
			expected:    "config: error deconding base64",
		},
		{
			description: "it should show the correct error message for password size problem",
			err:         config.ConfigError{Code: config.ConfigErrorCodePasswordSize},
			expected:    "config: invalid password size",
		},
		{
			description: "it should show the correct error message for filling iv problem",
			err:         config.ConfigError{Code: config.ConfigErrorCodeFillingIV},
			expected:    "config: error filling iv",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         config.ConfigError{Code: config.ConfigErrorCode("i-dont-exist")},
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

func TestConfigErrorEqual(t *testing.T) {
	scenarios := []struct {
		description string
		err1        error
		err2        error
		expected    bool
	}{
		{
			description: "it should detect equal ConfigError instances",
			err1: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the filename is different",
			err1: config.ConfigError{
				Filename: "example1.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: config.ConfigError{
				Filename: "example2.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeParsingYAML,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error 1"),
			},
			err2: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
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
			err1: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when both causes of the error are undefined",
			err1: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
			},
			err2: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
			},
			expected: true,
		},
		{
			description: "it should detect when only one causes of the error is undefined",
			err1: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't ConfigError type",
			err1: config.ConfigError{
				Filename: "example.txt",
				Code:     config.ConfigErrorCodeReadingFile,
				Err:      errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := config.ConfigErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
