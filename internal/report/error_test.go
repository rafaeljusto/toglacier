package report_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/report"
)

func TestError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         *report.Error
		expected    string
	}{
		{
			description: "it should show the message with the low level error",
			err: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			expected: "report: error parsing template. details: low level error",
		},
		{
			description: "it should show the correct error message for template parsing problem",
			err:         &report.Error{Code: report.ErrorCodeTemplate},
			expected:    "report: error parsing template",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         &report.Error{Code: report.ErrorCode("i-dont-exist")},
			expected:    "report: unknown error code",
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
			err1: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the code is different",
			err1: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2: &report.Error{
				Code: report.ErrorCode("unknown-error"),
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error 1"),
			},
			err2: &report.Error{
				Code: report.ErrorCodeTemplate,
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
			err1: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when both causes of the error are undefined",
			err1: &report.Error{
				Code: report.ErrorCodeTemplate,
			},
			err2: &report.Error{
				Code: report.ErrorCodeTemplate,
			},
			expected: true,
		},
		{
			description: "it should detect when only one causes of the error is undefined",
			err1: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2: &report.Error{
				Code: report.ErrorCodeTemplate,
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't Error type",
			err1: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := report.ErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
