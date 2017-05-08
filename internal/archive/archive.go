package archive

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

// ItemInfo stores all the necessary information to track the archive's item
// state.
type ItemInfo struct {
	ID     string
	Status ItemInfoStatus
	Hash   string // TODO: Rename to Checksum?
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
				ID:     lastItemInfo.ID,
				Status: ItemInfoStatusDeleted,
				Hash:   lastItemInfo.Hash,
			}
		}
	}
}

// Builder creates an archive joining all paths in a file.
type Builder interface {
	Build(lastArchiveInfo Info, backupPaths ...string) (string, Info, error)
	Extract(filename string, filter []string) (Info, error)
}

// Envelop manages the security of an archive encrypting and decrypting the
// content.
type Envelop interface {
	Encrypt(filename, secret string) (string, error)
	Decrypt(encryptedFilename, secret string) (string, error)
}
