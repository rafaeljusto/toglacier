package archive

import (
	"regexp"
)

const (
	// ItemInfoStatusNew refers to an item that appeared for the first time in the
	// archive.
	ItemInfoStatusNew ItemInfoStatus = "new"

	// ItemInfoStatusModified refers to an item that was modified since the last
	// archive built.
	ItemInfoStatusModified ItemInfoStatus = "modified"

	// ItemInfoStatusUnmodified refers to an item that was not modified since the
	// last archive built.
	ItemInfoStatusUnmodified ItemInfoStatus = "unmodified"

	// ItemInfoStatusDeleted refers to an item that disappeared since the last
	// archive built.
	ItemInfoStatusDeleted ItemInfoStatus = "deleted"
)

// ItemInfoStatus describes the current archive's item state.
type ItemInfoStatus string

// Useful returns if the current status indicates that the archive item is
// useful or not.
func (i ItemInfoStatus) Useful() bool {
	return i == ItemInfoStatusNew || i == ItemInfoStatusModified
}

// ItemInfo stores all the necessary information to track the archive's item
// state.
type ItemInfo struct {
	ID       string
	Status   ItemInfoStatus
	Checksum string
}

// Info stores extra information from the archive's items for allowing
// incremental archives.
type Info map[string]ItemInfo

// Merge adds all extra information that doesn't exist uet into the current
// storage.
func (a Info) Merge(info Info) {
	for mergeFilename, mergeItemInfo := range info {
		if _, ok := a[mergeFilename]; !ok {
			a[mergeFilename] = mergeItemInfo
		}
	}
}

// MergeLast verifies all extra information that appeared in the last archive
// creation, but it doesn't appeared now. This is necessary to detect when items
// where deleted.
func (a Info) MergeLast(last Info) {
	for lastFilename, lastItemInfo := range last {
		if _, ok := a[lastFilename]; !ok && lastItemInfo.Status != ItemInfoStatusDeleted {
			a[lastFilename] = ItemInfo{
				ID:       lastItemInfo.ID,
				Status:   ItemInfoStatusDeleted,
				Checksum: lastItemInfo.Checksum,
			}
		}
	}
}

// Statistics count the number of paths on each archive status.
func (a Info) Statistics() map[ItemInfoStatus]int {
	statistic := make(map[ItemInfoStatus]int)
	for _, itemInfo := range a {
		statistic[itemInfo.Status]++
	}
	return statistic
}

// FilterByStatuses returns the archive information only containing the items
// that have the desired statuses.
func (a Info) FilterByStatuses(statuses ...ItemInfoStatus) Info {
	filtered := make(Info)
	for filename, itemInfo := range a {
		for _, status := range statuses {
			if itemInfo.Status == status {
				filtered[filename] = itemInfo
				break
			}
		}
	}
	return filtered
}

// Archive manages an archive joining all paths in a file, extracting and
// calculating Checksums.
type Archive interface {
	Build(lastArchiveInfo Info, ignoreFiles *regexp.Regexp, backupPaths ...string) (string, Info, error)
	Extract(filename string, filter []string) (Info, error)
	FileChecksum(filename string) (string, error)
}

// Envelop manages the security of an archive encrypting and decrypting the
// content.
type Envelop interface {
	Encrypt(filename, secret string) (string, error)
	Decrypt(encryptedFilename, secret string) (string, error)
}
