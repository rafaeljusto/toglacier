package cloud

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	// CloudErrorCodeInitializingSession error connecting to the cloud server to
	// initialize the session.
	CloudErrorCodeInitializingSession CloudErrorCode = "initializing-session"

	// CloudErrorCodeOpeningArchive problem detected while trying to open the
	// archive that contains the backup data.
	CloudErrorCodeOpeningArchive CloudErrorCode = "opening-archive"

	// CloudErrorCodeArchiveInfo error while trying to get information about the
	// archive.
	CloudErrorCodeArchiveInfo CloudErrorCode = "archive-info"

	// CloudErrorCodeSendingArchive problem while uploading the archive to the
	// cloud.
	CloudErrorCodeSendingArchive CloudErrorCode = "sending-archive"

	// CloudErrorCodeComparingChecksums digest mismatch while comparing local
	// archive hash with the cloud archive hash.
	CloudErrorCodeComparingChecksums CloudErrorCode = "comparing-checksums"

	// CloudErrorCodeInitMultipart error while communicating to the cloud that we
	// are going to start sending pieces of the archive.
	CloudErrorCodeInitMultipart CloudErrorCode = "initi-multipart"

	// CloudErrorCodeCompleteMultipart error while signalizing to the cloud that
	// the multipart upload was done.
	CloudErrorCodeCompleteMultipart CloudErrorCode = "complete-multipart"

	// CloudErrorCodeInitJob error while asking to the cloud to initiate an
	// offline task.
	CloudErrorCodeInitJob CloudErrorCode = "init-job"

	// CloudErrorCodeJobComplete error while trying to retrieve an offline task
	// result from the cloud.
	CloudErrorCodeJobComplete CloudErrorCode = "job-complete"

	// CloudErrorCodeRetrievingJob error while trying to get a task status in the
	// cloud.
	CloudErrorCodeRetrievingJob CloudErrorCode = "retrieving-job"

	// CloudErrorCodeJobFailed offline task in the cloud failed to complete.
	CloudErrorCodeJobFailed CloudErrorCode = "job-failed"

	// CloudErrorCodeJobNotFound offline task missing from the cloud.
	CloudErrorCodeJobNotFound CloudErrorCode = "job-not-found"

	// CloudErrorCodeDecodingData problem decoding the data returned from the
	// cloud.
	CloudErrorCodeDecodingData CloudErrorCode = "decoding-data"

	// CloudErrorCodeCreatingArchive error while creating the file that will store
	// the data retrieved from the cloud.
	CloudErrorCodeCreatingArchive CloudErrorCode = "creating-archive"

	// CloudErrorCodeCopyingData problem while filling the created file with the
	// bytes retrieved from the cloud.
	CloudErrorCodeCopyingData CloudErrorCode = "copying-data"

	// CloudErrorCodeRemovingArchive error while removing the archive from the
	// cloud.
	CloudErrorCodeRemovingArchive CloudErrorCode = "removing-archive"
)

// CloudErrorCode stores the error type that occurred while performing any
// operation with the cloud.
type CloudErrorCode string

// String translate the error code to a human readable text.
func (c CloudErrorCode) String() string {
	switch c {
	case CloudErrorCodeInitializingSession:
		return "error initializing cloud session"
	case CloudErrorCodeOpeningArchive:
		return "error opening archive"
	case CloudErrorCodeArchiveInfo:
		return "error retrieving archive information"
	case CloudErrorCodeSendingArchive:
		return "error sending archive to the cloud"
	case CloudErrorCodeComparingChecksums:
		return "error comparing checksums"
	case CloudErrorCodeInitMultipart:
		return "error initializing multipart upload"
	case CloudErrorCodeCompleteMultipart:
		return "error completing multipart upload"
	case CloudErrorCodeInitJob:
		return "error initiating the job"
	case CloudErrorCodeJobComplete:
		return "error retrieving the complete job data"
	case CloudErrorCodeRetrievingJob:
		return "error retrieving the job status"
	case CloudErrorCodeJobFailed:
		return "job failed to complete in the cloud"
	case CloudErrorCodeJobNotFound:
		return "job not found"
	case CloudErrorCodeDecodingData:
		return "error decoding the iventory"
	case CloudErrorCodeCreatingArchive:
		return "error creating backup file"
	case CloudErrorCodeCopyingData:
		return "error copying data to the backup file"
	case CloudErrorCodeRemovingArchive:
		return "error removing backup"
	}

	return "unknown error code"
}

