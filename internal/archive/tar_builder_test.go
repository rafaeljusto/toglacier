package archive_test

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aryann/difflib"
	"github.com/davecgh/go-spew/spew"
	"github.com/rafaeljusto/toglacier/internal/archive"
)

func TestTARBuilder_Build(t *testing.T) {
	scenarios := []struct {
		description         string
		builder             *archive.TARBuilder
		lastArchiveInfo     func(backupPaths []string) archive.Info
		backupPaths         []string
		expected            func(filename string) error
		expectedArchiveInfo func(backupPaths []string) archive.Info
		expectedError       error
	}{
		{
			description: "it should create an archive correctly from directory path",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			lastArchiveInfo: func(backupPaths []string) archive.Info {
				return archive.Info{
					path.Join(backupPaths[0], "file1"): {
						ID:     "reference1",
						Status: archive.ItemInfoStatusNew,
						Hash:   "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file2"): {
						ID:     "reference2",
						Status: archive.ItemInfoStatusNew,
						Hash:   "abcdefghijklmnopqrstuvxz1234567890ABCDEFGHI=",
					},
					path.Join(backupPaths[0], "file3"): {
						ID:     "reference3",
						Status: archive.ItemInfoStatusDeleted,
						Hash:   "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
					},
				}
			},
			backupPaths: func() []string {
				d, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "file2"), []byte("file2 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				if err := os.Symlink(path.Join(d, "file2"), path.Join(d, "link1")); err != nil {
					t.Fatalf("error creating temporary link. details %s", err)
				}

				if err := os.Mkdir(path.Join(d, "dir1"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "dir1", "file3"), []byte("file3 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				// add an empty directory to see if it's going to be ignored
				return []string{d, ""}
			}(),
			expected: func(filename string) error {
				f, err := os.Open(filename)
				if err != nil {
					return fmt.Errorf("error opening archive. details: %s", err)
				}
				defer f.Close()

				basePath := `backup-[0-9]+`
				expectedFiles := []*regexp.Regexp{
					regexp.MustCompile(`^` + path.Join(basePath, archive.TARInfoFilename) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`) + `/$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `file2`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `dir1`) + `/$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `dir1`, `file3`) + `$`),
				}

				tr := tar.NewReader(f)
				for {
					hdr, err := tr.Next()
					if err == io.EOF {
						break
					} else if err != nil {
						return err
					}

					if len(expectedFiles) == 0 {
						return fmt.Errorf("content “%s” shouldn't be here", hdr.Name)
					}

					found := false
					for i, expectedFile := range expectedFiles {
						if expectedFile.MatchString(hdr.Name) {
							expectedFiles = append(expectedFiles[:i], expectedFiles[i+1:]...)
							found = true
							break
						}
					}

					if !found {
						return fmt.Errorf("file “%s” did not match with any of the expected files", hdr.Name)
					}
				}

				if len(expectedFiles) > 0 {
					return errors.New("not all files were found in the archive")
				}

				return nil
			},
			expectedArchiveInfo: func(backupPaths []string) archive.Info {
				return archive.Info(map[string]archive.ItemInfo{
					path.Join(backupPaths[0], "file1"): {
						ID:     "reference1",
						Status: archive.ItemInfoStatusUnmodified,
						Hash:   "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file2"): {
						Status: archive.ItemInfoStatusModified,
						Hash:   "xZzITM+6yGsa9masWjGdi+yAA0DlqCzTf/1795fy5Pk=",
					},
					path.Join(backupPaths[0], "file3"): {
						ID:     "reference3",
						Status: archive.ItemInfoStatusDeleted,
						Hash:   "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
					},
					path.Join(backupPaths[0], "dir1", "file3"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
					},
				})
			},
		},
		{
			description: "it should create an archive correctly from multiple directory paths",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			backupPaths: func() []string {
				d1, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err = ioutil.WriteFile(path.Join(d1, "file1"), []byte("file1 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				if err = ioutil.WriteFile(path.Join(d1, "file2"), []byte("file2 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				if err = os.Mkdir(path.Join(d1, "dir1"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err = ioutil.WriteFile(path.Join(d1, "dir1", "file3"), []byte("file3 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				d2, err := ioutil.TempDir("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err = ioutil.WriteFile(path.Join(d2, "file1"), []byte("file1 test in dir2"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				if err = ioutil.WriteFile(path.Join(d2, "file4"), []byte("file4 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				if err = ioutil.WriteFile(path.Join(d2, "file5"), []byte("file5 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				if err = os.Mkdir(path.Join(d2, "dir2"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err = ioutil.WriteFile(path.Join(d2, "dir2", "file6"), []byte("file6 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				// add an empty directory to see if it's going to be ignored
				return []string{d1, d2}
			}(),
			expected: func(filename string) error {
				f, err := os.Open(filename)
				if err != nil {
					return fmt.Errorf("error opening archive. details: %s", err)
				}
				defer f.Close()

				basePath := `backup-[0-9]+`
				expectedFiles := []*regexp.Regexp{
					regexp.MustCompile(`^` + path.Join(basePath, archive.TARInfoFilename) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`) + `/$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `file1`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `file2`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `dir1`) + `/$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `dir1`, `file3`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`) + `/$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `file1`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `file4`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `file5`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `dir2`) + `/$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`, `dir2`, `file6`) + `$`),
				}

				tr := tar.NewReader(f)
				for {
					hdr, err := tr.Next()
					if err == io.EOF {
						break
					} else if err != nil {
						return err
					}

					if len(expectedFiles) == 0 {
						return fmt.Errorf("content “%s” shouldn't be here", hdr.Name)
					}

					found := false
					for i, expectedFile := range expectedFiles {
						if expectedFile.MatchString(hdr.Name) {
							expectedFiles = append(expectedFiles[:i], expectedFiles[i+1:]...)
							found = true
							break
						}
					}

					if !found {
						return fmt.Errorf("file “%s” did not match with any of the expected files", hdr.Name)
					}
				}

				if len(expectedFiles) > 0 {
					return errors.New("not all files were found in the archive")
				}

				return nil
			},
			expectedArchiveInfo: func(backupPaths []string) archive.Info {
				return archive.Info(map[string]archive.ItemInfo{
					path.Join(backupPaths[0], "file1"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file2"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "xZzITM+6yGsa9masWjGdi+yAA0DlqCzTf/1795fy5Pk=",
					},
					path.Join(backupPaths[0], "dir1", "file3"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
					},
					path.Join(backupPaths[1], "file1"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "jtq4nMeFuT6h3DIgwFQ4sEQUlA/E9YVFlWkY5B6pxNw=",
					},
					path.Join(backupPaths[1], "file4"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "Rk2kHsOWFY5FFhsZrR5ykkCwc9WoZCWk/hEKbGhcCac=",
					},
					path.Join(backupPaths[1], "file5"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "VR88iTpGdm/q+zl26Ko0GPkgZOtZy0R0/zdoFK6Y3Uw=",
					},
					path.Join(backupPaths[1], "dir2", "file6"): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "Js5UlbJQRd2Ve3Nmoo7wfctK38eFEcHhlOUdApQKwnQ=",
					},
				})
			},
		},
		{
			description: "it should create an archive correctly from file path",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			backupPaths: func() []string {
				f, err := ioutil.TempFile("", "toglacier-test")
				if err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}
				defer f.Close()

				f.WriteString("file test")

				return []string{f.Name()}
			}(),
			expected: func(filename string) error {
				f, err := os.Open(filename)
				if err != nil {
					return fmt.Errorf("error opening archive. details: %s", err)
				}
				defer f.Close()

				basePath := `backup-[0-9]+`
				expectedFiles := []*regexp.Regexp{
					regexp.MustCompile(`^` + path.Join(basePath, archive.TARInfoFilename) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `tmp`, `toglacier-test[0-9]+`) + `$`),
				}

				tr := tar.NewReader(f)
				for {
					hdr, err := tr.Next()
					if err == io.EOF {
						break
					} else if err != nil {
						return err
					}

					if len(expectedFiles) == 0 {
						return fmt.Errorf("content “%s” shouldn't be here", hdr.Name)
					}

					found := false
					for i, expectedFile := range expectedFiles {
						if expectedFile.MatchString(hdr.Name) {
							expectedFiles = append(expectedFiles[:i], expectedFiles[i+1:]...)
							found = true
							break
						}
					}

					if !found {
						return fmt.Errorf("file “%s” did not match with any of the expected files", hdr.Name)
					}
				}

				if len(expectedFiles) > 0 {
					return errors.New("not all files were found in the archive")
				}

				return nil
			},
			expectedArchiveInfo: func(backupPaths []string) archive.Info {
				return archive.Info(map[string]archive.ItemInfo{
					path.Join(backupPaths[0]): {
						Status: archive.ItemInfoStatusNew,
						Hash:   "ih/0rvVdKZfnQdoKwTj5gbNVE+Re3o7D+woelvakOiE=",
					},
				})
			},
		},
		{
			description: "it should detect when the path does not exist",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			backupPaths: func() []string {
				return []string{"idontexist12345"}
			}(),
			expectedError: &archive.PathError{
				Path: "idontexist12345",
				Code: archive.PathErrorCodeInfo,
				Err: &os.PathError{
					Op:   "lstat",
					Path: "idontexist12345",
					Err:  errors.New("no such file or directory"),
				},
			},
		},
		{
			description: "it should detect when the path (directory) does not have permission",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			backupPaths: func() []string {
				n := path.Join(os.TempDir(), "toglacier-test-archive-dir-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					err := os.Mkdir(n, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary directory. details: %s", err)
					}
				}

				return []string{n}
			}(),
			expectedError: &archive.PathError{
				Path: path.Join(os.TempDir(), "toglacier-test-archive-dir-noperm"),
				Code: archive.PathErrorCodeInfo,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-test-archive-dir-noperm"),
					Err:  errors.New("permission denied"),
				},
			},
		},
		{
			description: "it should detect when the path (file) does not have permission",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			backupPaths: func() []string {
				n := path.Join(os.TempDir(), "toglacier-test-archive-file-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					f, err := os.OpenFile(n, os.O_CREATE, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary file. details: %s", err)
					}
					defer f.Close()

					f.WriteString("This is a test")
				}

				return []string{n}
			}(),
			expectedError: &archive.PathError{
				Path: path.Join(os.TempDir(), "toglacier-test-archive-file-noperm"),
				Code: archive.PathErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-test-archive-file-noperm"),
					Err:  errors.New("permission denied"),
				},
			},
		},
		{
			description: "it should detect an error while walking in the path",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			backupPaths: func() []string {
				n := path.Join(os.TempDir(), "toglacier-test-archive-dir-file-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					err := os.Mkdir(n, os.FileMode(0700))
					if err != nil {
						t.Fatalf("error creating a temporary directory. details: %s", err)
					}

					f, err := os.OpenFile(path.Join(n, "file1"), os.O_CREATE, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary file. details: %s", err)
					}
					defer f.Close()

					f.WriteString("file1 test")
				}

				return []string{n}
			}(),
			expectedError: &archive.PathError{
				Path: path.Join(os.TempDir(), "toglacier-test-archive-dir-file-noperm", "file1"),
				Code: archive.PathErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-test-archive-dir-file-noperm", "file1"),
					Err:  errors.New("permission denied"),
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			backupPaths := scenario.backupPaths

			var lastArchiveInfo archive.Info
			if scenario.lastArchiveInfo != nil {
				lastArchiveInfo = scenario.lastArchiveInfo(backupPaths)
			}

			filename, archiveInfo, err := scenario.builder.Build(lastArchiveInfo, backupPaths...)
			if scenario.expectedError == nil && scenario.expected != nil {
				if err = scenario.expected(filename); err != nil {
					t.Errorf("unexpected archive content (%s). details: %s", filename, err)
				}

				if archiveInfo != nil && scenario.expectedArchiveInfo == nil {
					t.Error("unexpected archive info")

				} else if scenario.expectedArchiveInfo != nil {
					expectedArchiveInfo := scenario.expectedArchiveInfo(backupPaths)
					if !reflect.DeepEqual(expectedArchiveInfo, archiveInfo) {
						t.Errorf("archive info don't match.\n%v", Diff(expectedArchiveInfo, archiveInfo))
					}
				}
			}

			if !archive.ErrorEqual(scenario.expectedError, err) && !archive.PathErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestTARBuilder_Extract(t *testing.T) {
	writeFile := func(tarArchive *tar.Writer, baseDir, content string) string {
		file, err := ioutil.TempFile("", "toglacier-test")
		if err != nil {
			t.Fatalf("error creating temporary file. details %s", err)
		}
		defer file.Close()

		file.WriteString(content)

		info, err := file.Stat()
		if err != nil {
			t.Fatalf("error retrieving file information. details %s", err)
		}
		file.Seek(0, 0)

		header, err := tar.FileInfoHeader(info, file.Name())
		if err != nil {
			t.Fatalf("error creating tar header. details %s", err)
		}
		header.Name = filepath.Join(baseDir, file.Name())

		if err = tarArchive.WriteHeader(header); err != nil {
			t.Fatalf("error writing tar header. details %s", err)
		}

		if _, err = io.CopyN(tarArchive, file, info.Size()); err != nil && err != io.EOF {
			t.Fatalf("error writing content to tar. details %s", err)
		}

		if err := os.Remove(file.Name()); err != nil {
			t.Fatalf("error removing temporary file. details %s", err)
		}

		return file.Name()
	}

	type scenario struct {
		description   string
		builder       *archive.TARBuilder
		filename      string
		filter        []string
		expected      func() error
		expectedError error
		clean         func()
	}

	scenarios := []scenario{
		func() scenario {
			var scenario scenario
			scenario.description = "it should extract an archive correctly"
			scenario.builder = archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			})

			tarFile, err := ioutil.TempFile("", "toglacier-test")
			if err != nil {
				t.Fatalf("error creating temporary file. details %s", err)
			}
			defer tarFile.Close()

			tarArchive := tar.NewWriter(tarFile)
			defer tarArchive.Close()

			baseDir := "backup-" + time.Now().Format("20060102150405")
			file1 := writeFile(tarArchive, baseDir, "this is test 1")
			file2 := writeFile(tarArchive, baseDir, "this is test 2")

			scenario.filename = tarFile.Name()
			scenario.filter = []string{file2}
			scenario.expected = func() error {
				if _, err = os.Stat(filepath.Join(baseDir, file1)); !os.IsNotExist(err) {
					return fmt.Errorf("file “%s” extracted when it shouldn't", file1)
				}

				content, err := ioutil.ReadFile(filepath.Join(baseDir, file2))
				if err != nil {
					return fmt.Errorf("error opening file “%s”. details: %s", file2, err)
				}

				if string(content) != "this is test 2" {
					return fmt.Errorf("expected content “this is test 2” and got “%s” in file “%s”. details: %s", string(content), file2, err)
				}

				return nil
			}
			scenario.clean = func() {
				os.RemoveAll(baseDir)
			}
			return scenario
		}(),
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			err := scenario.builder.Extract(scenario.filename, scenario.filter)

			if scenario.expected != nil {
				if err := scenario.expected(); err != nil {
					t.Error(err)
				}
			}

			if !archive.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}

			if scenario.clean != nil {
				scenario.clean()
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
