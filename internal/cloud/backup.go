package cloud

import "time"

type Backup struct {
	ID        string
	CreatedAt time.Time
	Checksum  string
	VaultName string
}
