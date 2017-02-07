package storage

import "github.com/rafaeljusto/toglacier/internal/cloud"

// Storage represents all commands to manage backups information locally. After
// the backup is uploaded we must keep track of them locally to speed up
// recovery and cloud cleanup (remove old ones).
type Storage interface {
	// Save a backup information.
	Save(cloud.Backup) error

	// List all backup informations in the storage.
	List() ([]cloud.Backup, error)

	// Remove a specific backup information from the storage.
	Remove(id string) error
}
