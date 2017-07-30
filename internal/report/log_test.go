package report_test

import (
	"reflect"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/log"
	"github.com/rafaeljusto/toglacier/internal/report"
)

func TestLog_Debug(t *testing.T) {
	scenarios := []struct {
		description string
		logger      log.Logger
		args        []interface{}
		expected    []string
	}{
		{
			description: "it should intercept a debug message correctly",
			logger: mockLogger{
				mockDebug: func(args ...interface{}) {},
			},
			args: []interface{}{"this is a message"},
			expected: []string{
				"this is a message",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			logger := report.NewLogger(scenario.logger)
			logger.Debug(scenario.args...)

			if !reflect.DeepEqual(scenario.expected, logger.Debugs) {
				t.Errorf("logs don't match.\n%s", Diff(scenario.expected, logger.Debugs))
			}
		})
	}
}

func TestLog_Debugf(t *testing.T) {
	scenarios := []struct {
		description string
		logger      log.Logger
		format      string
		args        []interface{}
		expected    []string
	}{
		{
			description: "it should intercept a debug message correctly",
			logger: mockLogger{
				mockDebugf: func(format string, args ...interface{}) {},
			},
			format: "this is a %s",
			args:   []interface{}{"message"},
			expected: []string{
				"this is a message",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			logger := report.NewLogger(scenario.logger)
			logger.Debugf(scenario.format, scenario.args...)

			if !reflect.DeepEqual(scenario.expected, logger.Debugs) {
				t.Errorf("logs don't match.\n%s", Diff(scenario.expected, logger.Debugs))
			}
		})
	}
}

func TestLog_Info(t *testing.T) {
	scenarios := []struct {
		description string
		logger      log.Logger
		args        []interface{}
		expected    []string
	}{
		{
			description: "it should intercept a info message correctly",
			logger: mockLogger{
				mockInfo: func(args ...interface{}) {},
			},
			args: []interface{}{"this is a message"},
			expected: []string{
				"this is a message",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			logger := report.NewLogger(scenario.logger)
			logger.Info(scenario.args...)

			if !reflect.DeepEqual(scenario.expected, logger.Infos) {
				t.Errorf("logs don't match.\n%s", Diff(scenario.expected, logger.Infos))
			}
		})
	}
}

func TestLog_Infof(t *testing.T) {
	scenarios := []struct {
		description string
		logger      log.Logger
		format      string
		args        []interface{}
		expected    []string
	}{
		{
			description: "it should intercept a info message correctly",
			logger: mockLogger{
				mockInfof: func(format string, args ...interface{}) {},
			},
			format: "this is a %s",
			args:   []interface{}{"message"},
			expected: []string{
				"this is a message",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			logger := report.NewLogger(scenario.logger)
			logger.Infof(scenario.format, scenario.args...)

			if !reflect.DeepEqual(scenario.expected, logger.Infos) {
				t.Errorf("logs don't match.\n%s", Diff(scenario.expected, logger.Infos))
			}
		})
	}
}

func TestLog_Warning(t *testing.T) {
	scenarios := []struct {
		description string
		logger      log.Logger
		args        []interface{}
		expected    []string
	}{
		{
			description: "it should intercept a warning message correctly",
			logger: mockLogger{
				mockWarning: func(args ...interface{}) {},
			},
			args: []interface{}{"this is a message"},
			expected: []string{
				"this is a message",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			logger := report.NewLogger(scenario.logger)
			logger.Warning(scenario.args...)

			if !reflect.DeepEqual(scenario.expected, logger.Warnings) {
				t.Errorf("logs don't match.\n%s", Diff(scenario.expected, logger.Warnings))
			}
		})
	}
}

func TestLog_Warningf(t *testing.T) {
	scenarios := []struct {
		description string
		logger      log.Logger
		format      string
		args        []interface{}
		expected    []string
	}{
		{
			description: "it should intercept a warning message correctly",
			logger: mockLogger{
				mockWarningf: func(format string, args ...interface{}) {},
			},
			format: "this is a %s",
			args:   []interface{}{"message"},
			expected: []string{
				"this is a message",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			logger := report.NewLogger(scenario.logger)
			logger.Warningf(scenario.format, scenario.args...)

			if !reflect.DeepEqual(scenario.expected, logger.Warnings) {
				t.Errorf("logs don't match.\n%s", Diff(scenario.expected, logger.Warnings))
			}
		})
	}
}

type mockLogger struct {
	mockDebug    func(args ...interface{})
	mockDebugf   func(format string, args ...interface{})
	mockInfo     func(args ...interface{})
	mockInfof    func(format string, args ...interface{})
	mockWarning  func(args ...interface{})
	mockWarningf func(format string, args ...interface{})
}

func (m mockLogger) Debug(args ...interface{}) {
	m.mockDebug(args...)
}

func (m mockLogger) Debugf(format string, args ...interface{}) {
	m.mockDebugf(format, args...)
}

func (m mockLogger) Info(args ...interface{}) {
	m.mockInfo(args...)
}

func (m mockLogger) Infof(format string, args ...interface{}) {
	m.mockInfof(format, args...)
}

func (m mockLogger) Warning(args ...interface{}) {
	m.mockWarning(args...)
}

func (m mockLogger) Warningf(format string, args ...interface{}) {
	m.mockWarningf(format, args...)
}
