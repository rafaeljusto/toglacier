package storage

import (
	"encoding/json"
	"os"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
)

// BoltDBBucket defines the bucket in the BoltDB database where the data will be
// stored.
var BoltDBBucket = []byte("toglacier")

// BoltDBFileMode defines the file mode used for the BoltDB database file. By
// default only the owner has permission to access the file.
var BoltDBFileMode = os.FileMode(0600)

// BoltDB stores all necessary data to use the BoltDB database. BoltDB was
// chosen as it is a fast key/value storage that uses only one local file. More
// information can be found at https://github.com/boltdb/bolt
type BoltDB struct {
	logger   log.Logger
	Filename string
}

// NewBoltDB initializes a BoltDB storage.
func NewBoltDB(logger log.Logger, filename string) *BoltDB {
	return &BoltDB{
		logger:   logger,
		Filename: filename,
	}
}

// Save a backup information. On error it will return an Error type encapsulated
// in a traceable error. To retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *storage.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (b *BoltDB) Save(backup Backup) error {
	b.logger.Debugf("storage: saving backup “%s” in boltdb storage", backup.Backup.ID)

	db, err := bolt.Open(b.Filename, BoltDBFileMode, nil)
	if err != nil {
		return errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer db.Close()

	encoded, err := json.Marshal(backup)
	if err != nil {
		return errors.WithStack(newError(ErrorCodeEncodingBackup, err))
	}

	b.logger.Debugf("storage: saving backup json format: “%s”", string(encoded))

	err = db.Update(func(tx *bolt.Tx) error {
		var bucket *bolt.Bucket
		if bucket, err = tx.CreateBucketIfNotExists(BoltDBBucket); err != nil {
			return errors.WithStack(newError(ErrorAccessingBucket, err))
		}

		if err = bucket.Put([]byte(backup.Backup.ID), encoded); err != nil {
			return errors.WithStack(newError(ErrorCodeSave, err))
		}

		return nil
	})

	if err != nil {
		return errors.WithStack(newError(ErrorCodeUpdatingDatabase, err))
	}

	b.logger.Infof("storage: backup “%s” saved successfully in boltdb storage", backup.Backup.ID)
	return nil
}

// List all backup information in the storage. On error it will return an Error
// type encapsulated in a traceable error. To retrieve the desired error you can
// do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *storage.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (b BoltDB) List() (Backups, error) {
	b.logger.Debug("storage: listing backups from boltdb storage")

	db, err := bolt.Open(b.Filename, BoltDBFileMode, nil)
	if err != nil {
		return nil, errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer db.Close()

	var backups Backups

	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BoltDBBucket)
		if bucket == nil {
			// no backup stored yet
			return nil
		}

		err = bucket.ForEach(func(k, v []byte) error {
			var backup Backup
			if err = json.Unmarshal(v, &backup); err != nil {
				return errors.WithStack(newError(ErrorCodeDecodingBackup, err))
			}
			backups.Add(backup)
			return nil
		})

		if err != nil {
			return errors.WithStack(newError(ErrorCodeIterating, err))
		}

		return nil
	})

	if err != nil {
		return nil, errors.WithStack(newError(ErrorCodeListingDatabase, err))
	}

	b.logger.Infof("storage: backups listed successfully from boltdb storage")
	return backups, nil
}

// Remove a specific backup information from the storage. On error it will
// return an Error type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *storage.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (b BoltDB) Remove(id string) error {
	b.logger.Debugf("storage: removing backup “%s” from boltdb storage", id)

	db, err := bolt.Open(b.Filename, BoltDBFileMode, nil)
	if err != nil {
		return errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BoltDBBucket)
		if bucket == nil {
			return errors.WithStack(newError(ErrorCodeDatabaseNotFound, nil))
		}

		if err = bucket.Delete([]byte(id)); err != nil {
			return errors.WithStack(newError(ErrorCodeDelete, err))
		}

		return nil
	})

	if err != nil {
		return errors.WithStack(newError(ErrorCodeUpdatingDatabase, err))
	}

	b.logger.Infof("storage: backup “%s” removed successfully from boltdb storage", id)
	return nil
}
