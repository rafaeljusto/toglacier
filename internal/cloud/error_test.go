package cloud_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/cloud"
)

func TestError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         *cloud.Error
		expected    string
	}{
		{
			description: "it should show the message with filename and low level error",
			err: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: "cloud: id “AWSID123”, error initializing cloud session. details: low level error",
		},
		{
			description: "it should show the message only with the ID",
			err: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
			},
			expected: "cloud: id “AWSID123”, error initializing cloud session",
		},
		{
			description: "it should show the message only with the low level error",
			err: &cloud.Error{
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: "cloud: error initializing cloud session. details: low level error",
		},
		{
			description: "it should show the correct error message for session initialization problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeInitializingSession},
			expected:    "cloud: error initializing cloud session",
		},
		{
			description: "it should show the correct error message for opening archive problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeOpeningArchive},
			expected:    "cloud: error opening archive",
		},
		{
			description: "it should show the correct error message for retrieving archive problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeArchiveInfo},
			expected:    "cloud: error retrieving archive information",
		},
		{
			description: "it should show the correct error message for retrieving remote archive info problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeRemoteArchiveInfo},
			expected:    "cloud: error retrieving remote archive information",
		},
		{
			description: "it should show the correct error message for sending archive problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeSendingArchive},
			expected:    "cloud: error sending archive to the cloud",
		},
		{
			description: "it should show the correct error message for comparing checksums problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeComparingChecksums},
			expected:    "cloud: error comparing checksums",
		},
		{
			description: "it should show the correct error message for initializing multipart upload problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeInitMultipart},
			expected:    "cloud: error initializing multipart upload",
		},
		{
			description: "it should show the correct error message for completing multipart upload problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeCompleteMultipart},
			expected:    "cloud: error completing multipart upload",
		},
		{
			description: "it should show the correct error message for initializing job problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeInitJob},
			expected:    "cloud: error initiating the job",
		},
		{
			description: "it should show the correct error message for job complete problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeJobComplete},
			expected:    "cloud: error retrieving the complete job data",
		},
		{
			description: "it should show the correct error message for job failed problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeJobFailed},
			expected:    "cloud: job failed to complete in the cloud",
		},
		{
			description: "it should show the correct error message for decoding data problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeDecodingData},
			expected:    "cloud: error decoding the inventory",
		},
		{
			description: "it should show the correct error message for creating archive problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeCreatingArchive},
			expected:    "cloud: error creating backup file",
		},
		{
			description: "it should show the correct error message for copying data problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeCopyingData},
			expected:    "cloud: error copying data to the backup file",
		},
		{
			description: "it should show the correct error message for removing archive problem",
			err:         &cloud.Error{Code: cloud.ErrorCodeRemovingArchive},
			expected:    "cloud: error removing backup",
		},
		{
			description: "it should show the correct error message for action cancelled by the user",
			err:         &cloud.Error{Code: cloud.ErrorCodeCancelled},
			expected:    "cloud: action cancelled by the user",
		},
		{
			description: "it should show the correct error message for iterating in the result set",
			err:         &cloud.Error{Code: cloud.ErrorCodeIterating},
			expected:    "cloud: error iterating in results",
		},
		{
			description: "it should show the correct error message for downloading archive",
			err:         &cloud.Error{Code: cloud.ErrorCodeDownloadingArchive},
			expected:    "cloud: error while downloading the archive",
		},
		{
			description: "it should show the correct error message for closing connection",
			err:         &cloud.Error{Code: cloud.ErrorCodeClosingConnection},
			expected:    "cloud: error closing connection",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         &cloud.Error{Code: cloud.ErrorCode("i-dont-exist")},
			expected:    "cloud: unknown error code",
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
			err1: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			err2: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the ID is different",
			err1: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			err2: &cloud.Error{
				ID:   "AWSID124",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			err2: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeRemovingArchive,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error 1"),
			},
			err2: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
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
			err1: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error 1"),
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't Error type",
			err1: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitializingSession,
				Err:  errors.New("low level error 1"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := cloud.ErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}

func TestMultipartError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         *cloud.MultipartError
		expected    string
	}{
		{
			description: "it should show the message with offset, size and low level error",
			err: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: "cloud: offset 200/400, error reading an archive part. details: low level error",
		},
		{
			description: "it should show the message only with the offset and size",
			err: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
			},
			expected: "cloud: offset 200/400, error reading an archive part",
		},
		{
			description: "it should show the correct error message for reading archve part problem",
			err:         &cloud.MultipartError{Code: cloud.MultipartErrorCodeReadingArchive},
			expected:    "cloud: offset 0/0, error reading an archive part",
		},
		{
			description: "it should show the correct error message for sending archive part problem",
			err:         &cloud.MultipartError{Code: cloud.MultipartErrorCodeSendingArchive},
			expected:    "cloud: offset 0/0, error sending an archive part",
		},
		{
			description: "it should show the correct error message for comparing checksums problem",
			err:         &cloud.MultipartError{Code: cloud.MultipartErrorCodeComparingChecksums},
			expected:    "cloud: offset 0/0, error comparing checksums on archive part",
		},
		{
			description: "it should show the correct error message for user cancelling action",
			err:         &cloud.MultipartError{Code: cloud.MultipartErrorCodeCancelled},
			expected:    "cloud: offset 0/0, action cancelled by the user",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         &cloud.MultipartError{Code: cloud.MultipartErrorCode("i-dont-exist")},
			expected:    "cloud: offset 0/0, unknown error code",
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

func TestMultipartErrorEqual(t *testing.T) {
	scenarios := []struct {
		description string
		err1        error
		err2        error
		expected    bool
	}{
		{
			description: "it should detect equal MultipartError instances",
			err1: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the offset is different",
			err1: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: &cloud.MultipartError{
				Offset: 300,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the size is different",
			err1: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: &cloud.MultipartError{
				Offset: 200,
				Size:   500,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeSendingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error 1"),
			},
			err2: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error 2"),
			},
			expected: false,
		},
		{
			description: "it should detect when both errors are undefined",
			expected:    true,
		},
		{
			description: "it should detect when only one error is undefined",
			err1: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't MultipartError type",
			err1: &cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := cloud.MultipartErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}

func TestJobsError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         *cloud.JobsError
		expected    string
	}{
		{
			description: "it should show the message with jobs and low level error",
			err: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error"),
			},
			expected: "cloud: jobs [AWSID123], error retrieving the job status. details: low level error",
		},
		{
			description: "it should show the message only with the jobs",
			err: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
			},
			expected: "cloud: jobs [AWSID123], error retrieving the job status",
		},
		{
			description: "it should show the message only with the low level error",
			err: &cloud.JobsError{
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error"),
			},
			expected: "cloud: error retrieving the job status. details: low level error",
		},
		{
			description: "it should show the correct error message for retrieving job status problem",
			err:         &cloud.JobsError{Code: cloud.JobsErrorCodeRetrievingJob},
			expected:    "cloud: error retrieving the job status",
		},
		{
			description: "it should show the correct error message for job not found problem",
			err:         &cloud.JobsError{Code: cloud.JobsErrorCodeJobNotFound},
			expected:    "cloud: job not found",
		},
		{
			description: "it should show the correct error message for cancelled action",
			err:         &cloud.JobsError{Code: cloud.JobsErrorCodeCancelled},
			expected:    "cloud: action cancelled by the user",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         &cloud.JobsError{Code: cloud.JobsErrorCode("i-dont-exist")},
			expected:    "cloud: unknown error code",
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

func TestJobsErrorEqual(t *testing.T) {
	scenarios := []struct {
		description string
		err1        error
		err2        error
		expected    bool
	}{
		{
			description: "it should detect equal JobsError instances",
			err1: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error"),
			},
			err2: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the job is different",
			err1: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error"),
			},
			err2: &cloud.JobsError{
				Jobs: []string{"AWSID124"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error"),
			},
			err2: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeJobNotFound,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error 1"),
			},
			err2: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
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
			err1: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error 1"),
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't JobsError type",
			err1: &cloud.JobsError{
				Jobs: []string{"AWSID123"},
				Code: cloud.JobsErrorCodeRetrievingJob,
				Err:  errors.New("low level error 1"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := cloud.JobsErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
