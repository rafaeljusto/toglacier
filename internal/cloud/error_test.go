package cloud_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/cloud"
)

func TestCloudError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         cloud.CloudError
		expected    string
	}{
		{
			description: "it should show the message with filename and low level error",
			err: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: "cloud: id “AWSID123”, error initializing cloud session. details: low level error",
		},
		{
			description: "it should show the message only with the ID",
			err: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
			},
			expected: "cloud: id “AWSID123”, error initializing cloud session",
		},
		{
			description: "it should show the message only with the low level error",
			err: cloud.CloudError{
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: "cloud: error initializing cloud session. details: low level error",
		},
		{
			description: "it should show the correct error message for session initialization problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeInitializingSession},
			expected:    "cloud: error initializing cloud session",
		},
		{
			description: "it should show the correct error message for opening archive problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeOpeningArchive},
			expected:    "cloud: error opening archive",
		},
		{
			description: "it should show the correct error message for retrieving archive problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeArchiveInfo},
			expected:    "cloud: error retrieving archive information",
		},
		{
			description: "it should show the correct error message for sending archive problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeSendingArchive},
			expected:    "cloud: error sending archive to the cloud",
		},
		{
			description: "it should show the correct error message for comparing checksums problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeComparingChecksums},
			expected:    "cloud: error comparing checksums",
		},
		{
			description: "it should show the correct error message for initializing multipart upload problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeInitMultipart},
			expected:    "cloud: error initializing multipart upload",
		},
		{
			description: "it should show the correct error message for completing multipart upload problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeCompleteMultipart},
			expected:    "cloud: error completing multipart upload",
		},
		{
			description: "it should show the correct error message for initializing job problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeInitJob},
			expected:    "cloud: error initiating the job",
		},
		{
			description: "it should show the correct error message for job complete problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeJobComplete},
			expected:    "cloud: error retrieving the complete job data",
		},
		{
			description: "it should show the correct error message for retrieving job status problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeRetrievingJob},
			expected:    "cloud: error retrieving the job status",
		},
		{
			description: "it should show the correct error message for job failed problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeJobFailed},
			expected:    "cloud: job failed to complete in the cloud",
		},
		{
			description: "it should show the correct error message for job not found problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeJobNotFound},
			expected:    "cloud: job not found",
		},
		{
			description: "it should show the correct error message for decoding data problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeDecodingData},
			expected:    "cloud: error decoding the iventory",
		},
		{
			description: "it should show the correct error message for creating archive problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeCreatingArchive},
			expected:    "cloud: error creating backup file",
		},
		{
			description: "it should show the correct error message for copying data problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeCopyingData},
			expected:    "cloud: error copying data to the backup file",
		},
		{
			description: "it should show the correct error message for removing archive problem",
			err:         cloud.CloudError{Code: cloud.CloudErrorCodeRemovingArchive},
			expected:    "cloud: error removing backup",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         cloud.CloudError{Code: cloud.CloudErrorCode("i-dont-exist")},
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

func TestCloudErrorEqual(t *testing.T) {
	scenarios := []struct {
		description string
		err1        error
		err2        error
		expected    bool
	}{
		{
			description: "it should detect equal CloudError instances",
			err1: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			err2: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the ID is different",
			err1: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			err2: cloud.CloudError{
				ID:   "AWSID124",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error"),
			},
			err2: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeRemovingArchive,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error 1"),
			},
			err2: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
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
			err1: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error 1"),
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't CloudError type",
			err1: cloud.CloudError{
				ID:   "AWSID123",
				Code: cloud.CloudErrorCodeInitializingSession,
				Err:  errors.New("low level error 1"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := cloud.CloudErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}

func TestMultipartError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         cloud.MultipartError
		expected    string
	}{
		{
			description: "it should show the message with offset, size and low level error",
			err: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: "cloud: offset 200/400, error reading an archive part. details: low level error",
		},
		{
			description: "it should show the message only with the offset and size",
			err: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
			},
			expected: "cloud: offset 200/400, error reading an archive part",
		},
		{
			description: "it should show the correct error message for reading archve part problem",
			err:         cloud.MultipartError{Code: cloud.MultipartErrorCodeReadingArchive},
			expected:    "cloud: offset 0/0, error reading an archive part",
		},
		{
			description: "it should show the correct error message for sending archive part problem",
			err:         cloud.MultipartError{Code: cloud.MultipartErrorCodeSendingArchive},
			expected:    "cloud: offset 0/0, error sending an archive part",
		},
		{
			description: "it should show the correct error message for comparing checksums problem",
			err:         cloud.MultipartError{Code: cloud.MultipartErrorCodeComparingChecksums},
			expected:    "cloud: offset 0/0, error comparing checksums on archive part",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         cloud.MultipartError{Code: cloud.MultipartErrorCode("i-dont-exist")},
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
			err1: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the offset is different",
			err1: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: cloud.MultipartError{
				Offset: 300,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the size is different",
			err1: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: cloud.MultipartError{
				Offset: 200,
				Size:   500,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			err2: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeSendingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error 1"),
			},
			err2: cloud.MultipartError{
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
			err1: cloud.MultipartError{
				Offset: 200,
				Size:   400,
				Code:   cloud.MultipartErrorCodeReadingArchive,
				Err:    errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't MultipartError type",
			err1: cloud.MultipartError{
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
