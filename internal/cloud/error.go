package cloud

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

const (
	// ErrorCodeInitializingSession error connecting to the cloud server to
	// initialize the session.
	ErrorCodeInitializingSession ErrorCode = "initializing-session"

	// ErrorCodeOpeningArchive problem detected while trying to open the archive
	// that contains the backup data.
	ErrorCodeOpeningArchive ErrorCode = "opening-archive"

	// ErrorCodeArchiveInfo error while trying to get information about the
	// archive.
	ErrorCodeArchiveInfo ErrorCode = "archive-info"

	// ErrorCodeSendingArchive problem while uploading the archive to the cloud.
	ErrorCodeSendingArchive ErrorCode = "sending-archive"

	// ErrorCodeComparingChecksums digest mismatch while comparing local archive
	// hash with the cloud archive hash.
	ErrorCodeComparingChecksums ErrorCode = "comparing-checksums"

	// ErrorCodeInitMultipart error while communicating to the cloud that we are
	// going to start sending pieces of the archive.
	ErrorCodeInitMultipart ErrorCode = "initi-multipart"

	// ErrorCodeCompleteMultipart error while signalizing to the cloud that the
	// multipart upload was done.
	ErrorCodeCompleteMultipart ErrorCode = "complete-multipart"

	// ErrorCodeInitJob error while asking to the cloud to initiate an offline
	// task.
	ErrorCodeInitJob ErrorCode = "init-job"

	// ErrorCodeJobComplete error while trying to retrieve an offline task result
	// from the cloud.
	ErrorCodeJobComplete ErrorCode = "job-complete"

	// ErrorCodeJobFailed offline task in the cloud failed to complete.
	ErrorCodeJobFailed ErrorCode = "job-failed"

	// ErrorCodeDecodingData problem decoding the data returned from the cloud.
	ErrorCodeDecodingData ErrorCode = "decoding-data"

	// ErrorCodeCreatingArchive error while creating the file that will store the
	// data retrieved from the cloud.
	ErrorCodeCreatingArchive ErrorCode = "creating-archive"

	// ErrorCodeCopyingData problem while filling the created file with the bytes
	// retrieved from the cloud.
	ErrorCodeCopyingData ErrorCode = "copying-data"

	// ErrorCodeRemovingArchive error while removing the archive from the cloud.
	ErrorCodeRemovingArchive ErrorCode = "removing-archive"

	// ErrorCodeCancelled action cancelled by the user.
	ErrorCodeCancelled ErrorCode = "cancelled"
)

// ErrorCode stores the error type that occurred while performing any operation
// with the cloud.
type ErrorCode string

var errorCodeString = map[ErrorCode]string{
	ErrorCodeInitializingSession: "error initializing cloud session",
	ErrorCodeOpeningArchive:      "error opening archive",
	ErrorCodeArchiveInfo:         "error retrieving archive information",
	ErrorCodeSendingArchive:      "error sending archive to the cloud",
	ErrorCodeComparingChecksums:  "error comparing checksums",
	ErrorCodeInitMultipart:       "error initializing multipart upload",
	ErrorCodeCompleteMultipart:   "error completing multipart upload",
	ErrorCodeInitJob:             "error initiating the job",
	ErrorCodeJobComplete:         "error retrieving the complete job data",
	ErrorCodeJobFailed:           "job failed to complete in the cloud",
	ErrorCodeDecodingData:        "error decoding the iventory",
	ErrorCodeCreatingArchive:     "error creating backup file",
	ErrorCodeCopyingData:         "error copying data to the backup file",
	ErrorCodeRemovingArchive:     "error removing backup",
	ErrorCodeCancelled:           "action cancelled by the user",
}

// String translate the error code to a human readable text.
func (e ErrorCode) String() string {
	if msg, ok := errorCodeString[e]; ok {
		return msg
	}

	return "unknown error code"
}

// Error stores error details from cloud operations.
type Error struct {
	ID   string
	Code ErrorCode
	Err  error
}

func newError(id string, code ErrorCode, err error) *Error {
	return &Error{
		ID:   id,
		Code: code,
		Err:  errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (e Error) Error() string {
	return e.String()
}

// String translate the error to a human readable text.
func (e Error) String() string {
	var id string
	if e.ID != "" {
		id = fmt.Sprintf("id “%s”, ", e.ID)
	}

	var err string
	if e.Err != nil {
		err = fmt.Sprintf(". details: %s", e.Err)
	}

	return fmt.Sprintf("cloud: %s%s%s", id, e.Code, err)
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

	// MultipartErrorCodeCancelled action cancelled by the user.
	MultipartErrorCodeCancelled MultipartErrorCode = "cancelled"
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
	case MultipartErrorCodeCancelled:
		return "action cancelled by the user"
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

func newMultipartError(offset, size int64, code MultipartErrorCode, err error) *MultipartError {
	return &MultipartError{
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

	err1, ok1 := errors.Cause(first).(*MultipartError)
	err2, ok2 := errors.Cause(second).(*MultipartError)

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

const (
	// JobsErrorCodeRetrievingJob error while trying to get a task status in the
	// cloud.
	JobsErrorCodeRetrievingJob JobsErrorCode = "retrieving-job"

	// JobsErrorCodeJobNotFound offline task missing from the cloud.
	JobsErrorCodeJobNotFound JobsErrorCode = "job-not-found"

	// JobsErrorCodeCancelled action cancelled by the user.
	JobsErrorCodeCancelled JobsErrorCode = "cancelled"
)

// JobsErrorCode stores the error type that occurred while performing any operation
// with the cloud.
type JobsErrorCode string

var jobsErrorCodeString = map[JobsErrorCode]string{
	JobsErrorCodeRetrievingJob: "error retrieving the job status",
	JobsErrorCodeJobNotFound:   "job not found",
	JobsErrorCodeCancelled:     "action cancelled by the user",
}

// String translate the error code to a human readable text.
func (e JobsErrorCode) String() string {
	if msg, ok := jobsErrorCodeString[e]; ok {
		return msg
	}

	return "unknown error code"
}

// JobsError stores error details that occurs when monitoring asynchronous jobs
// in the cloud.
type JobsError struct {
	Jobs []string
	Code JobsErrorCode
	Err  error
}

func newJobsError(jobs []string, code JobsErrorCode, err error) *JobsError {
	return &JobsError{
		Jobs: jobs,
		Code: code,
		Err:  errors.WithStack(err),
	}
}

// Error returns the error in a human readable format.
func (c JobsError) Error() string {
	return c.String()
}

// String translate the error to a human readable text.
func (c JobsError) String() string {
	var jobs string
	if c.Jobs != nil {
		jobs = fmt.Sprintf("jobs %v, ", c.Jobs)
	}

	var err string
	if c.Err != nil {
		err = fmt.Sprintf(". details: %s", c.Err)
	}

	return fmt.Sprintf("cloud: %s%s%s", jobs, c.Code, err)
}

// JobsErrorEqual compares two JobsError objects. This is useful to compare down
// to the low level errors.
func JobsErrorEqual(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := errors.Cause(first).(*JobsError)
	err2, ok2 := errors.Cause(second).(*JobsError)

	if !ok1 || !ok2 {
		return false
	}

	if !reflect.DeepEqual(err1.Jobs, err2.Jobs) || err1.Code != err2.Code {
		return false
	}

	errCause1 := errors.Cause(err1.Err)
	errCause2 := errors.Cause(err2.Err)

	if errCause1 == nil || errCause2 == nil {
		return errCause1 == errCause2
	}

	return errCause1.Error() == errCause2.Error()
}
