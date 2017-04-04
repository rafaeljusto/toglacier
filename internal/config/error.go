package config

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// ErrorCodeReadingFile error while reading the configuration file.
	ErrorCodeReadingFile ErrorCode = "reading-file"

	// ErrorCodeParsingYAML error while parsing the configuration file as YAML.
	ErrorCodeParsingYAML ErrorCode = "parsing-yaml"

	// ErrorCodeReadingEnvVars error while reading configuration values from
	// environment variables.
	ErrorCodeReadingEnvVars ErrorCode = "reading-env-vars"

	// ErrorCodeInitCipher error while initializing the engine used to encrypt or
	// decrypt the value.
	ErrorCodeInitCipher ErrorCode = "init-cipher"

	// ErrorCodeDecodeBase64 problem while decoding a base64 content.
	ErrorCodeDecodeBase64 ErrorCode = "decode-base64"

	// ErrorCodePasswordSize invalid password size. The password is smaller than
	// the cipher block size.
	ErrorCodePasswordSize ErrorCode = "password-size"

	// ErrorCodeFillingIV error while filling the IV array with random bytes.
	ErrorCodeFillingIV ErrorCode = "filling-iv"
)

// ErrorCode stores the error type that occurred while reading
// configuration parameters.
type ErrorCode string

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	switch e {
	case ErrorCodeReadingFile:
		return "error reading the configuration file"
	case ErrorCodeParsingYAML:
		return "error parsing yaml"
	case ErrorCodeReadingEnvVars:
		return "error reading environment variables"
	case ErrorCodeInitCipher:
		return "error initializing cipher"
	case ErrorCodeDecodeBase64:
		return "error deconding base64"
	case ErrorCodePasswordSize:
		return "invalid password size"
	case ErrorCodeFillingIV:
		return "error filling iv"
	}

	return "unknown error code"
}

// Error stores error details from a problem occurred while reading a
// configuration file or parsing the environment variables.
type Error struct {
	Filename string
	Code     ErrorCode
	Err      error
}

func newError(filename string, code ErrorCode, err error) *Error {
	return &Error{
		Filename: filename,
		Code:     code,
		Err:      errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (e Error) Error() string {
	return e.String()
}

// String translate the error to a human readable text.
func (e Error) String() string {
	var filename string
	if e.Filename != "" {
		filename = fmt.Sprintf("filename “%s”, ", e.Filename)
	}

	var err string
	if e.Err != nil {
		err = fmt.Sprintf(". details: %s", e.Err)
	}

	return fmt.Sprintf("config: %s%s%s", filename, e.Code, err)
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
