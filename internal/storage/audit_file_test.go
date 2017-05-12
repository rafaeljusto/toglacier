package storage_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aryann/difflib"
	"github.com/davecgh/go-spew/spew"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/log"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestAuditFile_Save(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		logger        log.Logger
		filename      string
		backup        storage.Backup
		expected      string
		expectedError error
	}{
		{
			description: "it should save a backup information correctly",
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
			expected: fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 120\n", now.Format(time.RFC3339)),
		},
		{
			description: "it should detect when the filename refers to a directory",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
				d := path.Join(os.TempDir(), "toglacier-test-dir")
				if err := os.MkdirAll(d, os.ModePerm); err != nil {
					t.Fatalf("error creating a temporary directory. details: %s", err)
				}
				return d
			}(),
			backup: storage.Backup{
				Backup: cloud.Backup{
					ID:        "123456",
					CreatedAt: now,
					Checksum:  "ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7",
					VaultName: "test",
				},
			},
			expectedError: &storage.Error{
				Code: storage.ErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-test-dir"),
					Err:  errors.New("is a directory"),
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			auditFile := storage.NewAuditFile(scenario.logger, scenario.filename)
			err := auditFile.Save(scenario.backup)

			auditFileContent, auditFileErr := ioutil.ReadFile(scenario.filename)
			if auditFileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading audit file. details: %s", auditFileErr)
			}

			if !reflect.DeepEqual(scenario.expected, string(auditFileContent)) {
				t.Errorf("audit file don't match. expected “%s” and got “%s”", scenario.expected, string(auditFileContent))
			}

			if !storage.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAuditFile_List(t *testing.T) {
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
				f, err := ioutil.TempFile("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating a temporary file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 120\n", now.Format(time.RFC3339)))
				f.WriteString(fmt.Sprintf("%s test 654321 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 120\n", now.Add(time.Second).Format(time.RFC3339)))
				return f.Name()
			}(),
			expected: storage.Backups{
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
			},
		},
		{
			description: "it should list all backups information correctly with format transition",
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
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 0  \n", now.Format(time.RFC3339)))
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
					},
				},
			},
		},
		{
			description: "it should list all backups information correctly with backward compatibility",
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
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)))
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
					},
				},
			},
		},
		{
			description: "it should return no backups when the audit file doesn't exist",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
				return path.Join(os.TempDir(), "toglacier-idontexist")
			}(),
		},
		{
			description: "it should detect when the audit file has no read permission",
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

					f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 120\n", now.Format(time.RFC3339)))
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
			description: "it should detect when the filename references to a directory",
			logger: mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			},
			filename: func() string {
				d := path.Join(os.TempDir(), "toglacier-test-dir")
				if err := os.MkdirAll(d, os.ModePerm); err != nil {
					t.Fatalf("error creating a temporary directory. details: %s", err)
				}
				return d
			}(),
			expectedError: &storage.Error{
				Code: storage.ErrorCodeReadingFile,
				Err: &os.PathError{
					Op:   "read",
					Path: path.Join(os.TempDir(), "toglacier-test-dir"),
					Err:  errors.New("is a directory"),
				},
			},
		},
		{
			description: "it should detect when an audit file line has the wrong number of columns",
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
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456\n", now.Format(time.RFC3339)))
				return f.Name()
			}(),
			expectedError: &storage.Error{
				Code: storage.ErrorCodeFormat,
			},
		},
		{
			description: "it should detect when the audit file contains an invalid date",
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
				defer f.Close()

				f.WriteString("XXXX test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 120\n")
				return f.Name()
			}(),
			expectedError: &storage.Error{
				Code: storage.ErrorCodeDateFormat,
				Err: &time.ParseError{
					Layout:     time.RFC3339,
					Value:      "XXXX",
					LayoutElem: "2006",
					ValueElem:  "XXXX",
					Message:    fmt.Sprintf(` as "%s": cannot parse "XXXX" as "2006"`, time.RFC3339),
				},
			},
		},
		{
			description: "it should detect when the audit file contains an invalid size",
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
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 XXXX\n", now.Format(time.RFC3339)))
				return f.Name()
			}(),
			expectedError: &storage.Error{
				Code: storage.ErrorCodeSizeFormat,
				Err: &strconv.NumError{
					Func: "ParseInt",
					Num:  "XXXX",
					Err:  errors.New("invalid syntax"),
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			auditFile := storage.NewAuditFile(scenario.logger, scenario.filename)
			backups, err := auditFile.List()

			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backups))
			}

			if !storage.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAuditFile_Remove(t *testing.T) {
	now := time.Now()

	scenarios := []struct {
		description   string
		logger        log.Logger
		filename      string
		id            string
		expected      string
		expectedError error
	}{
		{
			description: "it should remove a backup information correctly",
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
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 100\n", now.Format(time.RFC3339)))
				f.WriteString(fmt.Sprintf("%s test 123457 913b87897ffb6dca07e9f17e280aa8ecb9886dffeda8a15efeafec11dec0d108 200\n", now.Add(time.Second).Format(time.RFC3339)))
				return f.Name()
			}(),
			id:       "123457",
			expected: fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 100\n", now.Format(time.RFC3339)),
		},
		{
			description: "it should remove a backup information correctly with backward compatibility",
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
				defer f.Close()

				f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)))
				f.WriteString(fmt.Sprintf("%s test 123457 913b87897ffb6dca07e9f17e280aa8ecb9886dffeda8a15efeafec11dec0d108\n", now.Add(time.Second).Format(time.RFC3339)))
				return f.Name()
			}(),
			id:       "123457",
			expected: fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7 0\n", now.Format(time.RFC3339)),
		},
		{
			description: "it should detect when the audit file has no read permission",
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

					f.WriteString(fmt.Sprintf("%s test 123456 ca34f069795292e834af7ea8766e9e68fdddf3f46c7ce92ab94fc2174910adb7\n", now.Format(time.RFC3339)))
				}

				return n
			}(),
			id: "123456",
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
			auditFile := storage.NewAuditFile(scenario.logger, scenario.filename)
			err := auditFile.Remove(scenario.id)

			auditFileContent, auditFileErr := ioutil.ReadFile(scenario.filename)
			if auditFileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading audit file. details: %s", auditFileErr)
			}

			if !reflect.DeepEqual(scenario.expected, string(auditFileContent)) {
				t.Errorf("audit file don't match. expected “%s” and got “%s”", scenario.expectedError, string(auditFileContent))
			}

			if !storage.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

type mockLogger struct {
	mockDebug  func(args ...interface{})
	mockDebugf func(format string, args ...interface{})
	mockInfo   func(args ...interface{})
	mockInfof  func(format string, args ...interface{})
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

// Diff is useful to see the difference when comparing two complex types.
func Diff(a, b interface{}) []difflib.DiffRecord {
	return difflib.Diff(strings.SplitAfter(spew.Sdump(a), "\n"), strings.SplitAfter(spew.Sdump(b), "\n"))
}
