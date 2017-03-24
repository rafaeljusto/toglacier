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
