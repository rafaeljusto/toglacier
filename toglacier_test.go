package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestBackup(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description string
		backupPaths []string
		cloud       cloud.Cloud
		storage     storage.Storage
		expectedLog *regexp.Regexp
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
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error reading path “idontexist12345”. details: open idontexist12345: no such file or directory`),
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
			cloud: mockCloud{
				mockSend: func(filename string) (cloud.Backup, error) {
					return cloud.Backup{}, errors.New("error sending backup")
				},
			},
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error sending backup`),
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
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error saving the backup information`),
		},
	}

	originalBackupPaths := backupPaths
	defer func() {
		backupPaths = originalBackupPaths
	}()

	var output bytes.Buffer
	log.SetOutput(&output)

	for _, scenario := range scenarios {
		output.Reset()

		t.Run(scenario.description, func(t *testing.T) {
			backupPaths = scenario.backupPaths
			backup(scenario.cloud, scenario.storage)

			o := strings.TrimSpace(output.String())
			if scenario.expectedLog != nil && !scenario.expectedLog.MatchString(o) {
				t.Errorf("logs don't match. expected “%s” and got “%s”", scenario.expectedLog.String(), o)
			}
		})
	}
}

func TestListBackups(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description string
		remote      bool
		cloud       cloud.Cloud
		storage     storage.Storage
		expected    []cloud.Backup
		expectedLog *regexp.Regexp
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
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error listing backups`),
		},
		{
			description: "it should detect an error while listing the local backups",
			storage: mockStorage{
				mockList: func() ([]cloud.Backup, error) {
					return nil, errors.New("error listing backups")
				},
			},
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error listing backups`),
		},
	}

	var output bytes.Buffer
	log.SetOutput(&output)

	for _, scenario := range scenarios {
		output.Reset()

		t.Run(scenario.description, func(t *testing.T) {
			backups := listBackups(scenario.remote, scenario.cloud, scenario.storage)

			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", pretty.Diff(scenario.expected, backups))
			}

			o := strings.TrimSpace(output.String())
			if scenario.expectedLog != nil && !scenario.expectedLog.MatchString(o) {
				t.Errorf("logs don't match. expected “%s” and got “%s”", scenario.expectedLog.String(), o)
			}
		})
	}
}

func TestRetrieveBackup(t *testing.T) {
	scenarios := []struct {
		description string
		id          string
		cloud       cloud.Cloud
		expected    string
		expectedLog *regexp.Regexp
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
			description: "it should detect when there's an error retrieving a backup",
			cloud: mockCloud{
				mockGet: func(id string) (filename string, err error) {
					return "", errors.New("error retrieving the backup")
				},
			},
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error retrieving the backup`),
		},
	}

	var output bytes.Buffer
	log.SetOutput(&output)

	for _, scenario := range scenarios {
		output.Reset()

		t.Run(scenario.description, func(t *testing.T) {
			filename := retrieveBackup(scenario.id, scenario.cloud)

			if !reflect.DeepEqual(scenario.expected, filename) {
				t.Errorf("filenames don't match. expected “%s” and got “%s”", scenario.expected, filename)
			}

			o := strings.TrimSpace(output.String())
			if scenario.expectedLog != nil && !scenario.expectedLog.MatchString(o) {
				t.Errorf("logs don't match. expected “%s” and got “%s”", scenario.expectedLog.String(), o)
			}
		})
	}
}

func TestRemoveBackup(t *testing.T) {
	scenarios := []struct {
		description string
		id          string
		cloud       cloud.Cloud
		storage     storage.Storage
		expectedLog *regexp.Regexp
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
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error removing backup`),
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
			expectedLog: regexp.MustCompile(`[0-9]+/[0-9]+/[0-9]+ [0-9]+:[0-9]+:[0-9]+ error removing backup`),
		},
	}

	var output bytes.Buffer
	log.SetOutput(&output)

	for _, scenario := range scenarios {
		output.Reset()

		t.Run(scenario.description, func(t *testing.T) {
			removeBackup(scenario.id, scenario.cloud, scenario.storage)

			o := strings.TrimSpace(output.String())
			if scenario.expectedLog != nil && !scenario.expectedLog.MatchString(o) {
				t.Errorf("logs don't match. expected “%s” and got “%s”", scenario.expectedLog.String(), o)
			}
		})
	}
}

func TestRemoveOldBackups(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description string
		keepBackups int
		cloud       cloud.Cloud
		storage     storage.Storage
		expectedLog *regexp.Regexp
	}{
		{
			description: "it should remove all old backups correctly",
			keepBackups: 2,
			cloud: mockCloud{
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
						return fmt.Errorf("unexpected id %s", id)
					}
					return nil
				},
			},
			storage: mockStorage{
				mockRemove: func(id string) error {
					if id != "123458" {
						return fmt.Errorf("unexpected id %s", id)
					}
					return nil
				},
			},
		},
	}

	originalKeepBackups := keepBackups
	defer func() {
		keepBackups = originalKeepBackups
	}()

	var output bytes.Buffer
	log.SetOutput(&output)

	for _, scenario := range scenarios {
		output.Reset()

		t.Run(scenario.description, func(t *testing.T) {
			keepBackups = scenario.keepBackups
			removeOldBackups(scenario.cloud, scenario.storage)

			o := strings.TrimSpace(output.String())
			if scenario.expectedLog != nil && !scenario.expectedLog.MatchString(o) {
				t.Errorf("logs don't match. expected “%s” and got “%s”", scenario.expectedLog.String(), o)
			}
		})
	}
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
