package archive

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// ErrorCodeTARCreation error while creating the TAR file.
	ErrorCodeTARCreation ErrorCode = "tar-creation"

	// ErrorCodeTARGeneration error adding all files to the TAR.
	ErrorCodeTARGeneration ErrorCode = "tar-generation"

	// ErrorCodeOpeningFile error while opening a file to encrypt or decrypt.
	ErrorCodeOpeningFile ErrorCode = "opening-file"

	// ErrorCodeTmpFileCreation error creating a new temporary file.
	ErrorCodeTmpFileCreation ErrorCode = "tmp-file-creation"

	// ErrorCodeCalculateHMACSHA256 error calculating the HMAC SHA256 of the file.
	ErrorCodeCalculateHMACSHA256 ErrorCode = "calculate-hmac-sha256"

	// ErrorCodeGenerateRandomNumbers error while trying to retrieve random
	// numbers for a encryption process.
	ErrorCodeGenerateRandomNumbers ErrorCode = "generate-random-numbers"

	// ErrorCodeWritingLabel error while adding the “encrypted” label to the file.
	// This label identify when the content is encrypted or not.
	ErrorCodeWritingLabel ErrorCode = "writing-label"

	// ErrorCodeReadingLabel error while reading the “encrypted” label from the
	// file. This label identify when the content is encrypted or not.
	ErrorCodeReadingLabel ErrorCode = "reading-label"

	// ErrorCodeWritingAuth error while writing the HMAC-SHA256 authentication to
	// the file. The authentication is necessary to verify if the encrypted
	// content wasn't modified.
	ErrorCodeWritingAuth ErrorCode = "writing-auth"

	// ErrorCodeReadingAuth error while reading the HMAC-SHA256 authentication
	// from the file. The authentication is necessary to verify if the encrypted
	// content wasn't modified.
	ErrorCodeReadingAuth ErrorCode = "reading-auth"

	// ErrorCodeWritingIV error while writing the IV that is a slice of random
	// numbers used as a encryption source.
	ErrorCodeWritingIV ErrorCode = "writing-iv"

	// ErrorCodeReadingIV error while reading the IV that is used as the source to
	// decrypt the content.
	ErrorCodeReadingIV ErrorCode = "reading-iv"

	// ErrorCodeInitCipher error initializing cipher that is used for the
	// encryption process.
	ErrorCodeInitCipher ErrorCode = "init-cipher"

	// ErrorCodeEncryptingFile error while encrypting file.
	ErrorCodeEncryptingFile ErrorCode = "encrypting-file"

	// ErrorCodeDecryptingFile error while decrypting file.
	ErrorCodeDecryptingFile ErrorCode = "decypting-file"

	// ErrorCodeAuthFailed error when the HMAC authentication from the encrypted
	// file failed.
	ErrorCodeAuthFailed ErrorCode = "auth-failed"

	// ErrorCodeRewindingFile error while moving back to the beginning of the
	// file.
	ErrorCodeRewindingFile ErrorCode = "rewinding-file"

	// ErrorCodeEncodingInfo failed to add the archive information to the tarball.
	ErrorCodeEncodingInfo ErrorCode = "encoding-info"

	// ErrorCodeDecodingInfo failed to extract the archive information from the
	// tarball.
	ErrorCodeDecodingInfo ErrorCode = "decoding-info"

	// ErrorCodeReadingTAR error while iterating over the TAR headers.
	ErrorCodeReadingTAR ErrorCode = "reading-tar"

	// ErrorCodeCreatingDirectories failed to create directories.
	ErrorCodeCreatingDirectories ErrorCode = "creating-directories"

	// ErrorCodeExtractingFile problem extracting file from TAR.
	ErrorCodeExtractingFile ErrorCode = "extracting-file"
)

// ErrorCode stores the error type that occurred to easy automatize an external
// action depending on the problem.
type ErrorCode string

