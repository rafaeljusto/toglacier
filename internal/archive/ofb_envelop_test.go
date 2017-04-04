package archive_test

import (
	"crypto/aes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/archive"
)

func TestOFBEnvelop_Encrypt(t *testing.T) {
	type scenario struct {
		description   string
		envelop       archive.OFBEnvelop
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
			expectedError: &archive.Error{
				Filename: "toglacier-idontexist.tmp",
				Code:     archive.ErrorCodeOpeningFile,
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
			expectedError: &archive.Error{
				Filename: path.Join(os.TempDir(), "toglacier-test-noperm"),
				Code:     archive.ErrorCodeOpeningFile,
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

			var s scenario
			s.description = "it should detect when the random source generates an error"
			s.filename = f.Name()
			s.secret = "1234567890123456"
			s.randomSource = mockReader{
				mockRead: func(p []byte) (n int, err error) {
					return 0, errors.New("random error")
				},
			}
			s.expectedError = &archive.Error{
				Filename: f.Name(),
				Code:     archive.ErrorCodeGenerateRandomNumbers,
				Err:      errors.New("random error"),
			}

			return s
		}(),
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-test-")
			if err != nil {
				t.Fatalf("error creating file. details: %s", err)
			}
			defer f.Close()

			f.WriteString("Important information for the test backup")

			var s scenario
			s.description = "it should detect when the AES secret length is invalid"
			s.filename = f.Name()
			s.secret = "123456"
			s.randomSource = rand.Reader
			s.expectedError = &archive.Error{
				Filename: f.Name(),
				Code:     archive.ErrorCodeInitCipher,
				Err:      aes.KeySizeError(6),
			}

			return s
		}(),
	}

	originalRandomSource := archive.RandomSource
	defer func() {
		archive.RandomSource = originalRandomSource
	}()

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			archive.RandomSource = scenario.randomSource
			encryptedFilename, err := scenario.envelop.Encrypt(scenario.filename, scenario.secret)

			fileContent, fileErr := ioutil.ReadFile(encryptedFilename)
			if fileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading file. details: %s", fileErr)
			}

			if !reflect.DeepEqual(scenario.expectedFile, string(fileContent)) {
				t.Errorf("files don't match. expected “%s” and got “%s”", scenario.expectedFile, string(fileContent))
			}

			if !archive.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestOFBEnvelop_Decrypt(t *testing.T) {
	type scenario struct {
		description       string
		envelop           archive.OFBEnvelop
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
			expectedError: &archive.Error{
				Filename: path.Join(os.TempDir(), "toglacier-test-noperm"),
				Code:     archive.ErrorCodeOpeningFile,
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

			var s scenario
			s.description = "it should detect when the backup decrypt key has an invalid AES length"
			s.encryptedFilename = f.Name()
			s.secret = "123456"
			s.expectedError = &archive.Error{
				Filename: f.Name(),
				Code:     archive.ErrorCodeInitCipher,
				Err:      aes.KeySizeError(6),
			}

			return s
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
			expectedError: &archive.Error{
				Code: archive.ErrorCodeAuthFailed,
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
			filename, err := scenario.envelop.Decrypt(scenario.encryptedFilename, scenario.secret)

			fileContent, fileErr := ioutil.ReadFile(filename)
			if fileErr != nil && scenario.expectedError == nil {
				t.Errorf("error reading file. details: %s", fileErr)
			}

			if !reflect.DeepEqual(scenario.expectedFile, string(fileContent)) {
				t.Errorf("files don't match. expected “%s” and got “%s”", scenario.expectedFile, string(fileContent))
			}

			if !archive.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestOFBEnvelop_EncryptDecrypt(t *testing.T) {
	scenarios := []struct {
		description          string
		envelop              archive.OFBEnvelop
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
			encryptedFilename, err := scenario.envelop.Encrypt(scenario.filename, scenario.secret)
			if !reflect.DeepEqual(scenario.expectedEncryptError, err) {
				t.Fatalf("errors don't match. expected “%v” and got “%v”", scenario.expectedEncryptError, err)
			}

			if scenario.expectedEncryptError != nil {
				return
			}

			filename, err := scenario.envelop.Decrypt(encryptedFilename, scenario.secret)
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
