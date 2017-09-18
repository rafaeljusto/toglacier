package cloud

import (
	"context"
)

// Cloud offers all necessary operations to manage backups in the cloud.
type Cloud interface {
	// Send uploads the file to the cloud and return the backup archive
	// information. The upload operation can be cancelled anytime using the
	// context.
	Send(ctx context.Context, filename string) (Backup, error)

	// List retrieves all the uploaded backups information in the cloud. The
	// operation can be cancelled anytime using the context.
	List(ctx context.Context) ([]Backup, error)

	// Get retrieves the backups with the given ids and stores them locally in
	// files. The ids and corresponding filenames where the backups were saved are
	// returned. The operation can be cancelled anytime using the context.
	Get(ctx context.Context, ids ...string) (filenames map[string]string, err error)

	// Remove erase a specific backup from the cloud. The operation can be
	// cancelled anytime using the context.
	Remove(ctx context.Context, id string) error

	// Close ends the cloud service session.
	Close() error
}
