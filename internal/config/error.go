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

	// ErrorCodeDatabaseType informed database type is unknown, it should be
	// "audit-file" or "boltdb".
	ErrorCodeDatabaseType ErrorCode = "database-type"

	// ErrorCodeLogLevel informed log level is unknown, it should be "debug",
	// "info", "warning", "error", "fatal" or "panic".
	ErrorCodeLogLevel ErrorCode = "log-level"

	// ErrorCodeEmailFormat informed email format is unknown, it should be "plain"
	// or "html".
	ErrorCodeEmailFormat ErrorCode = "email-format"

	// ErrorCodePercentageFormat invalid percentage format.
	ErrorCodePercentageFormat ErrorCode = "percentage-format"

	// ErrorCodePercentageRange percentage must be between 0 and 100.
	ErrorCodePercentageRange ErrorCode = "percentage-range"

	// ErrorCodePattern invalid pattern detected when parsing the regular
	// expression.
	ErrorCodePattern ErrorCode = "pattern"

	// ErrorCodeSchedulerFormat the number of items of the schedule is wrong, we
	// expect 6 space-separated items.
	ErrorCodeSchedulerFormat ErrorCode = "scheduler-format"

	// ErrorCodeSchedulerValue one or more values of the scheduler is invalid.
	// Could be an invalid syntax or range.
	ErrorCodeSchedulerValue ErrorCode = "scheduler-value"
)

// ErrorCode stores the error type that occurred while reading
// configuration parameters.
type ErrorCode string

var errorCodeString = map[ErrorCode]string{
	ErrorCodeReadingFile:      "error reading the configuration file",
	ErrorCodeParsingYAML:      "error parsing yaml",
	ErrorCodeReadingEnvVars:   "error reading environment variables",
	ErrorCodeInitCipher:       "error initializing cipher",
	ErrorCodeDecodeBase64:     "error decoding base64",
	ErrorCodePasswordSize:     "invalid password size",
	ErrorCodeFillingIV:        "error filling iv",
	ErrorCodeDatabaseType:     "invalid database type",
	ErrorCodeLogLevel:         "invalid log level",
	ErrorCodeEmailFormat:      "invalid email format",
	ErrorCodePercentageFormat: "invalid percentage format",
	ErrorCodePercentageRange:  "invalid percentage range",
	ErrorCodePattern:          "invalid pattern",
	ErrorCodeSchedulerFormat:  "wrong number of space-separated values in scheduler",
	ErrorCodeSchedulerValue:   "invalid value in scheduler",
}

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	if msg, ok := errorCodeString[e]; ok {
		return msg
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
