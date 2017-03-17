package archive_test

import (
	"archive/tar"
	"crypto/aes"
	"crypto/rand"
	"encoding/hex"
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
			description: "it should create an archive correctly from directory path",
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

				basePath := `backup-[0-9]+`
				expectedFiles := []*regexp.Regexp{
					regexp.MustCompile(`^` + basePath + `/$`),
					regexp.MustCompile(`^` + path.Join(basePath, `file1`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `file2`) + `$`),
					regexp.MustCompile(`^` + path.Join(basePath, `dir1`) + `/$`),
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
			description: "it should create an archive correctly from file path",
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

				expectedFiles := []*regexp.Regexp{
					regexp.MustCompile(`^toglacier-test[0-9]+$`),
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
			expectedError: archive.PathError{
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
			expectedError: archive.PathError{
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
			expectedError: archive.PathError{
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
			expectedError: archive.PathError{
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
			filename, err := archive.Build(scenario.backupPaths...)
			if scenario.expectedError == nil && scenario.expected != nil {
				if err := scenario.expected(filename); err != nil {
					t.Errorf("unexpected archive content (%s). details: %s", filename, err)
				}
			}

			if !archive.ArchiveErrorEqual(scenario.expectedError, err) && !archive.PathErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestEncrypt(t *testing.T) {
	type scenario struct {
		description   string
		filename      string
		secret        string
		randomSource  io.Reader
		expectedFile  string
		expectedError error
	}

	scenarios := []scenario{
		{
			description: "it should detect when it tries to encrypt a file that doesn't exist",
			filename:    "toglacier-idontexist.tmp",
			secret:      "12345678901234567890123456789012",
			expectedError: archive.ArchiveError{
				Filename: "toglacier-idontexist.tmp",
				Code:     archive.ArchiveErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: "toglacier-idontexist.tmp",
					Err:  errors.New("no such file or directory"),
				},
			},
		},
		{
			description: "it should detect when the archive has no read permission",
			filename: func() string {
				n := path.Join(os.TempDir(), "toglacier-test-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					f, err := os.OpenFile(n, os.O_CREATE, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary file. details: %s", err)
					}
					defer f.Close()

					f.WriteString("Important information for the test backup")
				}

				return n
			}(),
			secret:       "12345678901234567890123456789012",
			randomSource: rand.Reader,
			expectedError: archive.ArchiveError{
				Filename: path.Join(os.TempDir(), "toglacier-test-noperm"),
				Code:     archive.ArchiveErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-test-noperm"),
					Err:  errors.New("permission denied"),
				},
			},
		},
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-test-")
			if err != nil {
				t.Fatalf("error creating file. details: %s", err)
			}
			defer f.Close()

			f.WriteString("Important information for the test backup")

			var scenario scenario
			scenario.description = "it should detect when the random source generates an error"
			scenario.filename = f.Name()
			scenario.secret = "1234567890123456"

			scenario.randomSource = mockReader{
				mockRead: func(p []byte) (n int, err error) {
					return 0, errors.New("random error")
				},
			}
			scenario.expectedError = archive.ArchiveError{
				Filename: f.Name(),
				Code:     archive.ArchiveErrorCodeGenerateRandomNumbers,
				Err:      errors.New("random error"),
			}

			return scenario
		}(),
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-test-")
			if err != nil {
				t.Fatalf("error creating file. details: %s", err)
			}
			defer f.Close()

			f.WriteString("Important information for the test backup")

			var scenario scenario
			scenario.description = "it should detect when the AES secret length is invalid"
			scenario.filename = f.Name()
			scenario.secret = "123456"
			scenario.randomSource = rand.Reader

			scenario.expectedError = archive.ArchiveError{
				Filename: f.Name(),
				Code:     archive.ArchiveErrorCodeInitCipher,
				Err:      aes.KeySizeError(6),
			}

			return scenario
		}(),
	}

	originalRandomSource := archive.RandomSource
	defer func() {
		archive.RandomSource = originalRandomSource
	}()

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			archive.RandomSource = scenario.randomSource
			encryptedFilename, err := archive.Encrypt(scenario.filename, scenario.secret)

			fileContent, fileErr := ioutil.ReadFile(encryptedFilename)
			if fileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading file. details: %s", fileErr)
			}

			if !reflect.DeepEqual(scenario.expectedFile, string(fileContent)) {
				t.Errorf("files don't match. expected “%s” and got “%s”", scenario.expectedFile, string(fileContent))
			}

			if !archive.ArchiveErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	type scenario struct {
		description       string
		encryptedFilename string
		secret            string
		expectedFile      string
		expectedError     error
	}

	scenarios := []scenario{
		{
			description: "it should detect when the archive has no read permission",
			encryptedFilename: func() string {
				n := path.Join(os.TempDir(), "toglacier-test-noperm")
				if _, err := os.Stat(n); os.IsNotExist(err) {
					f, err := os.OpenFile(n, os.O_CREATE, os.FileMode(0077))
					if err != nil {
						t.Fatalf("error creating a temporary file. details: %s", err)
					}
					defer f.Close()

					f.WriteString("Important information for the test backup")
				}

				return n
			}(),
			secret: "12345678901234567890123456789012",
			expectedError: archive.ArchiveError{
				Filename: path.Join(os.TempDir(), "toglacier-test-noperm"),
				Code:     archive.ArchiveErrorCodeOpeningFile,
				Err: &os.PathError{
					Op:   "open",
					Path: path.Join(os.TempDir(), "toglacier-test-noperm"),
					Err:  errors.New("permission denied"),
				},
			},
		},
		{
			description: "it should ignore an unencrypted data even if the secret is defined",
			secret:      "12345678901234567890123456789012",
			encryptedFilename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			expectedFile: "Important information for the test backup",
		},
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-test-")
			if err != nil {
				t.Fatalf("error creating file. details: %s", err)
			}
			defer f.Close()

			content, err := hex.DecodeString("656e637279707465643a8fbd41664a1d72b4ea1fcecd618a6ed5c05c95aaa5bfda2d4d176e8feff96f710000000000000000000000000000000091d8e827b5136dfac6bb3dbc51f15c17d34947880f91e62799910ea05053969abc28033550b3781111")
			if err != nil {
				t.Fatalf("error decoding encrypted archive. details: %s", err)
			}

			f.Write(content)

			var scenario scenario
			scenario.description = "it should detect when the backup decrypt key has an invalid AES length"
			scenario.encryptedFilename = f.Name()
			scenario.secret = "123456"

			scenario.expectedError = archive.ArchiveError{
				Filename: f.Name(),
				Code:     archive.ArchiveErrorCodeInitCipher,
				Err:      aes.KeySizeError(6),
			}

			return scenario
		}(),
		{
			description: "it should detect when the decrypt authentication data is invalid",
			secret:      "1234567890123456",
			encryptedFilename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				content, err := hex.DecodeString("656e637279707465643a8fbd41664a1d72b4ea1fcecd618a6ed5c05c95aaa5bfda2d4d176e8feff96f710000000000000000000000000000000091d8e827b5136dfac6bb3dbc51f15c17d34947880f91e62799910ea05053969abc28033550b3781111")
				if err != nil {
					t.Fatalf("error decoding encrypted archive. details: %s", err)
				}

				f.Write(content)
				return f.Name()
			}(),
			expectedError: archive.ArchiveError{
				Code: archive.ArchiveErrorCodeAuthFailed,
			},
		},
	}

	originalRandomSource := archive.RandomSource
	defer func() {
		archive.RandomSource = originalRandomSource
	}()

	archive.RandomSource = mockReader{
		mockRead: func(p []byte) (n int, err error) {
			for i := range p {
				p[i] = 0
			}
			return len(p), nil
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			filename, err := archive.Decrypt(scenario.encryptedFilename, scenario.secret)

			fileContent, fileErr := ioutil.ReadFile(filename)
			if fileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading file. details: %s", fileErr)
			}

			if !reflect.DeepEqual(scenario.expectedFile, string(fileContent)) {
				t.Errorf("files don't match. expected “%s” and got “%s”", scenario.expectedFile, string(fileContent))
			}

			if !archive.ArchiveErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func Test_EncryptDecrypt(t *testing.T) {
	scenarios := []struct {
		description          string
		filename             string
		secret               string
		expectedFile         string
		expectedEncryptError error
		expectedDecryptError error
	}{
		{
			description: "it should encrypt and decrypt the archive correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			secret:       "12345678901234567890123456789012",
			expectedFile: `Important information for the test backup`,
		},
	}

	originalRandomSource := archive.RandomSource
	defer func() {
		archive.RandomSource = originalRandomSource
	}()
	archive.RandomSource = mockReader{
		mockRead: func(p []byte) (n int, err error) {
			for i := range p {
				p[i] = 0
			}
			return len(p), nil
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			encryptedFilename, err := archive.Encrypt(scenario.filename, scenario.secret)
			if !reflect.DeepEqual(scenario.expectedEncryptError, err) {
				t.Fatalf("errors don't match. expected “%v” and got “%v”", scenario.expectedEncryptError, err)
			}

			if scenario.expectedEncryptError != nil {
				return
			}

			filename, err := archive.Decrypt(encryptedFilename, scenario.secret)
			if !reflect.DeepEqual(scenario.expectedDecryptError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedDecryptError, err)
			}

			fileContent, fileErr := ioutil.ReadFile(filename)
			if fileErr != nil && scenario.expectedDecryptError == nil {
				t.Errorf("error reading file. details: %s", fileErr)
			}

			if !reflect.DeepEqual(scenario.expectedFile, string(fileContent)) {
				t.Errorf("files don't match. expected “%s” and got “%s”", scenario.expectedFile, string(fileContent))
			}

			if !reflect.DeepEqual(scenario.expectedDecryptError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedDecryptError, err)
			}
		})
	}
}

type mockReader struct {
	mockRead func(p []byte) (n int, err error)
}

func (m mockReader) Read(p []byte) (int, error) {
	return m.mockRead(p)
}
