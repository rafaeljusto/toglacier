package report

import (
	"sync"
	"time"

	"github.com/rafaeljusto/toglacier/internal/cloud"
)

var (
	reports     []Report
	reportsLock sync.Mutex
)

type Report interface {
	// TODO
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

// AddReport store the report information to be retrieved later.
func AddReport(r Report) {
	reportsLock.Lock()
	defer reportsLock.Unlock()

	reports = append(reports, r)
}

// GetReport return all reports stored. Every time this function is called the
// internal cache of reports is cleared.
func GetReport() []Report {
	reportsLock.Lock()
	defer reportsLock.Unlock()
	defer func() {
		reports = nil
	}()

	return reports
}
