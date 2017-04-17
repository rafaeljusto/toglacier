package storage

import (
	"encoding/json"
	"os"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/log"
)

var BoltDBBucket = []byte("toglacier")
var BoltDBFileMode = os.FileMode(0600)

type BoltDB struct {
	logger   log.Logger
	Filename string
}

func NewBoltDB(logger log.Logger, filename string) *BoltDB {
	return &BoltDB{
		logger:   logger,
		Filename: filename,
	}
}

func (b *BoltDB) Save(backup cloud.Backup) error {
	db, err := bolt.Open(b.Filename, BoltDBFileMode, nil)
	if err != nil {
		return errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer db.Close()

	encoded, err := json.Marshal(b)
	if err != nil {
		return errors.WithStack(newError(ErrorCodeEncodingBackup, err))
	}

	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(BoltDBBucket)
		if err != nil {
			return errors.WithStack(newError(ErrorAccessingBucket, err))
		}

		if err := bucket.Put([]byte(backup.ID), encoded); err != nil {
			return errors.WithStack(newError(ErrorCodeSave, err))
		}

		return nil
	})

	if err != nil {
		return errors.WithStack(newError(ErrorCodeUpdatingDatabase, err))
	}

	return nil
}

func (b BoltDB) List() ([]cloud.Backup, error) {
	db, err := bolt.Open(b.Filename, BoltDBFileMode, nil)
	if err != nil {
		return nil, errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer db.Close()

	var backups []cloud.Backup

	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BoltDBBucket)
		if bucket == nil {
			// no backup stored yet
			return nil
		}

		err := bucket.ForEach(func(k, v []byte) error {
			var backup cloud.Backup
			if err := json.Unmarshal(v, &backup); err != nil {
				return errors.WithStack(newError(ErrorCodeDecodingBackup, err))
			}
			backups = append(backups, backup)
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

	return backups, nil
}

func (b BoltDB) Remove(id string) error {
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

		if err := bucket.Delete([]byte(id)); err != nil {
			return errors.WithStack(newError(ErrorCodeDelete, err))
		}

		return nil
	})

	if err != nil {
		return errors.WithStack(newError(ErrorCodeUpdatingDatabase, err))
	}

	return nil
}
