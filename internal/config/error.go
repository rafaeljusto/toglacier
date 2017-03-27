package config

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// ConfigErrorCodeReadingFile error while reading the configuration file.
	ConfigErrorCodeReadingFile ConfigErrorCode = "reading-file"

	// ConfigErrorCodeParsingYAML error while parsing the configuration file as
	// YAML.
	ConfigErrorCodeParsingYAML ConfigErrorCode = "parsing-yaml"

	// ConfigErrorCodeReadingEnvVars error while reading configuration values from
	// environment variables.
	ConfigErrorCodeReadingEnvVars ConfigErrorCode = "reading-env-vars"

	// ConfigErrorCodeInitCipher error while initializing the engine used to
	// encrypt or decrypt the value.
	ConfigErrorCodeInitCipher ConfigErrorCode = "init-cipher"

	// ConfigErrorCodeDecodeBase64 problem while deconding a base64 content.
	ConfigErrorCodeDecodeBase64 ConfigErrorCode = "decode-base64"

	// ConfigErrorCodePasswordSize invalid password size. The password is smaller
	// than the cipher block size.
	ConfigErrorCodePasswordSize ConfigErrorCode = "password-size"

	// ConfigErrorCodeFillingIV error while filling the IV array with random bytes.
	ConfigErrorCodeFillingIV ConfigErrorCode = "filling-iv"
)

// ConfigErrorCode stores the error type that occurred while performing any
// operation with the tool configurations.
type ConfigErrorCode string

// String translate the error code to a human readable text.
func (c ConfigErrorCode) String() string {
	switch c {
	case ConfigErrorCodeReadingFile:
		return "error reading the configuration file"
	case ConfigErrorCodeParsingYAML:
		return "error parsing yaml"
	case ConfigErrorCodeReadingEnvVars:
		return "error reading environment variables"
	case ConfigErrorCodeInitCipher:
		return "error initializing cipher"
	case ConfigErrorCodeDecodeBase64:
		return "error deconding base64"
	case ConfigErrorCodePasswordSize:
		return "invalid password size"
	case ConfigErrorCodeFillingIV:
		return "error filling iv"
	}

	return "unknown error code"
}

// ConfigError stores error details from cloud operations.
type ConfigError struct {
	Filename string
	Code     ConfigErrorCode
	Err      error
}

func newConfigError(filename string, code ConfigErrorCode, err error) ConfigError {
	return ConfigError{
		Filename: filename,
		Code:     code,
		Err:      errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (c ConfigError) Error() string {
	return c.String()
}

// String translate the error to a human readable text.
func (c ConfigError) String() string {
	var filename string
	if c.Filename != "" {
		filename = fmt.Sprintf("filename “%s”, ", c.Filename)
	}

	var err string
	if c.Err != nil {
		err = fmt.Sprintf(". details: %s", c.Err)
	}

	return fmt.Sprintf("config: %s%s%s", filename, c.Code, err)
}

// ConfigErrorEqual compares two ConfigError objects. This is useful to
// compare down to the low level errors.
func ConfigErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(ConfigError)
	err2, ok2 := errors.Cause(second).(ConfigError)

	if !ok1 || !ok2 {
		return false
	}

	if err1.Filename != err2.Filename || err1.Code != err2.Code {
		return false
	}

	errCause1 := errors.Cause(err1.Err)
	errCause2 := errors.Cause(err2.Err)

	if errCause1 == nil || errCause2 == nil {
		return errCause1 == errCause2
	}

	return errCause1.Error() == errCause2.Error()
}
