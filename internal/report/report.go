// Package report build a text with all actions performed by the tool. As the
// tool can work in background, it is useful to periodically retrieve a report
// with all actions.
package report

import (
	"bytes"
	"fmt"
	"sync"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/cloud"
)

var (
	reports     []Report
	reportsLock sync.Mutex
)

const (
	// FormatPlain send e-mail containing only ascii characters.
	FormatPlain Format = "plain"

	// FormatHTML send e-mail with a HTML structure for better presentation
	// of the content.
	FormatHTML Format = "html"
)

// Format defines the format used in the e-mail content.
type Format string

// String gives the string representation that can be used in e-mail headers.
func (f Format) String() string {
	switch f {
	case FormatPlain:
		return "text/plain"
	case FormatHTML:
		return "text/html"
	}

	return "text/plain"
}

const formatHTMLPrefix = `<!DOCTYPE html>
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
`

const formatHTMLSuffix = `  </body>
</html>`

// Report is the contract that every report must respect so it can be included
// in the notification engine.
type Report interface {
	Build(Format) (string, error)
}

type basic struct {
	CreatedAt time.Time
	Errors    []error
}

func newBasic() basic {
	return basic{
		CreatedAt: time.Now(),
	}
}

// SendBackup stores all useful information of an uploaded backup. It includes
// performance data for system improvements.
type SendBackup struct {
	basic

	Backup    cloud.Backup
	Paths     []string
	Durations struct {
		Build   time.Duration
		Encrypt time.Duration
		Send    time.Duration
	}
}

// NewSendBackup initialize a new report item for the backup upload action.
func NewSendBackup() SendBackup {
	return SendBackup{
		basic: newBasic(),
	}
}

// Build creates a report with details of an uploaded backup to the cloud. On
// error it will return an Error type encapsulated in a traceable error. To
// retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *report.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (s SendBackup) Build(f Format) (string, error) {
	var tmpl string

	switch f {
	case FormatHTML:
		tmpl = `
    <section class="report">
      <h1>Backups Sent</h1>
      <div class="date">
        {{.CreatedAt.Format "2006-01-02 15:04:05"}}
      </div>
      <h2>Backup</h2>
      <div>
        <label>ID:</label>
        <span>{{.Backup.ID}}</span>
      </div>
      <div>
        <label>Date:</label>
        <span>{{.Backup.CreatedAt.Format "2006-01-02 15:04:05"}}</span>
      </div>
      <div>
        <label>Vault:</label>
        <span>{{.Backup.VaultName}}</span>
      </div>
      <div>
        <label>Checksum:</label>
        <span>{{.Backup.Checksum}}</span>
      </div>
      <div>
        <label>Paths:</label>
        <ul>
          {{range $path := .Paths}}
          <li>{{$path}}</li>
          {{- end}}
        </ul>
      </div>
      <h2>Durations</h2>
      <div>
        <label>Build:</label>
        <span>{{.Durations.Build}}</span>
      </div>
      <div>
        <label>Encrypt:</label>
        <span>{{.Durations.Encrypt}}</span>
      </div>
      <div>
        <label>Send:</label>
        <span>{{.Durations.Send}}</span>
      </div>
      {{if .Errors -}}
      <h2>Errors</h2>
      <ul>
        {{range $err := .Errors}}
        <li>{{$err}}</li>
        {{- end -}}
      </ul>
      {{- end}}
    </section>
  `

	case FormatPlain:
		fallthrough

	default:
		tmpl = `
[{{.CreatedAt.Format "2006-01-02 15:04:05"}}] Backups Sent

  Backup
  ------

    ID:          {{.Backup.ID}}
    Date:        {{.Backup.CreatedAt.Format "2006-01-02 15:04:05"}}
    Vault:       {{.Backup.VaultName}}
    Checksum:    {{.Backup.Checksum}}
    Paths:       {{range $path := .Paths}}{{$path}} {{end}}

  Durations
  ---------

    Build:       {{.Durations.Build}}
    Encrypt:     {{.Durations.Encrypt}}
    Send:        {{.Durations.Send}}

  {{if .Errors -}}
  Errors
  ------
    {{range $err := .Errors}}
    * {{$err}}
    {{- end -}}
  {{- end}}
  `
	}

	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, s); err != nil {
		return "", errors.WithStack(newError(ErrorCodeTemplate, err))
	}
	return buffer.String(), nil
}

// ListBackups stores statistics and errors when the remote backups information
// are retrieved.
type ListBackups struct {
	basic

	Durations struct {
		List time.Duration
	}
}

// NewListBackups initialize a new report item to retrieve the remote backups.
func NewListBackups() ListBackups {
	return ListBackups{
		basic: newBasic(),
	}
}

// Build creates a report with details of a remote backups listing. On
// error it will return an Error type encapsulated in a traceable error. To
// retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *report.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (l ListBackups) Build(f Format) (string, error) {
	var tmpl string

	switch f {
	case FormatHTML:
		tmpl = `
    <section class="report">
      <h1>List Backup</h1>
      <div class="date">
        {{.CreatedAt.Format "2006-01-02 15:04:05"}}
      </div>
      <h2>Durations</h2>
      <div>
        <label>List:</label>
        <span>{{.Durations.List}}</span>
      </div>
      {{if .Errors -}}
      <h2>Errors</h2>
      <ul>
        {{range $err := .Errors}}
        <li>{{$err}}</li>
        {{- end -}}
      </ul>
      {{- end}}
    </section>
  `

	case FormatPlain:
		fallthrough

	default:
		tmpl = `
[{{.CreatedAt.Format "2006-01-02 15:04:05"}}] List Backup

  Durations
  ---------

    List:        {{.Durations.List}}

  {{if .Errors -}}
  Errors
  ------
    {{range $err := .Errors}}
    * {{$err}}
    {{- end -}}
  {{- end}}
  `
	}

	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, l); err != nil {
		return "", errors.WithStack(newError(ErrorCodeTemplate, err))
	}
	return buffer.String(), nil
}

