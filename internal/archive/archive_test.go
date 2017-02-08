package archive_test

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"regexp"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/archive"
)

func TestBuild(t *testing.T) {
	scenarios := []struct {
		description   string
		backupPaths   []string
		expected      func(filename string) error
		expectedError error
	}{
		{
			description: "it should create an archive correctly",
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

				// add an empty directory to see if it's going to be ignored
				return []string{d, ""}
			}(),
			expected: func(filename string) error {
				f, err := os.Open(filename)
				if err != nil {
					return fmt.Errorf("error opening archive. details: %s", err)
				}
				defer f.Close()

				basePath := path.Join(`backup-[0-9]+`, os.TempDir(), `toglacier-test[0-9]+`)
				expectedFiles := []*regexp.Regexp{
					regexp.MustCompile(`^` + path.Join(basePath, `file1`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `file2`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `dir1`, `file3`) + `$`),
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
		},
		{
			description: "it should detect when the path does not exist",
			backupPaths: func() []string {
				return []string{"idontexist12345"}
			}(),
			expectedError: fmt.Errorf("error reading path “idontexist12345”. details: %s", &os.PathError{
				Op:   "open",
				Path: "idontexist12345",
				Err:  errors.New("no such file or directory"),
			}),
		},
	}

	for _, scenario := range scenarios {
		filename, err := archive.Build(scenario.backupPaths...)
		if scenario.expectedError == nil && scenario.expected != nil {
			if err := scenario.expected(filename); err != nil {
				t.Errorf("unexpected archive content (%s). details: %s", filename, err)
			}
		}
		if !reflect.DeepEqual(scenario.expectedError, err) {
			t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
		}
	}
}
