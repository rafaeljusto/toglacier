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
		backup        storage.Backup
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
			backup: storage.Backup{
				Backup: cloud.Backup{
					ID:        "123456",
					CreatedAt: now,
					Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
					VaultName: "test",
					Size:      120,
				},
			},
		},
		{
			description: "it should fail when backup id is empty",
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
			backup: storage.Backup{
				Backup: cloud.Backup{
					ID:        "",
					CreatedAt: now,
					Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
					VaultName: "test",
				},
			},
			expectedError: &storage.Error{
				Code: storage.ErrorCodeUpdatingDatabase,
				Err: &storage.Error{
					Code: storage.ErrorCodeSave,
					Err:  bolt.ErrKeyRequired,
				},
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
		expected      storage.Backups
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
				backup1 := storage.Backup{
					Backup: cloud.Backup{
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
						Size:      120,
					},
				}

				encoded1, err := json.Marshal(backup1)
				if err != nil {
					t.Fatalf("error encoding backup 1. details: %s", err)
				}

				backup2 := storage.Backup{
					Backup: cloud.Backup{
						ID: "654321",
						CreatedAt: func() time.Time {
							var c time.Time
							if c, err = time.Parse(time.RFC3339, now.Add(time.Second).Format(time.RFC3339)); err != nil {
								t.Fatalf("error parsing current time. details: %s", err)
							}
							return c
						}(),
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
						Size:      120,
					},
				}

				encoded2, err := json.Marshal(backup2)
				if err != nil {
					t.Fatalf("error encoding backup 2. details: %s", err)
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
					var bucket *bolt.Bucket
					if bucket, err = tx.CreateBucketIfNotExists(storage.BoltDBBucket); err != nil {
						t.Fatalf("error creating or opening bucket. details: %s", err)
					}

					if err = bucket.Put([]byte(backup1.Backup.ID), encoded1); err != nil {
						t.Fatalf("error putting data in bucket. details: %s", err)
					}

					if err = bucket.Put([]byte(backup2.Backup.ID), encoded2); err != nil {
						t.Fatalf("error putting data in bucket. details: %s", err)
					}

					return nil
				})

				if err != nil {
					t.Fatalf("error updating bucket. details: %s", err)
				}

				return f.Name()
			}(),
			expected: storage.Backups{
				{
					Backup: cloud.Backup{
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
						Size:      120,
					},
				},
				{
					Backup: cloud.Backup{
						ID: "654321",
						CreatedAt: func() time.Time {
							c, err := time.Parse(time.RFC3339, now.Add(time.Second).Format(time.RFC3339))
							if err != nil {
								t.Fatalf("error parsing current time. details: %s", err)
							}
							return c
						}(),
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
						Size:      120,
					},
				},
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
		{
			description: "it should ignore when the bucket doesn't exist",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
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

				return f.Name()
			}(),
		},
		{
			description: "it should detect an invalid JSON",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
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
					var bucket *bolt.Bucket
					if bucket, err = tx.CreateBucketIfNotExists(storage.BoltDBBucket); err != nil {
						t.Fatalf("error creating or opening bucket. details: %s", err)
					}

					if err = bucket.Put([]byte("123456"), []byte("{invalid json")); err != nil {
						t.Fatalf("error putting data in bucket. details: %s", err)
					}

					return nil
				})

				if err != nil {
					t.Fatalf("error updating bucket. details: %s", err)
				}

				return f.Name()
			}(),
			expectedError: &storage.Error{
				Code: storage.ErrorCodeListingDatabase,
				Err: &storage.Error{
					Code: storage.ErrorCodeIterating,
					Err: &storage.Error{
						Code: storage.ErrorCodeDecodingBackup,
						Err:  errors.New(`invalid character 'i' looking for beginning of object key string`), // json.SyntaxError has private fields and cannot be used here
					},
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

func TestBoltDB_Remove(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		logger        log.Logger
		filename      string
		id            string
		expectedError error
	}{
		{
			description: "it should remove correctly from the database",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
				backup := storage.Backup{
					Backup: cloud.Backup{
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
						Size:      120,
					},
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
					var bucket *bolt.Bucket
					if bucket, err = tx.CreateBucketIfNotExists(storage.BoltDBBucket); err != nil {
						t.Fatalf("error creating or opening bucket. details: %s", err)
					}

					if err = bucket.Put([]byte(backup.Backup.ID), encoded); err != nil {
						t.Fatalf("error putting data in bucket. details: %s", err)
					}

					return nil
				})

				if err != nil {
					t.Fatalf("error updating bucket. details: %s", err)
				}

				return f.Name()
			}(),
			id: "123456",
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
		{
			description: "it should detect when the database bucket doesn't exist",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
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

				return f.Name()
			}(),
			id: "123456",
			expectedError: &storage.Error{
				Code: storage.ErrorCodeUpdatingDatabase,
				Err: &storage.Error{
					Code: storage.ErrorCodeDatabaseNotFound,
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			boltDB := storage.NewBoltDB(scenario.logger, scenario.filename)
			err := boltDB.Remove(scenario.id)

			if !storage.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}