// RemoveOldBackups stores useful information about the removed backups,
// including performance issues.
type RemoveOldBackups struct {
	basic

	Backups   []cloud.Backup
	Durations struct {
		List   time.Duration
		Remove time.Duration
	}
}

// NewRemoveOldBackups initialize a new report item for removing the old
// backups.
func NewRemoveOldBackups() RemoveOldBackups {
	return RemoveOldBackups{
		basic: newBasic(),
	}
}

// Build creates a report with details of old backups removal procedure. On
// error it will return an Error type encapsulated in a traceable error. To
// retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *report.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (r RemoveOldBackups) Build(f Format) (string, error) {
	var tmpl string

	switch f {
	case FormatHTML:
		tmpl = `
    <section class="report">
      <h1>Remove Old Backups</h1>
      <div class="date">
        {{.CreatedAt.Format "2006-01-02 15:04:05"}}
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
          {{range $backup := .Backups}}
          <td>{{$backup.ID}}</td>
          <td>{{$backup.CreatedAt.Format "2006-01-02 15:04:05"}}</td>
          <td>{{$backup.VaultName}}</td>
          <td>{{$backup.Checksum}}</td>
          {{- end}}
        </tbody>
      </table>
      <h2>Durations</h2>
      <div>
        <label>List:</label>
        <span>{{.Durations.List}}</span>
      </div>
      <div>
        <label>Remove:</label>
        <span>{{.Durations.Remove}}</span>
      </div>
      {{if .Errors -}}
      <h2>Errors</h2>
      <ul>
        {{range $err := .Errors}}
        <li>{{$err}}</li>
        {{- end -}}
      </ul>
      {{- end}}
    </section>
  `

	case FormatPlain:
		fallthrough

	default:
		tmpl = `
[{{.CreatedAt.Format "2006-01-02 15:04:05"}}] Remove Old Backups

  Backups
  -------
    {{range $backup := .Backups}}
    * ID:        {{$backup.ID}}
      Date:      {{$backup.CreatedAt.Format "2006-01-02 15:04:05"}}
      Vault:     {{$backup.VaultName}}
      Checksum:  {{$backup.Checksum}}
    {{- end}}

  Durations
  ---------

    List:        {{.Durations.List}}
    Remove:      {{.Durations.Remove}}

  {{if .Errors -}}
  Errors
  ------
    {{range $err := .Errors}}
    * {{$err}}
    {{- end -}}
  {{- end}}
  `
	}

	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, r); err != nil {
		return "", errors.WithStack(newError(ErrorCodeTemplate, err))
	}
	return buffer.String(), nil
}

// Test is a simple test report only to check if everything is working well.
type Test struct {
	basic
}

// NewTest initialize a new test report to verify the notification mechanisms.
func NewTest() Test {
	return Test{
		basic: newBasic(),
	}
}

// Build creates a report for testing purpose. On error it will return an
// Error type encapsulated in a traceable error. To retrieve the desired error
// you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *report.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (tr Test) Build(f Format) (string, error) {
	var tmpl string

	switch f {
	case FormatHTML:
		tmpl = `
    <section class="report">
      <h1>Test report</h1>
      <div class="date">
        {{.CreatedAt.Format "2006-01-02 15:04:05"}}
      </div>
      <p>Testing the notification mechanisms.</p>
      {{if .Errors -}}
      <h2>Errors</h2>
      <ul>
        {{range $err := .Errors}}
        <li>{{$err}}</li>
        {{- end -}}
      </ul>
      {{- end}}
    </section>
  `

	case FormatPlain:
		fallthrough

	default:
		tmpl = `
[{{.CreatedAt.Format "2006-01-02 15:04:05"}}] Test report

  Testing the notification mechanisms.

  {{if .Errors -}}
  Errors
  ------
    {{range $err := .Errors}}
    * {{$err}}
    {{- end -}}
  {{- end}}
  `
	}

	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, tr); err != nil {
		return "", errors.WithStack(newError(ErrorCodeTemplate, err))
	}
	return buffer.String(), nil
}

// Add stores the report information to be retrieved later.
func Add(r Report) {
	reportsLock.Lock()
	defer reportsLock.Unlock()

	reports = append(reports, r)
}

// Clear removes all reports from the internal cache. Useful for testing
// environments.
func Clear() {
	reportsLock.Lock()
	defer reportsLock.Unlock()

	reports = []Report{}
}

// Build generates the report in the specify format. Every time this function is
// called the internal cache of reports is cleared. On error it will return an
// Error type encapsulated in a traceable error. To retrieve the desired error
// you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *report.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func Build(f Format) (string, error) {
	reportsLock.Lock()
	defer reportsLock.Unlock()
	defer func() {
		reports = nil
	}()

	var buffer string
	for _, r := range reports {
		tmp, err := r.Build(f)
		if err != nil {
			return "", errors.WithStack(err)
		}

		// using fmt.Sprintln to create a cross platform line break
		buffer += fmt.Sprintln(tmp)
	}

	if f == FormatHTML {
		buffer = formatHTMLPrefix + buffer + formatHTMLSuffix
	}

	return buffer, nil
}
