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

type Report interface {
	Build() (string, error)
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

	Backup cloud.Backup

	// Paths list of directories that were used in this backup.
	// TODO: Move this to cloud.Backup type?
	Paths []string

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

func (s SendBackup) Build() (string, error) {
	tmpl := `
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
	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, s); err != nil {
		return "", errors.WithStack(newReportError(ReportErrorCodeTemplate, err))
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

func (l ListBackups) Build() (string, error) {
	tmpl := `
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
	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, l); err != nil {
		return "", errors.WithStack(newReportError(ReportErrorCodeTemplate, err))
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

func (r RemoveOldBackups) Build() (string, error) {
	tmpl := `
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
	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, r); err != nil {
		return "", errors.WithStack(newReportError(ReportErrorCodeTemplate, err))
	}
	return buffer.String(), nil
}

type Test struct {
	basic
}

// NewTest initialize a new test report to verify the notification mechanisms.
func NewTest() Test {
	return Test{
		basic: newBasic(),
	}
}

func (tr Test) Build() (string, error) {
	tmpl := `
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
	t := template.Must(template.New("report").Parse(tmpl))

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, tr); err != nil {
		return "", errors.WithStack(newReportError(ReportErrorCodeTemplate, err))
	}
	return buffer.String(), nil
}

// Add stores the report information to be retrieved later.
func Add(r Report) {
	reportsLock.Lock()
	defer reportsLock.Unlock()

	reports = append(reports, r)
}

// Build generates the report in text format. Every time this function is called the
// internal cache of reports is cleared.
func Build() (string, error) {
	reportsLock.Lock()
	defer reportsLock.Unlock()
	defer func() {
		reports = nil
	}()

	var buffer string
	for _, r := range reports {
		tmp, err := r.Build()
		if err != nil {
			return "", errors.WithStack(err)
		}

		// using fmt.Sprintln to create a cross platform line break
		buffer += fmt.Sprintln(tmp)
	}

	return buffer, nil
}
