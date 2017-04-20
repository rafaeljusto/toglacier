package cloud

import "time"

// Backup store all the necessary information of an already uploaded archive.
type Backup struct {
	// ID primary key to identify the archive in the cloud.
	ID string

	// Time that the archive was created in the cloud.
	CreatedAt time.Time

	// Checksum is a SHA256 of the archive content.
	Checksum string

	// VaultName is the identifier of the place in the cloud where the archive was
	// stored.
	VaultName string

	// Size backup archive size.
	Size int64
}
