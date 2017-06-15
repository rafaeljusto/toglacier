package toglacier_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier"
)

func TestError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         *toglacier.Error
		expected    string
	}{
		{
			description: "it should show the message with paths and low level error",
			err: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			expected: "toglacier: paths [/path1/important, /path2/also-important], too many files modified, aborting for precaution. details: low level error",
		},
		{
			description: "it should show the message only with the paths",
			err: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
			},
			expected: "toglacier: paths [/path1/important, /path2/also-important], too many files modified, aborting for precaution",
		},
		{
			description: "it should show the message only with the low level error",
			err: &toglacier.Error{
				Code: toglacier.ErrorCodeModifyTolerance,
				Err:  errors.New("low level error"),
			},
			expected: "toglacier: too many files modified, aborting for precaution. details: low level error",
		},
		{
			description: "it should show the correct error message for too many modified files",
			err:         &toglacier.Error{Code: toglacier.ErrorCodeModifyTolerance},
			expected:    "toglacier: too many files modified, aborting for precaution",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         &toglacier.Error{Code: toglacier.ErrorCode("i-dont-exist")},
			expected:    "toglacier: unknown error code",
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
			err1: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			err2: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the paths is different",
			err1: &toglacier.Error{
				Paths: []string{"/path1/important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			err2: &toglacier.Error{
				Paths: []string{"/path2/important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the code is different",
			err1: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			err2: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCode("some-another-error"),
				Err:   errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error 1"),
			},
			err2: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error 2"),
			},
			expected: false,
		},
		{
			description: "it should detect when both errors are undefined",
			expected:    true,
		},
		{
			description: "it should detect when only one error is undefined",
			err1: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when both causes of the error are undefined",
			err1: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
			},
			err2: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
			},
			expected: true,
		},
		{
			description: "it should detect when only one causes of the error is undefined",
			err1: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			err2: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't Error type",
			err1: &toglacier.Error{
				Paths: []string{"/path1/important", "/path2/also-important"},
				Code:  toglacier.ErrorCodeModifyTolerance,
				Err:   errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := toglacier.ErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
