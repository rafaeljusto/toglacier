package storage_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/log"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestBoltDB_Save(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		logger        log.Logger
		filename      string
		backup        cloud.Backup
		expectedError error
	}{
		{
			description: "it should save a backup correctly",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-")
				if err != nil {
					t.Fatalf("error creating a temporary file. details: %s", err)
				}
				defer f.Close()

				return f.Name()
			}(),
			backup: cloud.Backup{
				ID:        "123456",
				CreatedAt: now,
				Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
				VaultName: "test",
			},
		},
		{
			description: "it should fail to use a database file with no permission",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
				n := path.Join(os.TempDir(), "toglacier-test-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					f, err := os.OpenFile(n, os.O_CREATE, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary file. details: %s", err)
					}
					defer f.Close()
				}

				return n
			}(),
			expectedError: &storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-test-noperm"),
					Err:  errors.New("permission denied"),
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			boltDB := storage.NewBoltDB(scenario.logger, scenario.filename)
			err := boltDB.Save(scenario.backup)

			if !storage.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestBoltDB_List(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		logger        log.Logger
		filename      string
		expected      []cloud.Backup
		expectedError error
	}{
		{
			description: "it should list all backups information correctly",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
				backup := cloud.Backup{
					ID: "123456",
					CreatedAt: func() time.Time {
						c, err := time.Parse(time.RFC3339, now.Format(time.RFC3339))
						if err != nil {
							t.Fatalf("error parsing current time. details: %s", err)
						}
						return c
					}(),
					Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
					VaultName: "test",
				}

				encoded, err := json.Marshal(backup)
				if err != nil {
					t.Fatalf("error encoding backup. details: %s", err)
				}

				f, err := ioutil.TempFile("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating a temporary file. details: %s", err)
				}
				f.Close()

				boltDB, err := bolt.Open(f.Name(), storage.BoltDBFileMode, nil)
				if err != nil {
					t.Fatalf("error opening database. details: %s", err)
				}
				defer boltDB.Close()

				err = boltDB.Update(func(tx *bolt.Tx) error {
					bucket, err := tx.CreateBucketIfNotExists(storage.BoltDBBucket)
					if err != nil {
						t.Fatalf("error creating or opening bucket. details: %s", err)
					}

					if err := bucket.Put([]byte(backup.ID), encoded); err != nil {
						t.Fatalf("error putting data in bucket. details: %s", err)
					}

					return nil
				})

				if err != nil {
					t.Fatalf("error updating bucket. details: %s", err)
				}

				return f.Name()
			}(),
			expected: []cloud.Backup{
				{
					ID: "123456",
					CreatedAt: func() time.Time {
						c, err := time.Parse(time.RFC3339, now.Format(time.RFC3339))
						if err != nil {
							t.Fatalf("error parsing current time. details: %s", err)
						}
						return c
					}(),
					Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
					VaultName: "test",
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			boltDB := storage.NewBoltDB(scenario.logger, scenario.filename)
			backups, err := boltDB.List()

			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backups))
			}

			if !storage.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}
