package archive

import (
	"fmt"

	"github.com/registrobr/gostk/errors"
)

const (
	// ArchiveErrorCodeTARCreation error while creating the TAR file.
	ArchiveErrorCodeTARCreation ArchiveErrorCode = "tar-creation"

	// ArchiveErrorCodeTARGeneration error adding all files to the TAR.
	ArchiveErrorCodeTARGeneration ArchiveErrorCode = "tar-generation"

	// ArchiveErrorCodeOpeningFile error while opening a file to encrypt or
	// decrypt.
	ArchiveErrorCodeOpeningFile ArchiveErrorCode = "opening-file"

	// ArchiveErrorCodeTmpFileCreation error creating a new temporary file.
	ArchiveErrorCodeTmpFileCreation ArchiveErrorCode = "tmp-file-creation"

	// ArchiveErrorCodeCalculateHMACSHA256 error calculating the HMAC SHA256 of
	// the file.
	ArchiveErrorCodeCalculateHMACSHA256 ArchiveErrorCode = "calculate-hmac-sha256"

	// ArchiveErrorCodeGenerateRandomNumbers error while trying to retrieve random
	// numbers for a encryption process.
	ArchiveErrorCodeGenerateRandomNumbers ArchiveErrorCode = "generate-random-numbers"

	// ArchiveErrorCodeWritingLabel error while adding the “encrypted” label to
	// the file. This label identify when the content is encrypted or not.
	ArchiveErrorCodeWritingLabel ArchiveErrorCode = "writing-label"

	// ArchiveErrorCodeReadingLabel error while reading the “encrypted” label from
	// the file. This label identify when the content is encrypted or not.
	ArchiveErrorCodeReadingLabel ArchiveErrorCode = "reading-label"

	// ArchiveErrorCodeWritingAuth error while writing the HMAC-SHA256
	// authentication to the file. The authentication is necessary to verify if
	// the encrypted content wasn't modified.
	ArchiveErrorCodeWritingAuth ArchiveErrorCode = "writing-auth"

	// ArchiveErrorCodeReadingAuth error while reading the HMAC-SHA256
	// authentication from the file. The authentication is necessary to verify if
	// the encrypted content wasn't modified.
	ArchiveErrorCodeReadingAuth ArchiveErrorCode = "reading-auth"

	// ArchiveErrorCodeWritingIV error while writing the IV that is a slice of
	// random numbers used as a encryption source.
	ArchiveErrorCodeWritingIV ArchiveErrorCode = "writing-iv"

	// ArchiveErrorCodeInitCipher error initializing cipher that is used for the
	// encryption process.
	ArchiveErrorCodeInitCipher ArchiveErrorCode = "init-cipher"

	// ArchiveErrorCodeEncryptingFile error while encrypting file.
	ArchiveErrorCodeEncryptingFile ArchiveErrorCode = "encrypting-file"

	// ArchiveErrorCodeDecryptingFile error while decrypting file.
	ArchiveErrorCodeDecryptingFile ArchiveErrorCode = "decypting-file"

	// ArchiveErrorCodeAuthFailed error when the HMAC authentication from the
	// encrypted file failed.
	ArchiveErrorCodeAuthFailed ArchiveErrorCode = "auth-failed"

	// ArchiveErrorCodeRewindingFile error while moving back to the beginning of
	// the file.
	ArchiveErrorCodeRewindingFile ArchiveErrorCode = "rewinding-file"
)

// ArchiveErrorCode stores the error type that occurred to easy automatize an
// external actual depending on the problem.
type ArchiveErrorCode string

// String translate the error code to a human readable text.
func (a ArchiveErrorCode) String() string {
	switch a {
	case ArchiveErrorCodeTARCreation:
		return "error creating the tar file"
	case ArchiveErrorCodeTARGeneration:
		return "error generating tar file"
	case ArchiveErrorCodeOpeningFile:
		return "error opening file"
	case ArchiveErrorCodeTmpFileCreation:
		return "error creating temporary file"
	case ArchiveErrorCodeCalculateHMACSHA256:
		return "error calculating HMAC-SHA256"
	case ArchiveErrorCodeGenerateRandomNumbers:
		return "error filling iv with random numbers"
	case ArchiveErrorCodeWritingLabel:
		return "error writing label to encrypted file"
	case ArchiveErrorCodeReadingLabel:
		return "error reading encrypted file label"
	case ArchiveErrorCodeWritingAuth:
		return "error writing authentication to encrypted file"
	case ArchiveErrorCodeReadingAuth:
		return "error reading encrypted authentication"
	case ArchiveErrorCodeWritingIV:
		return "error writing iv to encrypted file"
	case ArchiveErrorCodeInitCipher:
		return "error initializing cipher"
	case ArchiveErrorCodeEncryptingFile:
		return "error encrypting file"
	case ArchiveErrorCodeDecryptingFile:
		return "error decrypting file"
	case ArchiveErrorCodeAuthFailed:
		return "encrypted content authentication failed"
	case ArchiveErrorCodeRewindingFile:
		return "error moving to the beggining of the file"
	}

	return "unknown error code"
}

// ArchiveError stores error details from archive operations.
type ArchiveError struct {
	Filename string
	Code     ArchiveErrorCode
	Err      error
}

func newArchiveError(filename string, code ArchiveErrorCode, err error) ArchiveError {
	return ArchiveError{
		Filename: filename,
		Code:     code,
		Err:      errors.NewWithFollowUp(err, 2),
	}
}

// Error returns the error in a human readable format.
func (a ArchiveError) Error() string {
	return a.String()
}

// String translate the error to a human readable text.
func (a ArchiveError) String() string {
	var filename string
	if a.Filename != "" {
		filename = fmt.Sprintf("filename “%s”, ", a.Filename)
	}

	var err string
	if a.Err != nil {
		err = fmt.Sprintf(". details: %s", a.Err)
	}

	return fmt.Sprintf("archive: %s%s%s", filename, a.Code, err)
}

// ArchiveErrorEqual compares two ArchiveError objects. This is useful to
// compare down to the low level errors.
func ArchiveErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := first.(ArchiveError)
	err2, ok2 := second.(ArchiveError)

	if !ok1 || !ok2 {
		return false
	}

	if err1.Filename != err2.Filename || err1.Code != err2.Code {
		return false
	}

	return errors.Equal(err1.Err, err2.Err)
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
	}

	return "unknown error code"
}

// PathError stores error details detected while traversing the path.
type PathError struct {
	Path string
	Code PathErrorCode
	Err  error
}

func newPathError(path string, code PathErrorCode, err error) PathError {
	return PathError{
		Path: path,
		Code: code,
		Err:  errors.NewWithFollowUp(err, 2),
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

	err1, ok1 := first.(PathError)
	err2, ok2 := second.(PathError)

	if !ok1 || !ok2 {
		return false
	}

	if err1.Path != err2.Path || err1.Code != err2.Code {
		return false
	}

	return errors.Equal(err1.Err, err2.Err)
}
