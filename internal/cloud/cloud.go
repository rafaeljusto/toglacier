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

	// Get retrieves a specific backup and stores it locally in a file. The
	// filename where the backup was saved is returned. The operation can be
	// cancelled anytime using the context.
	Get(ctx context.Context, id string) (filename string, err error)

	// Remove erase a specific backup from the cloud. The operation can be
	// cancelled anytime using the context.
	Remove(ctx context.Context, id string) error
}
