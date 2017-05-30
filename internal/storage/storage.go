package storage

import (
	"strings"

	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
)

// Backup stores the cloud location of the backup and some extra information
// about the files of the backup.
type Backup struct {
	Backup cloud.Backup // TODO: rename this attribute?
	Info   archive.Info
}

// Backups represents a sorted list of backups that are ordered by creation
// date. It has the necessary methods so you could use the sort package of the
// standard library.
type Backups []Backup

// Len returns the number of backups.
func (b Backups) Len() int { return len(b) }

// Less compares two positions of the slice and verifies the preference. They
// are ordered by the id, that should be unique.
func (b Backups) Less(i, j int) bool {
	return strings.Compare(b[i].Backup.ID, b[j].Backup.ID) >= 0
}

// Swap change the backups position inside the slice.
func (b Backups) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

// Storage represents all commands to manage backups information locally. After
// the backup is uploaded we must keep track of them locally to speed up
// recovery and cloud cleanup (remove old ones).
type Storage interface {
	// Save a backup information.
	Save(Backup) error

	// List all backup informations in the storage.
	List() (Backups, error)

	// Remove a specific backup information from the storage.
	Remove(id string) error
}
