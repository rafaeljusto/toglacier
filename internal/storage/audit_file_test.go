package storage_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestAuditFile_Save(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		filename      string
		backup        cloud.Backup
		expected      string
		expectedError error
	}{
		{
			description: "it should save a backup information correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test")
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
			expected: fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)),
		},
		{
			description: "it should detect when the filename refers to a directory",
			filename: func() string {
				d := path.Join(os.TempDir(), "toglacier-test-dir")
				if err := os.MkdirAll(d, os.ModePerm); err != nil {
					t.Fatalf("error creating a temporary directory. details: %s", err)
				}
				return d
			}(),
			backup: cloud.Backup{
				ID:        "123456",
				CreatedAt: now,
				Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
				VaultName: "test",
			},
			expectedError: fmt.Errorf("error opening the audit file. details: %s", &os.PathError{
				Op:   "open",
				Path: path.Join(os.TempDir(), "toglacier-test-dir"),
				Err:  errors.New("is a directory"),
			}),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			auditFile := storage.NewAuditFile(scenario.filename)
			err := auditFile.Save(scenario.backup)

			auditFileContent, auditFileErr := ioutil.ReadFile(scenario.filename)
			if auditFileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading audit file. details: %s", auditFileErr)
			}

			if !reflect.DeepEqual(scenario.expected, string(auditFileContent)) {
				t.Errorf("audit file don't match. expected “%s” and got “%s”", scenario.expectedError, string(auditFileContent))
			}

			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAuditFile_List(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		filename      string
		expected      []cloud.Backup
		expectedError error
	}{
		{
			description: "it should list all backups information correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating a temporary file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)))
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
		{
			description: "it should detect when the audit file has no read permission",
			filename: func() string {
				n := path.Join(os.TempDir(), "toglacier-test-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					f, err := os.OpenFile(n, os.O_CREATE, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary file. details: %s", err)
					}
					defer f.Close()

					f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)))
				}

				return n
			}(),
			expectedError: fmt.Errorf("error opening the audit file. details: %s", &os.PathError{
				Op:   "open",
				Path: path.Join(os.TempDir(), "toglacier-test-noperm"),
				Err:  errors.New("permission denied"),
			}),
		},
		{
			description: "it should detect when the filename references to a directory",
			filename: func() string {
				d := path.Join(os.TempDir(), "toglacier-test-dir")
				if err := os.MkdirAll(d, os.ModePerm); err != nil {
					t.Fatalf("error creating a temporary directory. details: %s", err)
				}
				return d
			}(),
			expectedError: fmt.Errorf("error reading the audit file. details: %s", &os.PathError{
				Op:   "read",
				Path: path.Join(os.TempDir(), "toglacier-test-dir"),
				Err:  errors.New("is a directory"),
			}),
		},
		{
			description: "it should detect when an audit file line has the wrong number of columns",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating a temporary file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456\n", now.Format(time.RFC3339)))
				return f.Name()
			}(),
			expectedError: errors.New("corrupted audit file. wrong number of columns"),
		},
		{
			description: "it should detect when the audit file contains an invalid date",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating a temporary file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("XXXX test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n")
				return f.Name()
			}(),
			expectedError: fmt.Errorf("corrupted audit file. invalid date format. details: %s", &time.ParseError{
				Layout:     time.RFC3339,
				Value:      "XXXX",
				LayoutElem: "2006",
				ValueElem:  "XXXX",
				Message:    fmt.Sprintf(` as "%s": cannot parse "XXXX" as "2006"`, time.RFC3339),
			}),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			auditFile := storage.NewAuditFile(scenario.filename)
			backups, err := auditFile.List()

			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", pretty.Diff(scenario.expected, backups))
			}

			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAuditFile_Remove(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		filename      string
		id            string
		expected      string
		expectedError error
	}{
		{
			description: "it should remove a backup information correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating a temporary file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)))
				f.WriteString(fmt.Sprintf("%s test 123457 913b87897ffb6dca07e9f17e280aa8ecb9886dffeda8a15efeafec11dec0d108\n", now.Add(time.Second).Format(time.RFC3339)))
				return f.Name()
			}(),
			id:       "123457",
			expected: fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)),
		},
		{
			description: "it should detect when the audit file has no read permission",
			filename: func() string {
				n := path.Join(os.TempDir(), "toglacier-test-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					f, err := os.OpenFile(n, os.O_CREATE, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary file. details: %s", err)
					}
					defer f.Close()

					f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)))
				}

				return n
			}(),
			id: "123456",
			expectedError: fmt.Errorf("error opening the audit file. details: %s", &os.PathError{
				Op:   "open",
				Path: path.Join(os.TempDir(), "toglacier-test-noperm"),
				Err:  errors.New("permission denied"),
			}),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			auditFile := storage.NewAuditFile(scenario.filename)
			err := auditFile.Remove(scenario.id)

			auditFileContent, auditFileErr := ioutil.ReadFile(scenario.filename)
			if auditFileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading audit file. details: %s", auditFileErr)
			}

			if !reflect.DeepEqual(scenario.expected, string(auditFileContent)) {
				t.Errorf("audit file don't match. expected “%s” and got “%s”", scenario.expectedError, string(auditFileContent))
			}

			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}
