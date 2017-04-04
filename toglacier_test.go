package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/smtp"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aryann/difflib"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/report"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestBackup(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		backupPaths   []string
		backupSecret  string
		builder       archive.Builder
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
			builder: mockBuilder{
				mockBuild: func(backupPaths ...string) (string, error) {
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
				mockSave: func(b cloud.Backup) error {
					return nil
				},
			},
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
			builder: mockBuilder{
				mockBuild: func(backupPaths ...string) (string, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil
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
				mockSave: func(b cloud.Backup) error {
					return nil
				},
			},
		},
		{
			description: "it should detect an error while building the package",
			backupPaths: func() []string {
				return []string{"idontexist12345"}
			}(),
			builder: mockBuilder{
				mockBuild: func(backupPaths ...string) (string, error) {
					return "", errors.New("path doesn't exist")
				},
			},
			expectedError: errors.New("path doesn't exist"),
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
			builder: mockBuilder{
				mockBuild: func(backupPaths ...string) (string, error) {
					f, err := ioutil.TempFile("", "toglacier-test")
					if err != nil {
						t.Fatalf("error creating temporary file. details: %s", err)
					}
					defer f.Close()

					return f.Name(), nil
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
				mockSave: func(b cloud.Backup) error {
					return nil
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
			builder: mockBuilder{
				mockBuild: func(backupPaths ...string) (string, error) {
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
					return cloud.Backup{}, errors.New("error sending backup")
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
			builder: mockBuilder{
				mockBuild: func(backupPaths ...string) (string, error) {
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
				mockSave: func(b cloud.Backup) error {
					return errors.New("error saving the backup information")
				},
			},
			expectedError: errors.New("error saving the backup information"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			err := backup(scenario.backupPaths, scenario.backupSecret, scenario.builder, scenario.envelop, scenario.cloud, scenario.storage)

			if !archive.ErrorEqual(scenario.expectedError, err) && !archive.PathErrorEqual(scenario.expectedError, err) && !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestListBackups(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		remote        bool
		cloud         cloud.Cloud
		storage       storage.Storage
		expected      []cloud.Backup
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
				mockSave: func(b cloud.Backup) error {
					if b.ID != "123456" {
						return fmt.Errorf("adding unexpected id %s", b.ID)
					}

					return nil
				},
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123454",
							CreatedAt: now.Add(-time.Second),
							Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
							VaultName: "test",
						},
						{
							ID:        "123455",
							CreatedAt: now.Add(-time.Minute),
							Checksum:  "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
							VaultName: "test",
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
			expected: []cloud.Backup{
				{
					ID:        "123456",
					CreatedAt: now,
					Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
					VaultName: "test",
				},
			},
		},
		{
			description: "it should list the local backups correctly",
			storage: mockStorage{
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
			expected: []cloud.Backup{
				{
					ID:        "123456",
					CreatedAt: now,
					Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
					VaultName: "test",
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
			expectedError: errors.New("error listing backups"),
		},
		{
			description: "it should detect an error while listing the local backups",
			storage: mockStorage{
				mockList: func() ([]cloud.Backup, error) {
					return nil, errors.New("error listing backups")
				},
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
				mockSave: func(b cloud.Backup) error {
					if b.ID != "123456" {
						return fmt.Errorf("adding unexpected id %s", b.ID)
					}

					return nil
				},
				mockList: func() ([]cloud.Backup, error) {
					return nil, errors.New("error retrieving backups")
				},
				mockRemove: func(id string) error {
					if id != "123454" && id != "123455" {
						return fmt.Errorf("removing unexpected id %s", id)
					}

					return nil
				},
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
				mockSave: func(b cloud.Backup) error {
					if b.ID != "123456" {
						return fmt.Errorf("adding unexpected id %s", b.ID)
					}

					return nil
				},
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123454",
							CreatedAt: now.Add(-time.Second),
							Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
							VaultName: "test",
						},
						{
							ID:        "123455",
							CreatedAt: now.Add(-time.Minute),
							Checksum:  "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
							VaultName: "test",
						},
					}, nil
				},
				mockRemove: func(id string) error {
					return errors.New("error removing backup")
				},
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
				mockSave: func(b cloud.Backup) error {
					return errors.New("error adding backup")
				},
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123454",
							CreatedAt: now.Add(-time.Second),
							Checksum:  "03c7c9c26fbb71dbc1546fd2fd5f2fbc3f4a410360e8fc016c41593b2456cf59",
							VaultName: "test",
						},
						{
							ID:        "123455",
							CreatedAt: now.Add(-time.Minute),
							Checksum:  "49ddf1762657fa04e29aa8ca6b22a848ce8a9b590748d6d708dd208309bcfee6",
							VaultName: "test",
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
			expectedError: errors.New("error adding backup"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			backups, err := listBackups(scenario.remote, scenario.cloud, scenario.storage)

			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backups))
			}

			if !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestRetrieveBackup(t *testing.T) {
	scenarios := []struct {
		description   string
		id            string
		backupSecret  string
		envelop       archive.Envelop
		cloud         cloud.Cloud
		expected      string
		expectedError error
	}{
		{
			description: "it should retrieve a backup correctly",
			cloud: mockCloud{
				mockGet: func(id string) (filename string, err error) {
					return "toglacier-archive.tar.gz", nil
				},
			},
			expected: "toglacier-archive.tar.gz",
		},
		{
			description:  "it should retrieve an encrypted backup correctly",
			backupSecret: "1234567890123456",
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
				mockGet: func(id string) (filename string, err error) {
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

					return n, nil
				},
			},
			expected: path.Join(os.TempDir(), "toglacier-test-getenc"),
		},
		{
			description: "it should detect when there's an error retrieving a backup",
			cloud: mockCloud{
				mockGet: func(id string) (filename string, err error) {
					return "", errors.New("error retrieving the backup")
				},
			},
			expectedError: errors.New("error retrieving the backup"),
		},
		{
			description:  "it should detect an error decrypting the backup",
			backupSecret: "123456",
			envelop: mockEnvelop{
				mockDecrypt: func(encryptedFilename, secret string) (string, error) {
					return "", errors.New("invalid encrypted content")
				},
			},
			cloud: mockCloud{
				mockGet: func(id string) (filename string, err error) {
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

					return n, nil
				},
			},
			expectedError: errors.New("invalid encrypted content"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			filename, err := retrieveBackup(scenario.id, scenario.backupSecret, scenario.envelop, scenario.cloud)

			if !reflect.DeepEqual(scenario.expected, filename) {
				t.Errorf("filenames don't match. expected “%s” and got “%s”", scenario.expected, filename)
			}

			if !archive.ErrorEqual(scenario.expectedError, err) && !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestRemoveBackup(t *testing.T) {
	scenarios := []struct {
		description   string
		id            string
		cloud         cloud.Cloud
		storage       storage.Storage
		expectedError error
	}{
		{
			description: "it should remove a backup correctly",
			id:          "123456",
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return nil
				},
			},
			storage: mockStorage{
				mockRemove: func(id string) error {
					return nil
				},
			},
		},
		{
			description: "it should detect an error while removing the remote backup",
			id:          "123456",
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
			id:          "123456",
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
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			err := removeBackup(scenario.id, scenario.cloud, scenario.storage)

			if !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestRemoveOldBackups(t *testing.T) {
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
					if id != "123458" {
						return fmt.Errorf("unexpected id %s", id)
					}
					return nil
				},
			},
			storage: mockStorage{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
						{
							ID:        "123457",
							CreatedAt: now.Add(time.Second),
							Checksum:  "0484ed70359cd1a4337d16a4143a3d247e0a3ecbce01482c318d709ed5161016",
							VaultName: "test",
						},
						{
							ID:        "123458",
							CreatedAt: now.Add(time.Minute),
							Checksum:  "5f9c426fb1e150c1c09dda260bb962c7602b595df7586a1f3899735b839b138f",
							VaultName: "test",
						},
					}, nil
				},
				mockRemove: func(id string) error {
					if id != "123458" {
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
				mockList: func() ([]cloud.Backup, error) {
					return nil, errors.New("local storage corrupted")
				},
			},
			expectedError: errors.New("local storage corrupted"),
		},
		{
			description: "it should detect when there's an error removing an old backup from the cloud",
			keepBackups: 2,
			cloud: mockCloud{
				mockRemove: func(id string) error {
					return errors.New("backup not found")
				},
			},
			storage: mockStorage{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
						{
							ID:        "123457",
							CreatedAt: now.Add(time.Second),
							Checksum:  "0484ed70359cd1a4337d16a4143a3d247e0a3ecbce01482c318d709ed5161016",
							VaultName: "test",
						},
						{
							ID:        "123458",
							CreatedAt: now.Add(time.Minute),
							Checksum:  "5f9c426fb1e150c1c09dda260bb962c7602b595df7586a1f3899735b839b138f",
							VaultName: "test",
						},
					}, nil
				},
				mockRemove: func(id string) error {
					if id != "123458" {
						return fmt.Errorf("removing unexpected id %s", id)
					}
					return nil
				},
			},
			expectedError: errors.New("backup not found"),
		},
		{
			description: "it should detect when there's an error removing an old backup from the local storage",
			keepBackups: 2,
			cloud: mockCloud{
				mockRemove: func(id string) error {
					if id != "123458" {
						return fmt.Errorf("unexpected id %s", id)
					}
					return nil
				},
			},
			storage: mockStorage{
				mockList: func() ([]cloud.Backup, error) {
					return []cloud.Backup{
						{
							ID:        "123456",
							CreatedAt: now,
							Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
							VaultName: "test",
						},
						{
							ID:        "123457",
							CreatedAt: now.Add(time.Second),
							Checksum:  "0484ed70359cd1a4337d16a4143a3d247e0a3ecbce01482c318d709ed5161016",
							VaultName: "test",
						},
						{
							ID:        "123458",
							CreatedAt: now.Add(time.Minute),
							Checksum:  "5f9c426fb1e150c1c09dda260bb962c7602b595df7586a1f3899735b839b138f",
							VaultName: "test",
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
			err := removeOldBackups(scenario.keepBackups, scenario.cloud, scenario.storage)

			if !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestSendReport(t *testing.T) {
	date := time.Date(2017, 3, 10, 14, 10, 46, 0, time.UTC)

	scenarios := []struct {
		description   string
		reports       []report.Report
		emailSender   emailSender
		emailServer   string
		emailPort     int
		emailUsername string
		emailPassword string
		emailFrom     string
		emailTo       []string
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
			emailSender: emailSenderFunc(func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
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
		},
		{
			description: "it should fail to build the reports",
			reports: []report.Report{
				mockReport{
					mockBuild: func() (string, error) {
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
			expectedError: errors.New("error generating report"),
		},
		{
			description: "it should detect an error while sending the e-mail",
			emailSender: emailSenderFunc(func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
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
			expectedError: errors.New("generic error while sending e-mail"),
		},
	}

	for _, scenario := range scenarios {
		report.Clear()

		t.Run(scenario.description, func(t *testing.T) {
			for _, r := range scenario.reports {
				report.Add(r)
			}

			err := sendReport(scenario.emailSender, scenario.emailServer, scenario.emailPort, scenario.emailUsername, scenario.emailPassword, scenario.emailFrom, scenario.emailTo)

			if !ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

type mockBuilder struct {
	mockBuild func(backupPaths ...string) (string, error)
}

func (m mockBuilder) Build(backupPaths ...string) (string, error) {
	return m.mockBuild(backupPaths...)
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
	mockGet    func(id string) (filename string, err error)
	mockRemove func(id string) error
}

func (m mockCloud) Send(filename string) (cloud.Backup, error) {
	return m.mockSend(filename)
}

func (m mockCloud) List() ([]cloud.Backup, error) {
	return m.mockList()
}

func (m mockCloud) Get(id string) (filename string, err error) {
	return m.mockGet(id)
}

func (m mockCloud) Remove(id string) error {
	return m.mockRemove(id)
}

type mockStorage struct {
	mockSave   func(cloud.Backup) error
	mockList   func() ([]cloud.Backup, error)
	mockRemove func(id string) error
}

func (m mockStorage) Save(b cloud.Backup) error {
	return m.mockSave(b)
}

func (m mockStorage) List() ([]cloud.Backup, error) {
	return m.mockList()
}

func (m mockStorage) Remove(id string) error {
	return m.mockRemove(id)
}

type mockReport struct {
	mockBuild func() (string, error)
}

func (r mockReport) Build() (string, error) {
	return r.mockBuild()
}

type mockLog struct {
	mockDebug  func(args ...interface{})
	mockDebugf func(format string, args ...interface{})
	mockInfo   func(args ...interface{})
	mockInfof  func(format string, args ...interface{})
}

func (m mockLog) Debug(args ...interface{}) {
	m.mockDebug(args...)
}
func (m mockLog) Debugf(format string, args ...interface{}) {
	m.mockDebugf(format, args...)
}
func (m mockLog) Info(args ...interface{}) {
	m.mockInfo(args...)
}
func (m mockLog) Infof(format string, args ...interface{}) {
	m.mockInfof(format, args...)
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