// CloudError stores error details from cloud operations.
type CloudError struct {
	ID   string
	Code CloudErrorCode
	Err  error
}

func newCloudError(id string, code CloudErrorCode, err error) CloudError {
	return CloudError{
		ID:   id,
		Code: code,
		Err:  errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (c CloudError) Error() string {
	return c.String()
}

// String translate the error to a human readable text.
func (c CloudError) String() string {
	var id string
	if c.ID != "" {
		id = fmt.Sprintf("id “%s”, ", c.ID)
	}

	var err string
	if c.Err != nil {
		err = fmt.Sprintf(". details: %s", c.Err)
	}

	return fmt.Sprintf("cloud: %s%s%s", id, c.Code, err)
}

// CloudErrorEqual compares two CloudError objects. This is useful to
// compare down to the low level errors.
func CloudErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(CloudError)
	err2, ok2 := errors.Cause(second).(CloudError)

	if !ok1 || !ok2 {
		return false
	}

	if err1.ID != err2.ID || err1.Code != err2.Code {
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
	// MultipartErrorCodeReadingArchive error reading a piece of the archive.
	MultipartErrorCodeReadingArchive MultipartErrorCode = "reading-archive"

	// MultipartErrorCodeSendingArchive error sending a piece of the archive to
	// the cloud.
	MultipartErrorCodeSendingArchive MultipartErrorCode = "sending-archive"

	// MultipartErrorCodeComparingChecksums error comparing checksums with the
	// cloud of the uploaded archive part.
	MultipartErrorCodeComparingChecksums MultipartErrorCode = "comparing-checksums"
)

// MultipartErrorCode stores the error type that occurred while sending a piece
// of the archive to the cloud.
type MultipartErrorCode string

// String translate the error code to a human readable text.
func (c MultipartErrorCode) String() string {
	switch c {
	case MultipartErrorCodeReadingArchive:
		return "error reading an archive part"
	case MultipartErrorCodeSendingArchive:
		return "error sending an archive part"
	case MultipartErrorCodeComparingChecksums:
		return "error comparing checksums on archive part"
	}

	return "unknown error code"
}

// MultipartError stores error details that occurs when sending pieces of the
// archive to the cloud.
type MultipartError struct {
	Offset int64
	Size   int64
	Code   MultipartErrorCode
	Err    error
}

func newMultipartError(offset, size int64, code MultipartErrorCode, err error) MultipartError {
	return MultipartError{
		Offset: offset,
		Size:   size,
		Code:   code,
		Err:    errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (c MultipartError) Error() string {
	return c.String()
}

// String translate the error to a human readable text.
func (c MultipartError) String() string {
	var err string
	if c.Err != nil {
		err = fmt.Sprintf(". details: %s", c.Err)
	}

	return fmt.Sprintf("cloud: offset %d/%d, %s%s", c.Offset, c.Size, c.Code, err)
}

// MultipartErrorEqual compares two MultipartError objects. This is useful to
// compare down to the low level errors.
func MultipartErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(MultipartError)
	err2, ok2 := errors.Cause(second).(MultipartError)

	if !ok1 || !ok2 {
		return false
	}

	if err1.Offset != err2.Offset || err1.Size != err2.Size || err1.Code != err2.Code {
		return false
	}

	errCause1 := errors.Cause(err1.Err)
	errCause2 := errors.Cause(err2.Err)

	if errCause1 == nil || errCause2 == nil {
		return errCause1 == errCause2
	}

	return errCause1.Error() == errCause2.Error()
}
