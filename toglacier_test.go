package toglacier_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/smtp"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aryann/difflib"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/log"
	"github.com/rafaeljusto/toglacier/internal/report"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestToGlacier_Backup(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		backupPaths   []string
		backupSecret  string
		archive       archive.Archive
		envelop       archive.Envelop
		cloud         cloud.Cloud
		storage       storage.Storage
		expectedError error
	}{
		{
			description: "it should backup correctly an archive",
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				return []string{d}
			}(),
			archive: mockArchive{
				mockBuild: func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
					if len(backupPaths) == 0 {
						t.Fatalf("no backup path informed")
					}

					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), archive.Info{
						path.Join(backupPaths[0], "file1"): archive.ItemInfo{
							ID:       "",
							Status:   archive.ItemInfoStatusModified,
							Checksum: "11e87f16676135f6b4bc8da00883e4e02e51595d07841dbc8c16c5d2047a304d",
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockSend: func(filename string) (cloud.Backup, error) {
					return cloud.Backup{
						ID:        "123456",
						CreatedAt: now,
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123455",
								CreatedAt: now.Add(-time.Hour),
								Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
								VaultName: "test",
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "123455",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
								},
							},
						},
					}, nil
				},
			},
		},
		{
			description: "it should detect when there's a problem listing the current backups",
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				return []string{d}
			}(),
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return nil, errors.New("problem loading backups from storage")
				},
			},
			expectedError: errors.New("problem loading backups from storage"),
		},
		{
			description: "it should backup correctly an archive with encryption",
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				return []string{d}
			}(),
			backupSecret: "12345678901234567890123456789012",
			archive: mockArchive{
				mockBuild: func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil, nil
				},
			},
			envelop: mockEnvelop{
				mockEncrypt: func(filename, secret string) (string, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil
				},
			},
			cloud: mockCloud{
				mockSend: func(filename string) (cloud.Backup, error) {
					return cloud.Backup{
						ID:        "123456",
						CreatedAt: now,
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return nil, nil
				},
			},
		},
		{
			description: "it should detect an error while building the package",
			backupPaths: func() []string {
				return []string{"idontexist12345"}
			}(),
			archive: mockArchive{
				mockBuild: func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
					return "", nil, errors.New("path doesn't exist")
				},
			},
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return nil, nil
				},
			},
			expectedError: errors.New("path doesn't exist"),
		},
		{
			description: "it should detect when there is nothing in the tarball",
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}
				return []string{d}
			}(),
			archive: mockArchive{
				mockBuild: func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
					if len(backupPaths) == 0 {
						t.Fatalf("no backup path informed")
					}

					return "", nil, nil
				},
			},
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123455",
								CreatedAt: now.Add(-time.Hour),
								Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
								VaultName: "test",
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "123455",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
								},
							},
						},
					}, nil
				},
			},
		},
		{
			description: "it should detect an error while encrypting the package",
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				return []string{d}
			}(),
			backupSecret: "123456",
			archive: mockArchive{
				mockBuild: func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil, nil
				},
			},
			envelop: mockEnvelop{
				mockEncrypt: func(filename, secret string) (string, error) {
					return "", errors.New("failed to encrypt the archive")
				},
			},
			cloud: mockCloud{
				mockSend: func(filename string) (cloud.Backup, error) {
					return cloud.Backup{
						ID:        "123456",
						CreatedAt: now,
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return nil, nil
				},
			},
			expectedError: errors.New("failed to encrypt the archive"),
		},
		{
			description: "it should detect an error while sending the backup",
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				return []string{d}
			}(),
			archive: mockArchive{
				mockBuild: func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil, nil
				},
			},
			cloud: mockCloud{
				mockSend: func(filename string) (cloud.Backup, error) {
					return cloud.Backup{}, errors.New("error sending backup")
				},
			},
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return nil, nil
				},
			},
			expectedError: errors.New("error sending backup"),
		},
		{
			description: "it should detect an error while saving the backup information",
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				return []string{d}
			}(),
			archive: mockArchive{
				mockBuild: func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil, nil
				},
			},
			cloud: mockCloud{
				mockSend: func(filename string) (cloud.Backup, error) {
					return cloud.Backup{
						ID:        "123456",
						CreatedAt: now,
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return errors.New("error saving the backup information")
				},
				mockList: func() (storage.Backups, error) {
					return nil, nil
				},
			},
			expectedError: errors.New("error saving the backup information"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			toGlacier := toglacier.ToGlacier{
				Context: context.Background(),
				Archive: scenario.archive,
				Envelop: scenario.envelop,
				Cloud:   scenario.cloud,
				Storage: scenario.storage,
			}

			err := toGlacier.Backup(scenario.backupPaths, scenario.backupSecret)
			if !archive.ErrorEqual(scenario.expectedError, err) && !archive.PathErrorEqual(scenario.expectedError, err) && !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestToGlacier_ListBackups(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		remote        bool
		cloud         cloud.Cloud
		storage       storage.Storage
		logger        log.Logger
		expected      storage.Backups
		expectedError error
	}{
		{
			description: "it should list the remote backups correctly",
			remote:      true,
			cloud: mockCloud{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "123456" {
						return fmt.Errorf("adding unexpected id %s", b.Backup.ID)
					}

					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123454",
								CreatedAt: now.Add(-24 * time.Hour),
								Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123455",
								CreatedAt: now.Add(-30 * time.Hour),
								Checksum:  "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123456",
								CreatedAt: now.Add(-time.Hour),
								Checksum:  "75fcc5623af832086719316b41dcf744893514d8a5fefb376c6426d7911f215f",
								VaultName: "test",
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "123454",
									Status:   archive.ItemInfoStatusModified,
									Checksum: "915bd6a5873681a273f405c62993b6a96237eab9150fc525c9d57af0becb7ec1",
								},
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123457",
								CreatedAt: now.Add(-23 * time.Hour),
								Checksum:  "e1f6e5d1d7c964e46503bcf1812910c005634236ea087d9cadb1abdef3ae9a61",
								VaultName: "test",
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					if id != "123454" && id != "123455" && id != "123456" {
						return fmt.Errorf("removing unexpected id %s", id)
					}

					return nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expected: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "123456",
						CreatedAt: now,
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
					},
					Info: archive.Info{
						"file1": archive.ItemInfo{
							ID:       "123454",
							Status:   archive.ItemInfoStatusModified,
							Checksum: "915bd6a5873681a273f405c62993b6a96237eab9150fc525c9d57af0becb7ec1",
						},
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "123457",
						CreatedAt: now.Add(-23 * time.Hour),
						Checksum:  "e1f6e5d1d7c964e46503bcf1812910c005634236ea087d9cadb1abdef3ae9a61",
						VaultName: "test",
					},
				},
			},
		},
		{
			description: "it should list the local backups correctly",
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123456",
								CreatedAt: now,
								Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
								VaultName: "test",
							},
						},
					}, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expected: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "123456",
						CreatedAt: now,
						Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
						VaultName: "test",
					},
				},
			},
		},
		{
			description: "it should detect an error while listing the remote backups",
			remote:      true,
			cloud: mockCloud{
				mockList: func() ([]cloud.Backup, error) {
					return nil, errors.New("error listing backups")
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error listing backups"),
		},
		{
			description: "it should detect an error while listing the local backups",
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return nil, errors.New("error listing backups")
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error listing backups"),
		},
		{
			description: "it should detect an error while retrieving local backups for synch",
			remote:      true,
			cloud: mockCloud{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "123456" {
						return fmt.Errorf("adding unexpected id %s", b.Backup.ID)
					}

					return nil
				},
				mockList: func() (storage.Backups, error) {
					return nil, errors.New("error retrieving backups")
				},
				mockRemove: func(id string) error {
					if id != "123454" && id != "123455" {
						return fmt.Errorf("removing unexpected id %s", id)
					}

					return nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error retrieving backups"),
		},
		{
			description: "it should detect an error while removing local backups due to synch",
			remote:      true,
			cloud: mockCloud{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "123456" {
						return fmt.Errorf("adding unexpected id %s", b.Backup.ID)
					}

					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123454",
								CreatedAt: now.Add(-30 * time.Hour),
								Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123455",
								CreatedAt: now.Add(-40 * time.Hour),
								Checksum:  "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
								VaultName: "test",
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					return errors.New("error removing backup")
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error removing backup"),
		},
		{
			description: "it should detect an error while removing local recent backups due to synch",
			remote:      true,
			cloud: mockCloud{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "123456" {
						return fmt.Errorf("adding unexpected id %s", b.Backup.ID)
					}

					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123456",
								CreatedAt: now.Add(-time.Hour),
								Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
								VaultName: "test",
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					return errors.New("error removing backup")
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error removing backup"),
		},
		{
			description: "it should detect an error while adding new backups due to synch",
			remote:      true,
			cloud: mockCloud{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
					}, nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return errors.New("error adding backup")
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123454",
								CreatedAt: now.Add(-time.Second),
								Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123455",
								CreatedAt: now.Add(-time.Minute),
								Checksum:  "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
								VaultName: "test",
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					if id != "123454" && id != "123455" {
						return fmt.Errorf("removing unexpected id %s", id)
					}

					return nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error adding backup"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			toGlacier := toglacier.ToGlacier{
				Context: context.Background(),
				Cloud:   scenario.cloud,
				Storage: scenario.storage,
				Logger:  scenario.logger,
			}

			backups, err := toGlacier.ListBackups(scenario.remote)

			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backups))
			}

			if !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestToGlacier_RetrieveBackup(t *testing.T) {
	scenarios := []struct {
		description    string
		id             string
		backupSecret   string
		skipUnmodified bool
		storage        storage.Storage
		envelop        archive.Envelop
		cloud          cloud.Cloud
		archive        archive.Archive
		logger         log.Logger
		expectedError  error
	}{
		{
			description: "it should retrieve a backup correctly",
			id:          "AWSID123",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" && b.Backup.ID != "AWSID122" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID122",
								CreatedAt: time.Date(2015, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "8d9ccbb4e474dbd211a7b1f115c7bddaa950842e51a60418c4e943dee29e9113",
								VaultName: "vault",
								Size:      41,
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
								"file2": archive.ItemInfo{
									ID:       "AWSID122",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
								"file3": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "429713c8e82ae8d02bff0cd368581903ac6d368cfdacc5bb5ec6fc14d13f3fd0",
								},
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) != 2 {
						return nil, fmt.Errorf("unexpected number of ids: %v", ids)
					}

					return map[string]string{
						"AWSID123": "toglacier-archive-1.tar.gz",
						"AWSID122": "toglacier-archive-2.tar.gz",
					}, nil
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					sort.Strings(filter)

					switch filename {
					case "toglacier-archive-1.tar.gz":
						if len(filter) != 2 || filter[0] != "file1" || filter[1] != "file3" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					case "toglacier-archive-2.tar.gz":
						if len(filter) != 1 || filter[0] != "file2" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					}
					return nil, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
		},
		{
			description:  "it should retrieve an encrypted backup correctly",
			id:           "AWSID123",
			backupSecret: "1234567890123456",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
						},
					}, nil
				},
			},
			envelop: mockEnvelop{
				mockDecrypt: func(encryptedFilename, secret string) (string, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) == 0 {
						return nil, nil
					}

					n := path.Join(os.TempDir(), "toglacier-test-getenc")
					if _, err := os.Stat(n); os.IsNotExist(err) {
						f, err := os.Create(n)
						if err != nil {
							t.Fatalf("error creating a temporary file. details: %s", err)
						}
						defer f.Close()

						content, err := hex.DecodeString("656e637279707465643a8fbd41664a1d72b4ea1fcecd618a6ed5c05c95bf65bfda2d4d176e8feff96f710000000000000000000000000000000091d8e827b5136dfac6bb3dbc51f15c17d34947880f91e62799910ea05053969abc28033550b3781111")
						if err != nil {
							t.Fatalf("error decoding encrypted archive. details: %s", err)
						}

						f.Write(content)
					}

					return map[string]string{ids[0]: n}, nil
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					return nil, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
		},
		{
			description: "it should retrieve a backup correctly with no archive information and all other backup parts",
			id:          "AWSID123",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" && b.Backup.ID != "AWSID122" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID122",
								CreatedAt: time.Date(2015, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "325152353325adc8854e185ab59daf44c51e78404e1512eea9dca116f3a8c16d",
								VaultName: "vault",
								Size:      38,
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) == 0 {
						return nil, nil
					}

					switch ids[0] {
					case "AWSID123":
						return map[string]string{
							"AWSID123": "toglacier-archive-1.tar.gz",
						}, nil
					case "AWSID122":
						return map[string]string{
							"AWSID122": "toglacier-archive-2.tar.gz",
						}, nil
					}

					return nil, fmt.Errorf("unexpected id “%s”", ids[0])
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					switch filename {
					case "toglacier-archive-1.tar.gz":
						if len(filter) != 0 {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}

						return archive.Info{
							"file1": archive.ItemInfo{
								Status:   archive.ItemInfoStatusNew,
								ID:       "AWSID123",
								Checksum: "a5b2df3d72bd28d2382b0b4cca4c25fa260e018b58a915f1e5af14485a746ca8",
							},
							"file2": archive.ItemInfo{
								Status:   archive.ItemInfoStatusModified,
								ID:       "AWSID122",
								Checksum: "a8c23a9b1441de7f048471994f9500664acb0f6551e418e5b9da5af559606a63",
							},
						}, nil

					case "toglacier-archive-2.tar.gz":
						if len(filter) != 1 || filter[0] != "file2" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					}
					return nil, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
		},
		{
			description:    "it should retrieve a backup correctly skipping unmodified files in disk",
			id:             "AWSID123",
			skipUnmodified: true,
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
								"file2": archive.ItemInfo{
									ID:       "AWSID122",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "46813af30d24fb7ad0a019b0da4fcde88368133fcfe39c5a8b25a328e6be4ab2",
								},
								"file3": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "429713c8e82ae8d02bff0cd368581903ac6d368cfdacc5bb5ec6fc14d13f3fd0",
								},
								"file4": archive.ItemInfo{
									ID:       "AWSID124",
									Status:   archive.ItemInfoStatusUnmodified,
									Checksum: "79edf074b55cdb3088721e88814523124c7da05001175e14b0dcf78336730fcd",
								},
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) != 1 {
						return nil, fmt.Errorf("unexpected number of ids: %v", ids)
					}

					return map[string]string{
						"AWSID123": "toglacier-archive-1.tar.gz",
					}, nil
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					sort.Strings(filter)

					switch filename {
					case "toglacier-archive-1.tar.gz":
						if len(filter) != 2 || filter[0] != "file1" || filter[1] != "file3" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					case "toglacier-archive-2.tar.gz":
						if len(filter) != 1 || filter[0] != "file2" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					}
					return nil, nil
				},
				mockFileChecksum: func(filename string) (string, error) {
					switch filename {
					case "file1":
						return "a9300479a7d2c663b4806af1bce4483f93175cae287979ee0364d057445482c8", nil
					case "file2":
						return "46813af30d24fb7ad0a019b0da4fcde88368133fcfe39c5a8b25a328e6be4ab2", nil
					case "file3":
						return "64bd312e9c81172627d898d7ad146d2e9ea47f47dd67ea79477ab224ab8fb01b", nil
					case "file4":
						return "57ab560c94249dd6f3e5ee6397364a86aa38b1e893c23b1198e8cad8f2a063c5", nil
					}

					return "", fmt.Errorf("unexpected filename “%s”", filename)
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
		},
		{
			description:    "it should detect when there is a problem calculating the file checksum",
			id:             "AWSID123",
			skipUnmodified: true,
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
								"file2": archive.ItemInfo{
									ID:       "AWSID122",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "46813af30d24fb7ad0a019b0da4fcde88368133fcfe39c5a8b25a328e6be4ab2",
								},
								"file3": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "429713c8e82ae8d02bff0cd368581903ac6d368cfdacc5bb5ec6fc14d13f3fd0",
								},
								"file4": archive.ItemInfo{
									ID:       "AWSID124",
									Status:   archive.ItemInfoStatusUnmodified,
									Checksum: "79edf074b55cdb3088721e88814523124c7da05001175e14b0dcf78336730fcd",
								},
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) != 1 {
						return nil, fmt.Errorf("unexpected number of ids: %v", ids)
					}

					return map[string]string{
						"AWSID123": "toglacier-archive-1.tar.gz",
					}, nil
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					sort.Strings(filter)

					switch filename {
					case "toglacier-archive-1.tar.gz":
						if len(filter) != 2 || filter[0] != "file1" || filter[1] != "file3" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					case "toglacier-archive-2.tar.gz":
						if len(filter) != 1 || filter[0] != "file2" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					}
					return nil, nil
				},
				mockFileChecksum: func(filename string) (string, error) {
					return "", errors.New("checksum failed")
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("checksum failed"),
		},
		{
			description: "it should detect an error while retrieving a backup part",
			id:          "AWSID123",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) == 0 {
						return nil, nil
					}

					switch ids[0] {
					case "AWSID123":
						return map[string]string{
							"AWSID123": "toglacier-archive-1.tar.gz",
						}, nil
					case "AWSID122":
						return nil, errors.New("failed to download backup")
					}

					return nil, fmt.Errorf("unexpected id “%s”", ids[0])
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					switch filename {
					case "toglacier-archive-1.tar.gz":
						if len(filter) != 0 {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}

						return archive.Info{
							"file1": archive.ItemInfo{
								Status:   archive.ItemInfoStatusNew,
								ID:       "AWSID123",
								Checksum: "a5b2df3d72bd28d2382b0b4cca4c25fa260e018b58a915f1e5af14485a746ca8",
							},
							"file2": archive.ItemInfo{
								Status:   archive.ItemInfoStatusModified,
								ID:       "AWSID122",
								Checksum: "a8c23a9b1441de7f048471994f9500664acb0f6551e418e5b9da5af559606a63",
							},
						}, nil
					}
					return nil, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("failed to download backup"),
		},
		{
			description: "it should detect an error listing backups from local storage",
			id:          "AWSID123",
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return nil, errors.New("error listing the backups")
				},
			},
			expectedError: errors.New("error listing the backups"),
		},
		{
			description: "it should detect when there's an error retrieving a backup",
			id:          "AWSID123",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					return nil, errors.New("error retrieving the backup")
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error retrieving the backup"),
		},
		{
			description:  "it should detect an error decrypting the backup",
			id:           "AWSID123",
			backupSecret: "123456",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
						},
					}, nil
				},
			},
			envelop: mockEnvelop{
				mockDecrypt: func(encryptedFilename, secret string) (string, error) {
					return "", errors.New("invalid encrypted content")
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) == 0 {
						return nil, errors.New("no ids given")
					}

					n := path.Join(os.TempDir(), "toglacier-test-getenc")
					if _, err := os.Stat(n); os.IsNotExist(err) {
						f, err := os.Create(n)
						if err != nil {
							t.Fatalf("error creating a temporary file. details: %s", err)
						}
						defer f.Close()

						content, err := hex.DecodeString("656e637279707465643a8fbd41664a1d72b4ea1fcecd618a6ed5c05c95bf65bfda2d4d176e8feff96f710000000000000000000000000000000091d8e827b5136dfac6bb3dbc51f15c17d34947880f91e62799910ea05053969abc28033550b3781111")
						if err != nil {
							t.Fatalf("error decoding encrypted archive. details: %s", err)
						}

						f.Write(content)
					}

					return map[string]string{ids[0]: n}, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("invalid encrypted content"),
		},
		{
			description: "it should detect an error while extracting the backup",
			id:          "AWSID123",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "AWSID123" {
						return fmt.Errorf("unexpected id %s", b.Backup.ID)
					}
					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID122",
								CreatedAt: time.Date(2015, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "350c8ae1300b38a6cc74793e28712b5473c5f663bf8085b5c9bb0f191ed68f6d",
								VaultName: "vault",
								Size:      89,
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
								"file2": archive.ItemInfo{
									ID:       "AWSID122",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					return map[string]string{
						"AWSID123": "toglacier-archive-1.tar.gz",
						"AWSID122": "toglacier-archive-2.tar.gz",
					}, nil
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					switch filename {
					case "toglacier-archive-2.tar.gz":
						return nil, errors.New("error extracting backup")
					}
					return nil, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("error extracting backup"),
		},
		{
			description: "it should detect an error while saving a backup locally",
			id:          "AWSID123",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return errors.New("something went wrong")
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID122",
								CreatedAt: time.Date(2015, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "325152353325adc8854e185ab59daf44c51e78404e1512eea9dca116f3a8c16d",
								VaultName: "vault",
								Size:      38,
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) == 0 {
						return nil, nil
					}

					switch ids[0] {
					case "AWSID123":
						return map[string]string{
							"AWSID123": "toglacier-archive-1.tar.gz",
						}, nil
					case "AWSID122":
						return map[string]string{
							"AWSID122": "toglacier-archive-2.tar.gz",
						}, nil
					}

					return nil, fmt.Errorf("unexpected id “%s”", ids[0])
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					switch filename {
					case "toglacier-archive-1.tar.gz":
						if len(filter) != 0 {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}

						return archive.Info{
							"file1": archive.ItemInfo{
								Status:   archive.ItemInfoStatusNew,
								ID:       "AWSID123",
								Checksum: "a5b2df3d72bd28d2382b0b4cca4c25fa260e018b58a915f1e5af14485a746ca8",
							},
							"file2": archive.ItemInfo{
								Status:   archive.ItemInfoStatusModified,
								ID:       "AWSID122",
								Checksum: "a8c23a9b1441de7f048471994f9500664acb0f6551e418e5b9da5af559606a63",
							},
						}, nil

					case "toglacier-archive-2.tar.gz":
						if len(filter) != 1 || filter[0] != "file2" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					}
					return nil, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("something went wrong"),
		},
		{
			description: "it should detect an error while saving a backup part locally",
			id:          "AWSID123",
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return errors.New("something went wrong")
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "AWSID122",
								CreatedAt: time.Date(2015, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "8d9ccbb4e474dbd211a7b1f115c7bddaa950842e51a60418c4e943dee29e9113",
								VaultName: "vault",
								Size:      41,
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "AWSID123",
								CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
								VaultName: "vault",
								Size:      41,
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
								"file2": archive.ItemInfo{
									ID:       "AWSID122",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
								},
								"file3": archive.ItemInfo{
									ID:       "AWSID123",
									Status:   archive.ItemInfoStatusNew,
									Checksum: "429713c8e82ae8d02bff0cd368581903ac6d368cfdacc5bb5ec6fc14d13f3fd0",
								},
							},
						},
					}, nil
				},
			},
			cloud: mockCloud{
				mockGet: func(ids ...string) (filenames map[string]string, err error) {
					if len(ids) != 2 {
						return nil, fmt.Errorf("unexpected number of ids: %v", ids)
					}

					return map[string]string{
						"AWSID123": "toglacier-archive-1.tar.gz",
						"AWSID122": "toglacier-archive-2.tar.gz",
					}, nil
				},
			},
			archive: mockArchive{
				mockExtract: func(filename string, filter []string) (archive.Info, error) {
					sort.Strings(filter)

					switch filename {
					case "toglacier-archive-1.tar.gz":
						if len(filter) != 2 || filter[0] != "file1" || filter[1] != "file3" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					case "toglacier-archive-2.tar.gz":
						if len(filter) != 1 || filter[0] != "file2" {
							return nil, fmt.Errorf("unexpected filter “%v”", filter)
						}
					}
					return nil, nil
				},
			},
			logger: mockLogger{
				mockDebug:    func(args ...interface{}) {},
				mockDebugf:   func(format string, args ...interface{}) {},
				mockInfo:     func(args ...interface{}) {},
				mockInfof:    func(format string, args ...interface{}) {},
				mockWarning:  func(args ...interface{}) {},
				mockWarningf: func(format string, args ...interface{}) {},
			},
			expectedError: errors.New("something went wrong"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			toGlacier := toglacier.ToGlacier{
				Context: context.Background(),
				Storage: scenario.storage,
				Envelop: scenario.envelop,
				Cloud:   scenario.cloud,
				Archive: scenario.archive,
				Logger:  scenario.logger,
			}

			err := toGlacier.RetrieveBackup(scenario.id, scenario.backupSecret, scenario.skipUnmodified)

			if !archive.ErrorEqual(scenario.expectedError, err) && !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestToGlacier_RemoveBackup(t *testing.T) {
	scenarios := []struct {
		description   string
		ids           []string
		cloud         cloud.Cloud
		storage       storage.Storage
		expectedError error
	}{
		{
			description: "it should remove a backup correctly",
			ids:         []string{"123456"},
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					if b.Backup.ID != "123457" {
						return fmt.Errorf("saving unexpected backup id “%s”", b.Backup.ID)
					}

					if len(b.Info) > 0 {
						return fmt.Errorf("unexpected number (%d) of items info", len(b.Info))
					}

					return nil
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID: "123457",
							},
							Info: archive.Info{
								"filename1": archive.ItemInfo{
									ID: "123456",
								},
							},
						},
						{
							Backup: cloud.Backup{
								ID: "123455",
							},
							Info: archive.Info{
								"filename2": archive.ItemInfo{
									ID: "123454",
								},
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					if id != "123456" {
						return fmt.Errorf("unexpected id “%s”", id)
					}
					return nil
				},
			},
		},
		{
			description: "it should detect an error while removing the remote backup",
			ids:         []string{"123456"},
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return errors.New("error removing backup")
				},
			},
			storage: mockStorage{
				mockRemove: func(id string) error {
					return nil
				},
			},
			expectedError: errors.New("error removing backup"),
		},
		{
			description: "it should detect an error while removing the local backup",
			ids:         []string{"123456"},
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return nil
				},
			},
			storage: mockStorage{
				mockRemove: func(id string) error {
					return errors.New("error removing backup")
				},
			},
			expectedError: errors.New("error removing backup"),
		},
		{
			description: "it should detect an error listing the backups",
			ids:         []string{"123456"},
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return nil
				},
			},
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return nil, errors.New("failed to list backups")
				},
				mockRemove: func(id string) error {
					return nil
				},
			},
			expectedError: errors.New("failed to list backups"),
		},
		{
			description: "it should detect an error saving the backup",
			ids:         []string{"123456"},
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return nil
				},
			},
			storage: mockStorage{
				mockSave: func(b storage.Backup) error {
					return errors.New("could not save the backup")
				},
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID: "123457",
							},
							Info: archive.Info{
								"filename1": archive.ItemInfo{
									ID: "123456",
								},
							},
						},
						{
							Backup: cloud.Backup{
								ID: "123455",
							},
							Info: archive.Info{
								"filename2": archive.ItemInfo{
									ID: "123454",
								},
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					return nil
				},
			},
			expectedError: errors.New("could not save the backup"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			toGlacier := toglacier.ToGlacier{
				Context: context.Background(),
				Cloud:   scenario.cloud,
				Storage: scenario.storage,
			}

			if err := toGlacier.RemoveBackup(scenario.ids...); !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestToGlacier_RemoveOldBackups(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		keepBackups   int
		cloud         cloud.Cloud
		storage       storage.Storage
		expectedError error
	}{
		{
			description: "it should remove all old backups correctly",
			keepBackups: 2,
			cloud: mockCloud{
				mockRemove: func(id string) error {
					if id != "123456" {
						return fmt.Errorf("unexpected id %s", id)
					}
					return nil
				},
			},
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123456",
								CreatedAt: now,
								Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123457",
								CreatedAt: now.Add(time.Second),
								Checksum:  "0484ed70359cd1a4337d16a4143a3d247e0a3ecbce01482c318d709ed5161016",
								VaultName: "test",
							},
							Info: archive.Info{
								"file1": archive.ItemInfo{
									ID:       "123459",
									Status:   archive.ItemInfoStatusUnmodified,
									Checksum: "4c6733f2d51c5cde947835279ce9f031bcacaa2265988ef1353078810695fb20",
								},
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123458",
								CreatedAt: now.Add(time.Minute),
								Checksum:  "5f9c426fb1e150c1c09dda260bb962c7602b595df7586a1f3899735b839b138f",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123459",
								CreatedAt: now.Add(-time.Hour),
								Checksum:  "9a16f6eaebe1a7a3c9e456c5a37063d712de11d839040e5963cf864feb16e114",
								VaultName: "test",
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					if id != "123456" {
						return fmt.Errorf("removing unexpected id %s", id)
					}
					return nil
				},
			},
		},
		{
			description: "it should detect when there's an error listing the local backups",
			keepBackups: 2,
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return nil, errors.New("local storage corrupted")
				},
			},
			expectedError: errors.New("local storage corrupted"),
		},
		{
			description: "it should detect when there is an error removing an old backup from the cloud",
			keepBackups: 2,
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return errors.New("backup not found")
				},
			},
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123456",
								CreatedAt: now,
								Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123457",
								CreatedAt: now.Add(time.Second),
								Checksum:  "0484ed70359cd1a4337d16a4143a3d247e0a3ecbce01482c318d709ed5161016",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123458",
								CreatedAt: now.Add(time.Minute),
								Checksum:  "5f9c426fb1e150c1c09dda260bb962c7602b595df7586a1f3899735b839b138f",
								VaultName: "test",
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					if id != "123456" {
						return fmt.Errorf("removing unexpected id %s", id)
					}
					return nil
				},
			},
			expectedError: errors.New("backup not found"),
		},
		{
			description: "it should detect when there is an error removing an old backup from the local storage",
			keepBackups: 2,
			cloud: mockCloud{
				mockRemove: func(id string) error {
					if id != "123456" {
						return fmt.Errorf("unexpected id %s", id)
					}
					return nil
				},
			},
			storage: mockStorage{
				mockList: func() (storage.Backups, error) {
					return storage.Backups{
						{
							Backup: cloud.Backup{
								ID:        "123456",
								CreatedAt: now,
								Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123457",
								CreatedAt: now.Add(time.Second),
								Checksum:  "0484ed70359cd1a4337d16a4143a3d247e0a3ecbce01482c318d709ed5161016",
								VaultName: "test",
							},
						},
						{
							Backup: cloud.Backup{
								ID:        "123458",
								CreatedAt: now.Add(time.Minute),
								Checksum:  "5f9c426fb1e150c1c09dda260bb962c7602b595df7586a1f3899735b839b138f",
								VaultName: "test",
							},
						},
					}, nil
				},
				mockRemove: func(id string) error {
					return errors.New("backup not found")
				},
			},
			expectedError: errors.New("backup not found"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			toGlacier := toglacier.ToGlacier{
				Context: context.Background(),
				Cloud:   scenario.cloud,
				Storage: scenario.storage,
			}

			if err := toGlacier.RemoveOldBackups(scenario.keepBackups); !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestToGlacier_SendReport(t *testing.T) {
	date := time.Date(2017, 3, 10, 14, 10, 46, 0, time.UTC)

	scenarios := []struct {
		description   string
		reports       []report.Report
		emailSender   toglacier.EmailSender
		emailServer   string
		emailPort     int
		emailUsername string
		emailPassword string
		emailFrom     string
		emailTo       []string
		format        report.Format
		expectedError error
	}{
		{
			description: "it should send an e-mail correctly",
			reports: []report.Report{
				func() report.Report {
					r := report.NewTest()
					r.CreatedAt = date
					r.Errors = append(r.Errors, errors.New("timeout connecting to aws"))
					return r
				}(),
			},
			emailSender: toglacier.EmailSenderFunc(func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
				if addr != "127.0.0.1:587" {
					return fmt.Errorf("unexpected “address” %s", addr)
				}

				if from != "test@example.com" {
					return fmt.Errorf("unexpected “from” %s", from)
				}

				if !reflect.DeepEqual(to, []string{"user@example.com"}) {
					return fmt.Errorf("unexpected “to” %v", to)
				}

				expectedMsg := `From: test@example.com
To: user@example.com
Subject: toglacier report
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8


[2017-03-10 14:10:46] Test report

  Testing the notification mechanisms.

  Errors
  ------

    * timeout connecting to aws

`

				msgLines := strings.Split(string(msg), "\n")
				for i := range msgLines {
					msgLines[i] = strings.TrimSpace(msgLines[i])
				}

				expectedLines := strings.Split(expectedMsg, "\n")
				for i := range expectedLines {
					expectedLines[i] = strings.TrimSpace(expectedLines[i])
				}

				if !reflect.DeepEqual(expectedLines, msgLines) {
					return fmt.Errorf("unexpected message\n%v", Diff(expectedLines, msgLines))
				}

				return nil
			}),
			emailServer:   "127.0.0.1",
			emailPort:     587,
			emailUsername: "user",
			emailPassword: "abc123",
			emailFrom:     "test@example.com",
			emailTo: []string{
				"user@example.com",
			},
			format: report.FormatPlain,
		},
		{
			description: "it should fail to build the reports",
			reports: []report.Report{
				mockReport{
					mockBuild: func(report.Format) (string, error) {
						return "", errors.New("error generating report")
					},
				},
			},
			emailServer:   "127.0.0.1",
			emailPort:     587,
			emailUsername: "user",
			emailPassword: "abc123",
			emailFrom:     "test@example.com",
			emailTo: []string{
				"user@example.com",
			},
			format:        report.FormatPlain,
			expectedError: errors.New("error generating report"),
		},
		{
			description: "it should detect an error while sending the e-mail",
			emailSender: toglacier.EmailSenderFunc(func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
				return errors.New("generic error while sending e-mail")
			}),
			emailServer:   "127.0.0.1",
			emailPort:     587,
			emailUsername: "user",
			emailPassword: "abc123",
			emailFrom:     "test@example.com",
			emailTo: []string{
				"user@example.com",
			},
			format:        report.FormatPlain,
			expectedError: errors.New("generic error while sending e-mail"),
		},
	}

	for _, scenario := range scenarios {
		report.Clear()

		t.Run(scenario.description, func(t *testing.T) {
			toGlacier := toglacier.ToGlacier{}

			for _, r := range scenario.reports {
				report.Add(r)
			}

			emailInfo := toglacier.EmailInfo{
				Sender:   scenario.emailSender,
				Server:   scenario.emailServer,
				Port:     scenario.emailPort,
				Username: scenario.emailUsername,
				Password: scenario.emailPassword,
				From:     scenario.emailFrom,
				To:       scenario.emailTo,
				Format:   scenario.format,
			}

			if err := toGlacier.SendReport(emailInfo); !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

type mockArchive struct {
	mockBuild        func(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error)
	mockExtract      func(filename string, filter []string) (archive.Info, error)
	mockFileChecksum func(filename string) (string, error)
}

func (m mockArchive) Build(lastArchiveInfo archive.Info, backupPaths ...string) (string, archive.Info, error) {
	return m.mockBuild(lastArchiveInfo, backupPaths...)
}

func (m mockArchive) Extract(filename string, filter []string) (archive.Info, error) {
	return m.mockExtract(filename, filter)
}

func (m mockArchive) FileChecksum(filename string) (string, error) {
	return m.mockFileChecksum(filename)
}

type mockEnvelop struct {
	mockEncrypt func(filename, secret string) (string, error)
	mockDecrypt func(encryptedFilename, secret string) (string, error)
}

func (m mockEnvelop) Encrypt(filename, secret string) (string, error) {
	return m.mockEncrypt(filename, secret)
}

func (m mockEnvelop) Decrypt(encryptedFilename, secret string) (string, error) {
	return m.mockDecrypt(encryptedFilename, secret)
}

type mockCloud struct {
	mockSend   func(filename string) (cloud.Backup, error)
	mockList   func() ([]cloud.Backup, error)
	mockGet    func(id ...string) (filenames map[string]string, err error)
	mockRemove func(id string) error
}

func (m mockCloud) Send(ctx context.Context, filename string) (cloud.Backup, error) {
	return m.mockSend(filename)
}

func (m mockCloud) List(ctx context.Context) ([]cloud.Backup, error) {
	return m.mockList()
}

func (m mockCloud) Get(ctx context.Context, id ...string) (filenames map[string]string, err error) {
	return m.mockGet(id...)
}

func (m mockCloud) Remove(ctx context.Context, id string) error {
	return m.mockRemove(id)
}

type mockStorage struct {
	mockSave   func(storage.Backup) error
	mockList   func() (storage.Backups, error)
	mockRemove func(id string) error
}

func (m mockStorage) Save(b storage.Backup) error {
	return m.mockSave(b)
}

func (m mockStorage) List() (storage.Backups, error) {
	return m.mockList()
}

func (m mockStorage) Remove(id string) error {
	return m.mockRemove(id)
}

type mockReport struct {
	mockBuild func(report.Format) (string, error)
}

func (r mockReport) Build(f report.Format) (string, error) {
	return r.mockBuild(f)
}

type mockLogger struct {
	mockDebug    func(args ...interface{})
	mockDebugf   func(format string, args ...interface{})
	mockInfo     func(args ...interface{})
	mockInfof    func(format string, args ...interface{})
	mockWarning  func(args ...interface{})
	mockWarningf func(format string, args ...interface{})
}

func (m mockLogger) Debug(args ...interface{}) {
	m.mockDebug(args...)
}

func (m mockLogger) Debugf(format string, args ...interface{}) {
	m.mockDebugf(format, args...)
}

func (m mockLogger) Info(args ...interface{}) {
	m.mockInfo(args...)
}

func (m mockLogger) Infof(format string, args ...interface{}) {
	m.mockInfof(format, args...)
}

func (m mockLogger) Warning(args ...interface{}) {
	m.mockWarning(args...)
}

func (m mockLogger) Warningf(format string, args ...interface{}) {
	m.mockWarningf(format, args...)
}

// ErrorEqual compares the errors messages. This is useful in unit tests to
// compare encapsulated error messages.
func ErrorEqual(first, second error) bool {
	first = errors.Cause(first)
	second = errors.Cause(second)

	if first == nil || second == nil {
		return first == second
	}

	return first.Error() == second.Error()
}

// Diff is useful to see the difference when comparing two complex types.
func Diff(a, b interface{}) []difflib.DiffRecord {
	return difflib.Diff(strings.SplitAfter(spew.Sdump(a), "\n"), strings.SplitAfter(spew.Sdump(b), "\n"))
}
