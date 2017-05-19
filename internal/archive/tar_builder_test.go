package archive_test

import (
	"archive/tar"
	"encoding/json"
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
						ID:       "reference1",
						Status:   archive.ItemInfoStatusNew,
						Checksum: "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file2"): {
						ID:       "reference2",
						Status:   archive.ItemInfoStatusNew,
						Checksum: "abcdefghijklmnopqrstuvxz1234567890ABCDEFGHI=",
					},
					path.Join(backupPaths[0], "file3"): {
						ID:       "reference3",
						Status:   archive.ItemInfoStatusDeleted,
						Checksum: "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
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
						ID:       "reference1",
						Status:   archive.ItemInfoStatusUnmodified,
						Checksum: "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file2"): {
						Status:   archive.ItemInfoStatusModified,
						Checksum: "xZzITM+6yGsa9masWjGdi+yAA0DlqCzTf/1795fy5Pk=",
					},
					path.Join(backupPaths[0], "dir1", "file3"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
					},
				})
			},
		},
		{
			description: "it should ignore the build when all files are unmodified",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			lastArchiveInfo: func(backupPaths []string) archive.Info {
				return archive.Info{
					path.Join(backupPaths[0], "file1"): {
						ID:       "reference1",
						Status:   archive.ItemInfoStatusNew,
						Checksum: "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file2"): {
						ID:       "reference2",
						Status:   archive.ItemInfoStatusNew,
						Checksum: "xZzITM+6yGsa9masWjGdi+yAA0DlqCzTf/1795fy5Pk=",
					},
					path.Join(backupPaths[0], "dir1", "file3"): {
						ID:       "reference3",
						Status:   archive.ItemInfoStatusNew,
						Checksum: "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
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

				if err := os.Mkdir(path.Join(d, "dir1"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary directory. details %s", err)
				}

				if err := ioutil.WriteFile(path.Join(d, "dir1", "file3"), []byte("file3 test"), os.ModePerm); err != nil {
					t.Fatalf("error creating temporary file. details %s", err)
				}

				return []string{d}
			}(),
			expected: func(filename string) error {
				if filename != "" {
					return fmt.Errorf("unexpected tar file “%s” when all files where unchanged", filename)
				}

				return nil
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
			lastArchiveInfo: func(backupPaths []string) archive.Info {
				return archive.Info{
					path.Join(backupPaths[0], "old-file1"): {
						ID:       "reference1",
						Status:   archive.ItemInfoStatusNew,
						Checksum: "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "old-file2"): {
						ID:       "reference1",
						Status:   archive.ItemInfoStatusDeleted,
						Checksum: "ffa0a67cec9c5ca1d0b18e7bba59430d450378ced0cd24185a5afc8094783d5a",
					},
				}
			},
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
					path.Join(backupPaths[0], "old-file1"): {
						ID:       "reference1",
						Status:   archive.ItemInfoStatusDeleted,
						Checksum: "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file1"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "+pJSD0LPX/FSn3AwOnGKsCXJSMN3o9JPyWzVv4RYqpU=",
					},
					path.Join(backupPaths[0], "file2"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "xZzITM+6yGsa9masWjGdi+yAA0DlqCzTf/1795fy5Pk=",
					},
					path.Join(backupPaths[0], "dir1", "file3"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "sFwN7pdLHnHZHCmTuhFWYvYTYz9g8XzISkAR1+UOS5c=",
					},
					path.Join(backupPaths[1], "file1"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "jtq4nMeFuT6h3DIgwFQ4sEQUlA/E9YVFlWkY5B6pxNw=",
					},
					path.Join(backupPaths[1], "file4"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "Rk2kHsOWFY5FFhsZrR5ykkCwc9WoZCWk/hEKbGhcCac=",
					},
					path.Join(backupPaths[1], "file5"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "VR88iTpGdm/q+zl26Ko0GPkgZOtZy0R0/zdoFK6Y3Uw=",
					},
					path.Join(backupPaths[1], "dir2", "file6"): {
						Status:   archive.ItemInfoStatusNew,
						Checksum: "Js5UlbJQRd2Ve3Nmoo7wfctK38eFEcHhlOUdApQKwnQ=",
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
						Status:   archive.ItemInfoStatusNew,
						Checksum: "ih/0rvVdKZfnQdoKwTj5gbNVE+Re3o7D+woelvakOiE=",
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
	writeDir := func(tarArchive *tar.Writer, baseDir string) string {
		dir, err := ioutil.TempDir("", "toglacier-test")
		if err != nil {
			t.Fatalf("error creating temporary directory. details %s", err)
		}

		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("error retrieving directory information. details %s", err)
		}

		header, err := tar.FileInfoHeader(info, path.Base(dir))
		if err != nil {
			t.Fatalf("error creating tar header. details %s", err)
		}
		header.Name = filepath.Join(baseDir, path.Base(dir)) + "/"

		if err = tarArchive.WriteHeader(header); err != nil {
			t.Fatalf("error writing tar header. details %s", err)
		}

		return path.Base(dir)
	}

	writeFile := func(tarArchive *tar.Writer, baseDir, name, content string) string {
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

		if name != "" {
			header.Name = filepath.Join(baseDir, name)
		} else {
			header.Name = filepath.Join(baseDir, file.Name())
		}

		if err = tarArchive.WriteHeader(header); err != nil {
			t.Fatalf("error writing tar header. details %s", err)
		}

		if _, err = io.CopyN(tarArchive, file, info.Size()); err != nil && err != io.EOF {
			t.Fatalf("error writing content to tar. details %s", err)
		}

		return file.Name()
	}

	writeLink := func(tarArchive *tar.Writer, baseDir, filename string) string {
		linkname := filepath.Join(os.TempDir(), "toglacier-test"+time.Now().Format("20060102150405.000000000"))

		err := os.Symlink(filename, linkname)
		if err != nil {
			t.Fatalf("error creating temporary link. details %s", err)
		}

		info, err := os.Stat(linkname)
		if err != nil {
			t.Fatalf("error retrieving link information. details %s", err)
		}

		header, err := tar.FileInfoHeader(info, linkname)
		if err != nil {
			t.Fatalf("error creating tar header. details %s", err)
		}
		header.Name = filepath.Join(baseDir, linkname)

		if err = tarArchive.WriteHeader(header); err != nil {
			t.Fatalf("error writing tar header. details %s", err)
		}

		file, err := os.Open(filename)
		if err != nil {
			t.Fatalf("error opening temporary file. details %s", err)
		}

		fileInfo, err := file.Stat()
		if err != nil {
			t.Fatalf("error retrieving file information. details %s", err)
		}

		if _, err = io.CopyN(tarArchive, file, fileInfo.Size()); err != nil && err != io.EOF {
			t.Fatalf("error writing content to tar. details %s", err)
		}

		return linkname
	}

	type scenario struct {
		description         string
		builder             *archive.TARBuilder
		filename            string
		filter              []string
		expected            func() error
		expectedArchiveInfo archive.Info
		expectedError       error
		clean               func()
	}

	scenarios := []scenario{
		func() scenario {
			var s scenario
			s.description = "it should extract an archive correctly with filters"
			s.builder = archive.NewTARBuilder(mockLogger{
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

			baseDir := "backup-" + time.Now().Format("20060102150405.000000000")
			dir1 := writeDir(tarArchive, baseDir)
			file1 := writeFile(tarArchive, filepath.Join(baseDir, dir1), "", "this is test 1")
			dir2 := writeDir(tarArchive, baseDir)
			file2 := writeFile(tarArchive, filepath.Join(baseDir, dir2), "", "this is test 2")
			link1 := writeLink(tarArchive, filepath.Join(baseDir, dir2), file2)

			archiveInfo := archive.Info{
				file1: archive.ItemInfo{
					ID:       "AWS123456",
					Status:   archive.ItemInfoStatusModified,
					Checksum: "34dd713af2cf182e27310b36bf26254d5c75335f76a8f9ca4e0d0428c2bbf709",
				},
				file2: archive.ItemInfo{
					Status:   archive.ItemInfoStatusNew,
					Checksum: "d650616996f255dc8ecda15eca765a490c5b52f3fe2a3f184f38b307dcd57b51",
				},
			}

			archiveInfoData, err := json.Marshal(archiveInfo)
			if err != nil {
				t.Fatalf("error encoding archive info. details %s", err)
			}
			writeFile(tarArchive, baseDir, archive.TARInfoFilename, string(archiveInfoData))

			s.filename = tarFile.Name()
			s.filter = []string{filepath.Join("/", dir2, file2)}
			s.expected = func() error {
				filename1 := filepath.Join(baseDir, dir1, file1)

				if _, err = os.Stat(filename1); !os.IsNotExist(err) {
					return fmt.Errorf("file “%s” extracted when it shouldn't", filename1)
				}

				filename2 := filepath.Join(baseDir, dir2, file2)

				content, err := ioutil.ReadFile(filename2)
				if err != nil {
					return fmt.Errorf("error opening file “%s”. details: %s", filename2, err)
				}

				if string(content) != "this is test 2" {
					return fmt.Errorf("expected content “this is test 2” and got “%s” in file “%s”. details: %s", string(content), filename2, err)
				}

				linkname1 := filepath.Join(baseDir, dir1, link1)

				if _, err = os.Stat(linkname1); !os.IsNotExist(err) {
					return fmt.Errorf("link “%s” extracted when it shouldn't", linkname1)
				}

				return nil
			}
			s.expectedArchiveInfo = archive.Info{
				file1: archive.ItemInfo{
					ID:       "AWS123456",
					Status:   archive.ItemInfoStatusModified,
					Checksum: "34dd713af2cf182e27310b36bf26254d5c75335f76a8f9ca4e0d0428c2bbf709",
				},
				file2: archive.ItemInfo{
					Status:   archive.ItemInfoStatusNew,
					Checksum: "d650616996f255dc8ecda15eca765a490c5b52f3fe2a3f184f38b307dcd57b51",
				},
			}
			s.clean = func() {
				os.RemoveAll(baseDir)
			}
			return s
		}(),
		func() scenario {
			var s scenario
			s.description = "it should extract an archive correctly without filters"
			s.builder = archive.NewTARBuilder(mockLogger{
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

			baseDir := "backup-" + time.Now().Format("20060102150405.000000000")
			file1 := writeFile(tarArchive, baseDir, "", "this is test 1")
			file2 := writeFile(tarArchive, baseDir, "", "this is test 2")

			s.filename = tarFile.Name()
			s.expected = func() error {
				filename1 := filepath.Join(baseDir, file1)

				content, err := ioutil.ReadFile(filename1)
				if err != nil {
					return fmt.Errorf("error opening file “%s”. details: %s", filename1, err)
				}

				if string(content) != "this is test 1" {
					return fmt.Errorf("expected content “this is test 1” and got “%s” in file “%s”. details: %s", string(content), filename1, err)
				}

				filename2 := filepath.Join(baseDir, file2)

				content, err = ioutil.ReadFile(filename2)
				if err != nil {
					return fmt.Errorf("error opening file “%s”. details: %s", filename2, err)
				}

				if string(content) != "this is test 2" {
					return fmt.Errorf("expected content “this is test 2” and got “%s” in file “%s”. details: %s", string(content), filename2, err)
				}

				return nil
			}
			s.clean = func() {
				os.RemoveAll(baseDir)
			}
			return s
		}(),
		{
			description: "it should detect when the file doesn't exist",
			builder: archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			}),
			filename: path.Join(os.TempDir(), "toglacier-idontexist.tar.gz"),
			expectedError: &archive.Error{
				Filename: path.Join(os.TempDir(), "toglacier-idontexist.tar.gz"),
				Code:     archive.ErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-idontexist.tar.gz"),
					Err:  errors.New("no such file or directory"),
				},
			},
		},
		func() scenario {
			var s scenario
			s.description = "it should detect when the file isn't a TAR"
			s.builder = archive.NewTARBuilder(mockLogger{
				mockDebug:  func(args ...interface{}) {},
				mockDebugf: func(format string, args ...interface{}) {},
				mockInfo:   func(args ...interface{}) {},
				mockInfof:  func(format string, args ...interface{}) {},
			})

			file, err := ioutil.TempFile("", "toglacier-test")
			if err != nil {
				t.Fatalf("error creating temporary file. details %s", err)
			}
			defer file.Close()

			file.WriteString("I'm not a TAR")

			s.filename = file.Name()
			s.expectedError = &archive.Error{
				Filename: file.Name(),
				Code:     archive.ErrorCodeReadingTAR,
				Err:      io.ErrUnexpectedEOF,
			}

			return s
		}(),
		func() scenario {
			var s scenario
			s.description = "it should detect a corrupted archive info"
			s.builder = archive.NewTARBuilder(mockLogger{
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

			baseDir := "backup-" + time.Now().Format("20060102150405.000000000")
			writeFile(tarArchive, baseDir, archive.TARInfoFilename, "{{{{")

			s.filename = tarFile.Name()
			s.expectedError = &archive.Error{
				Filename: tarFile.Name(),
				Code:     archive.ErrorCodeDecodingInfo,
				Err:      errors.New(`invalid character '{' looking for beginning of object key string`), // json.SyntaxError message is a private attribute
			}
			s.clean = func() {
				os.RemoveAll(baseDir)
			}
			return s
		}(),
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			archiveInfo, err := scenario.builder.Extract(scenario.filename, scenario.filter)

			if scenario.expected != nil {
				if scenarioErr := scenario.expected(); scenarioErr != nil {
					t.Error(err)
				}
			}

			if !reflect.DeepEqual(scenario.expectedArchiveInfo, archiveInfo) {
				t.Errorf("archive info don't match.\n%v", Diff(scenario.expectedArchiveInfo, archiveInfo))
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
