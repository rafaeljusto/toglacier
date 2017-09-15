package cloud

import (
	"fmt"
	"strings"
	"time"
)

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

	// Location defines where the backup was stored.
	Location Location
}

const (
	// LocationAWS indicates that the backup was stored in Amazon AWS cloud.
	LocationAWS Location = "aws"

	// LocationGCS indicates that the backup was stored in Google Cloud Storage.
	LocationGCS Location = "gcs"
)

// Location contains the cloud that is current storing the backup data.
type Location string

// ParseLocation converts a text to a Location type.
func ParseLocation(value string) (Location, error) {
	value = strings.ToLower(value)
	value = strings.TrimSpace(value)

	switch value {
	case string(LocationAWS):
		return LocationAWS, nil
	case string(LocationGCS):
		return LocationGCS, nil
	}

	// not return a library error here because this is used by the library itself
	// to build backups from storage
	return Location(""), fmt.Errorf("unknown location “%s”", value)
}

// Defined returns true if the location has a valid value.
func (l Location) Defined() bool {
	return l == LocationAWS || l == LocationGCS
}
