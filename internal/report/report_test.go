package report_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aryann/difflib"
	"github.com/davecgh/go-spew/spew"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/report"
)

func TestBuild(t *testing.T) {
	date := time.Date(2017, 3, 10, 14, 10, 46, 0, time.UTC)

	scenarios := []struct {
		description   string
		reports       []report.Report
		expected      string
		expectedError error
	}{
		{
			description: "it should build correctly all types of reports",
			reports: []report.Report{
				func() report.Report {
					r := report.NewSendBackup()
					r.CreatedAt = date
					r.Backup = cloud.Backup{
						ID:        "AWSID123",
						CreatedAt: date.Add(-time.Second),
						VaultName: "vault",
						Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
					}
					r.Backup.Paths = []string{"/data/important-files"}
					r.Durations.Build = 2 * time.Second
					r.Durations.Encrypt = 6 * time.Second
					r.Durations.Send = 6 * time.Minute
					r.Errors = append(r.Errors, errors.New("timeout connecting to aws"))
					return r
				}(),
				func() report.Report {
					r := report.NewListBackups()
					r.CreatedAt = date
					r.Durations.List = 6 * time.Hour
					r.Errors = append(r.Errors, errors.New("timeout connecting to aws"))
					return r
				}(),
				func() report.Report {
					r := report.NewRemoveOldBackups()
					r.CreatedAt = date
					r.Backups = []cloud.Backup{
						{
							ID:        "AWSID123",
							CreatedAt: date.Add(-time.Second),
							VaultName: "vault",
							Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
						},
					}
					r.Durations.List = 6 * time.Hour
					r.Durations.Remove = 2 * time.Second
					r.Errors = append(r.Errors, errors.New("timeout connecting to aws"))
					return r
				}(),
				func() report.Report {
					r := report.NewTest()
					r.CreatedAt = date
					r.Errors = append(r.Errors, errors.New("timeout connecting to aws"))
					return r
				}(),
			},
			expected: `[2017-03-10 14:10:46] Backups Sent

  Backup
  ------

    ID:          AWSID123
    Date:        2017-03-10 14:10:45
    Vault:       vault
    Checksum:    cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705
    Paths:       /data/important-files

  Durations
  ---------

    Build:       2s
    Encrypt:     6s
    Send:        6m0s

  Errors
  ------

    * timeout connecting to aws


[2017-03-10 14:10:46] List Backup

  Durations
  ---------

    List:        6h0m0s

  Errors
  ------

    * timeout connecting to aws


[2017-03-10 14:10:46] Remove Old Backups

  Backups
  -------

    * ID:        AWSID123
      Date:      2017-03-10 14:10:45
      Vault:     vault
      Checksum:  cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705

  Durations
  ---------

    List:        6h0m0s
    Remove:      2s

  Errors
  ------

    * timeout connecting to aws


[2017-03-10 14:10:46] Test report

  Testing the notification mechanisms.

  Errors
  ------

    * timeout connecting to aws`,
		},
		{
			description: "it should detect an error while building a report",
			reports: []report.Report{
				mockReport{
					mockBuild: func() (string, error) {
						return "", &report.Error{
							Code: report.ErrorCodeTemplate,
							Err:  errors.New("error generating report"),
						}
					},
				},
			},
			expectedError: &report.Error{
				Code: report.ErrorCodeTemplate,
				Err:  errors.New("error generating report"),
			},
		},
	}

	for _, scenario := range scenarios {
		report.Clear()

		t.Run(scenario.description, func(t *testing.T) {
			for _, r := range scenario.reports {
				report.Add(r)
			}

			output, err := report.Build()
			output = strings.TrimSpace(output)

			outputLines := strings.Split(output, "\n")
			for i := range outputLines {
				outputLines[i] = strings.TrimSpace(outputLines[i])
			}

			scenario.expected = strings.TrimSpace(scenario.expected)
			expectedLines := strings.Split(scenario.expected, "\n")
			for i := range expectedLines {
				expectedLines[i] = strings.TrimSpace(expectedLines[i])
			}

			if !reflect.DeepEqual(expectedLines, outputLines) {
				t.Errorf("output don't match.\n%s", Diff(expectedLines, outputLines))
			}

			if !report.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

type mockReport struct {
	mockBuild func() (string, error)
}

func (r mockReport) Build() (string, error) {
	return r.mockBuild()
}

// Diff is useful to see the difference when comparing two complex types.
func Diff(a, b interface{}) []difflib.DiffRecord {
	return difflib.Diff(strings.SplitAfter(spew.Sdump(a), "\n"), strings.SplitAfter(spew.Sdump(b), "\n"))
}