var errorCodeString = map[ErrorCode]string{
	ErrorCodeTARCreation:           "error creating the tar file",
	ErrorCodeTARGeneration:         "error generating tar file",
	ErrorCodeOpeningFile:           "error opening file",
	ErrorCodeTmpFileCreation:       "error creating temporary file",
	ErrorCodeCalculateHMACSHA256:   "error calculating hmac-sha256",
	ErrorCodeGenerateRandomNumbers: "error filling iv with random numbers",
	ErrorCodeWritingLabel:          "error writing label to encrypted file",
	ErrorCodeReadingLabel:          "error reading encrypted file label",
	ErrorCodeWritingAuth:           "error writing authentication to encrypted file",
	ErrorCodeReadingAuth:           "error reading encrypted authentication",
	ErrorCodeWritingIV:             "error writing iv to encrypt file",
	ErrorCodeReadingIV:             "error reading iv to decrypt file",
	ErrorCodeInitCipher:            "error initializing cipher",
	ErrorCodeEncryptingFile:        "error encrypting file",
	ErrorCodeDecryptingFile:        "error decrypting file",
	ErrorCodeAuthFailed:            "encrypted content authentication failed",
	ErrorCodeRewindingFile:         "error moving to the beginning of the file",
	ErrorCodeEncodingInfo:          "error encoding the archive information in the tarball",
	ErrorCodeDecodingInfo:          "error decoding the archive information from the tarball",
	ErrorCodeReadingTAR:            "error reading tar",
	ErrorCodeCreatingDirectories:   "error while creating directories",
	ErrorCodeExtractingFile:        "error extracting file",
}

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	if msg, ok := errorCodeString[e]; ok {
		return msg
	}

	return "unknown error code"
}

// Error stores error details from archive operations.
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

	return fmt.Sprintf("archive: %s%s%s", filename, e.Code, err)
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

const (
	// PathErrorCodeInfo error retrieving the path information.
	PathErrorCodeInfo PathErrorCode = "info"

	// PathErrorCodeCreateTARHeader error while creating the TAR header from the
	// path information.
	PathErrorCodeCreateTARHeader PathErrorCode = "create-tar-header"

	// PathErrorCodeWritingTARHeader error while writing the header into the TAR
	// file.
	PathErrorCodeWritingTARHeader PathErrorCode = "writing-tar-header"

	// PathErrorCodeOpeningFile error while opening file.
	PathErrorCodeOpeningFile PathErrorCode = "opening-file"

	// PathErrorCodeWritingFile error while writing the file content to the TAR
	// file.
	PathErrorCodeWritingFile PathErrorCode = "writing-file"

	// PathErrorCodeSHA256 error calculating SHA256 hash from the file.
	PathErrorCodeSHA256 PathErrorCode = "sha-256"

	// PathErrorCodeRewindingFile error while moving back to the beginning of the
	// file.
	PathErrorCodeRewindingFile PathErrorCode = "rewinding-file"
)

// PathErrorCode stores the error type that occurred to easy automatize an
// external actual depending on the problem.
type PathErrorCode string

// String translate the error code to a human readable text.
func (p PathErrorCode) String() string {
	switch p {
	case PathErrorCodeInfo:
		return "error retrieving information"
	case PathErrorCodeCreateTARHeader:
		return "error creating tar header"
	case PathErrorCodeWritingTARHeader:
		return "error writing header in tar"
	case PathErrorCodeOpeningFile:
		return "error opening file"
	case PathErrorCodeWritingFile:
		return "error writing content in tar"
	case PathErrorCodeSHA256:
		return "error calculating hash SHA256 from file"
	case PathErrorCodeRewindingFile:
		return "error moving to the beginning of the file"
	}

	return "unknown error code"
}

// PathError stores error details detected while traversing the path.
type PathError struct {
	Path string
	Code PathErrorCode
	Err  error
}

func newPathError(path string, code PathErrorCode, err error) *PathError {
	return &PathError{
		Path: path,
		Code: code,
		Err:  errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (p PathError) Error() string {
	return p.String()
}

// String translate the error to a human readable text.
func (p PathError) String() string {
	var path string
	if p.Path != "" {
		path = fmt.Sprintf("path “%s”, ", p.Path)
	}

	var err string
	if p.Err != nil {
		err = fmt.Sprintf(". details: %s", p.Err)
	}

	return fmt.Sprintf("archive: %s%s%s", path, p.Code, err)
}

// PathErrorEqual compares two PathError objects. This is useful to compare down
// to the low level errors.
func PathErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(*PathError)
	err2, ok2 := errors.Cause(second).(*PathError)

	if !ok1 || !ok2 {
		return false
	}

	if err1.Path != err2.Path || err1.Code != err2.Code {
		return false
	}

	errCause1 := errors.Cause(err1.Err)
	errCause2 := errors.Cause(err2.Err)

	if errCause1 == nil || errCause2 == nil {
		return errCause1 == errCause2
	}

	return errCause1.Error() == errCause2.Error()
}
