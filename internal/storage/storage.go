package storage

import (
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
)

// Backup stores the cloud location of the backup and some extra information
// about the files of the backup.
type Backup struct {
	Backup cloud.Backup
	Info   archive.Info
}

// Storage represents all commands to manage backups information locally. After
// the backup is uploaded we must keep track of them locally to speed up
// recovery and cloud cleanup (remove old ones).
type Storage interface {
	// Save a backup information.
	Save(Backup) error

	// List all backup informations in the storage.
	List() ([]Backup, error)

	// Remove a specific backup information from the storage.
	Remove(id string) error
}
