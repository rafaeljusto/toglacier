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

func TestFormat_String(t *testing.T) {
	scenarios := []struct {
		description string
		format      report.Format
		expected    string
	}{
		{
			description: "it should convert a plain text format to string correctly",
			format:      report.FormatPlain,
			expected:    "text/plain",
		},
		{
			description: "it should convert a plain html format to string correctly",
			format:      report.FormatHTML,
			expected:    "text/html",
		},
		{
			description: "it should convert an unknown format to plain text string correspondent",
			format:      report.Format("i-dont-exist"),
			expected:    "text/plain",
		},
	}

	for _, scenario := range scenarios {
		converted := scenario.format.String()
		if converted != scenario.expected {
			t.Errorf("wrong conversion. expected “%s” and got “%s”", scenario.expected, converted)
		}
	}
}

func TestBuild(t *testing.T) {
	date := time.Date(2017, 3, 10, 14, 10, 46, 0, time.UTC)

	scenarios := []struct {
		description   string
		reports       []report.Report
		format        report.Format
		expected      string
		expectedError error
	}{
		{
			description: "it should build correctly all types of reports in plain text",
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
					r.Paths = []string{"/data/important-files"}
					r.Durations.Build = 2 * time.Second
					r.Durations.Encrypt = 6 * time.Second
					r.Durations.Send = 6 * time.Minute
					r.Errors = append(r.Errors, errors.New("timeout connecting to aws"))
					return r
				}(),
				func() report.Report {
					r := report.NewSendBackup()
					r.CreatedAt = date
					r.Paths = []string{"/data/important-files"}
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
			format: report.FormatPlain,
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


[2017-03-10 14:10:46] Backups Sent



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
			description: "it should build correctly all types of reports in html",
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
					r.Paths = []string{"/data/important-files"}
					r.Durations.Build = 2 * time.Second
					r.Durations.Encrypt = 6 * time.Second
					r.Durations.Send = 6 * time.Minute
					r.Errors = append(r.Errors, errors.New("timeout connecting to aws"))
					return r
				}(),
				func() report.Report {
					r := report.NewSendBackup()
					r.CreatedAt = date
					r.Paths = []string{"/data/important-files"}
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
			format: report.FormatHTML,
			expected: `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>toglacier report</title>
    <style type="text/css">
      body {
        font-family: "sans-serif";
      }

      .title {
        background: url("https://github.com/rafaeljusto/toglacier/raw/master/toglacier.png") no-repeat 0px -225px / cover;
        height: 400px;
        width: 100%;
      }

      .report {
        border-bottom: 2px solid lightgrey;
        padding: 10px 0px 20px 0px;
      }

      .report h1 {
        background-color: #66ccff;
        border-radius: 10px;
        box-shadow: 10px 10px 5px #888888;
        margin-bottom: 30px;
        padding: 15px;
      }

      .report .date {
        color: grey;
      }
    </style>
  </head>
  <body>
    <section class="title"></section>

    <section class="report">
      <h1>Backups Sent</h1>
      <div class="date">
        2017-03-10 14:10:46
      </div>
      <h2>Backup</h2>
      <div>
        <label>ID:</label>
        <span>AWSID123</span>
      </div>
      <div>
        <label>Date:</label>
        <span>2017-03-10 14:10:45</span>
      </div>
      <div>
        <label>Vault:</label>
        <span>vault</span>
      </div>
      <div>
        <label>Checksum:</label>
        <span>cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705</span>
      </div>
      <div>
        <label>Paths:</label>
        <ul>
          <li>/data/important-files</li>
        </ul>
      </div>
      <h2>Durations</h2>
      <div>
        <label>Build:</label>
        <span>2s</span>
      </div>
      <div>
        <label>Encrypt:</label>
        <span>6s</span>
      </div>
      <div>
        <label>Send:</label>
        <span>6m0s</span>
      </div>
      <h2>Errors</h2>
      <ul>
        <li>timeout connecting to aws</li>
      </ul>
    </section>


		<section class="report">
      <h1>Backups Sent</h1>
      <div class="date">
        2017-03-10 14:10:46
      </div>

      <div>
        <label>Paths:</label>
        <ul>
          <li>/data/important-files</li>
        </ul>
      </div>
      <h2>Durations</h2>
      <div>
        <label>Build:</label>
        <span>2s</span>
      </div>
      <div>
        <label>Encrypt:</label>
        <span>6s</span>
      </div>
      <div>
        <label>Send:</label>
        <span>6m0s</span>
      </div>
      <h2>Errors</h2>
      <ul>
        <li>timeout connecting to aws</li>
      </ul>
    </section>


    <section class="report">
      <h1>List Backup</h1>
      <div class="date">
        2017-03-10 14:10:46
      </div>
      <h2>Durations</h2>
      <div>
        <label>List:</label>
        <span>6h0m0s</span>
      </div>
      <h2>Errors</h2>
      <ul>
        <li>timeout connecting to aws</li>
      </ul>
    </section>


    <section class="report">
      <h1>Remove Old Backups</h1>
      <div class="date">
        2017-03-10 14:10:46
      </div>
      <h2>Backups</h2>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Date</th>
            <th>Vault</th>
            <th>Checksum</th>
          </tr>
        </thead>
        <tbody>
          <td>AWSID123</td>
          <td>2017-03-10 14:10:45</td>
          <td>vault</td>
          <td>cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705</td>
        </tbody>
      </table>
      <h2>Durations</h2>
      <div>
        <label>List:</label>
        <span>6h0m0s</span>
      </div>
      <div>
        <label>Remove:</label>
        <span>2s</span>
      </div>
      <h2>Errors</h2>
      <ul>
        <li>timeout connecting to aws</li>
      </ul>
    </section>


    <section class="report">
      <h1>Test report</h1>
      <div class="date">
        2017-03-10 14:10:46
      </div>
      <p>Testing the notification mechanisms.</p>
      <h2>Errors</h2>
      <ul>
        <li>timeout connecting to aws</li>
      </ul>
    </section>

  </body>
</html>`,
		},
		{
			description: "it should detect an error while building a report",
			reports: []report.Report{
				mockReport{
					mockBuild: func(report.Format) (string, error) {
						return "", &report.Error{
							Code: report.ErrorCodeTemplate,
							Err:  errors.New("error generating report"),
						}
					},
				},
			},
			format: report.FormatPlain,
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

			output, err := report.Build(scenario.format)
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
	mockBuild func(report.Format) (string, error)
}

func (r mockReport) Build(f report.Format) (string, error) {
	return r.mockBuild(f)
}

// Diff is useful to see the difference when comparing two complex types.
func Diff(a, b interface{}) []difflib.DiffRecord {
	return difflib.Diff(strings.SplitAfter(spew.Sdump(a), "\n"), strings.SplitAfter(spew.Sdump(b), "\n"))
}
