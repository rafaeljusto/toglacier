package report_test

import (
	"errors"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/report"
)

func TestReportError_Error(t *testing.T) {
	scenarios := []struct {
		description string
		err         report.ReportError
		expected    string
	}{
		{
			description: "it should show the message with the low level error",
			err: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			expected: "report: error parsing template. details: low level error",
		},
		{
			description: "it should show the correct error message for template parsing problem",
			err:         report.ReportError{Code: report.ReportErrorCodeTemplate},
			expected:    "report: error parsing template",
		},
		{
			description: "it should detect when the code doesn't exist",
			err:         report.ReportError{Code: report.ReportErrorCode("i-dont-exist")},
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

func TestReportErrorEqual(t *testing.T) {
	scenarios := []struct {
		description string
		err1        error
		err2        error
		expected    bool
	}{
		{
			description: "it should detect equal ReportError instances",
			err1: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			expected: true,
		},
		{
			description: "it should detect when the code is different",
			err1: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2: report.ReportError{
				Code: report.ReportErrorCode("unknown-error"),
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when the low level error is different",
			err1: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error 1"),
			},
			err2: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
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
			err1: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			expected: false,
		},
		{
			description: "it should detect when both causes of the error are undefined",
			err1: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
			},
			err2: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
			},
			expected: true,
		},
		{
			description: "it should detect when only one causes of the error is undefined",
			err1: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
			},
			expected: false,
		},
		{
			description: "it should detect when one the error isn't ReportError type",
			err1: report.ReportError{
				Code: report.ReportErrorCodeTemplate,
				Err:  errors.New("low level error"),
			},
			err2:     errors.New("low level error"),
			expected: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if equal := report.ReportErrorEqual(scenario.err1, scenario.err2); equal != scenario.expected {
				t.Errorf("results don't match. expected “%t” and got “%t”", scenario.expected, equal)
			}
		})
	}
}
