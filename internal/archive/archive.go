package archive

import (
	"archive/tar"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// RandomSource defines from where we are going to read random values to encrypt
// the archives.
var RandomSource = rand.Reader

// encryptedLabel is used to identify if an archive was encrypted or not.
const encryptedLabel = "encrypted:"

// Build builds a tarball containing all the desired files that you want to
// backup. On success it will return an open file, so the caller is responsible
// for closing it. On error it will return an Error or PathError type
// encapsulated in a traceable error. To retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *archive.Error:
//         // handle specifically
//       case *archive.PathError:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func Build(backupPaths ...string) (string, error) {
	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", errors.WithStack(newError("", ErrorCodeTARCreation, err))
	}
	defer tarFile.Close()

	tarArchive := tar.NewWriter(tarFile)
	basePath := "backup-" + time.Now().Format("20060102150405")

	for _, path := range backupPaths {
		if path == "" {
			continue
		}

		if err := build(tarArchive, basePath, path); err != nil {
			return "", errors.WithStack(err)
		}
	}

	if err := tarArchive.Close(); err != nil {
		return "", errors.WithStack(newError(tarFile.Name(), ErrorCodeTARGeneration, err))
	}

	return tarFile.Name(), nil
}

func build(tarArchive *tar.Writer, baseDir, source string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeInfo, err))
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeCreateTARHeader, err))
		}

		if path == source && !info.IsDir() {
			// when we are building an archive of a single file, we don't need to
			// create a base directory
			header.Name = filepath.Base(path)

		} else {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			// tar always use slash as a path separator, even on Windows
			header.Name += "/"
		}

		if err = tarArchive.WriteHeader(header); err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeWritingTARHeader, err))
		}

		if info.IsDir() {
			return nil
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return errors.WithStack(newPathError(path, PathErrorCodeOpeningFile, err))
			}
			defer file.Close()

			_, err = io.CopyN(tarArchive, file, info.Size())
			if err != nil && err != io.EOF {
				return errors.WithStack(newPathError(path, PathErrorCodeWritingFile, err))
			}
		}

		return nil
	})
}

// Encrypt do what we expect, encrypting the content with a shared secret. It
// adds authentication using HMAC-SHA256. It will return the encrypted
// filename or an Error type encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *archive.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func Encrypt(filename, secret string) (string, error) {
	archive, err := os.Open(filename)
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeOpeningFile, err))
	}
	defer archive.Close()

	encryptedArchive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeTmpFileCreation, err))
	}
	defer encryptedArchive.Close()

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", errors.WithStack(err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = io.ReadFull(RandomSource, iv); err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeGenerateRandomNumbers, err))
	}

	if _, err = encryptedArchive.WriteString(encryptedLabel); err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeWritingLabel, err))
	}

	if _, err = encryptedArchive.Write(hash); err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeWritingAuth, err))
	}

	if _, err = encryptedArchive.Write(iv); err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeWritingIV, err))
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeInitCipher, err))
	}

	writer := cipher.StreamWriter{
		S: cipher.NewOFB(block, iv),
		W: encryptedArchive,
	}
	defer writer.Close()

	if _, err = io.Copy(&writer, archive); err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeEncryptingFile, err))
	}

	return encryptedArchive.Name(), nil
}

// Decrypt do what we expect, decrypting the content with a shared secret. It
// authenticates the data using HMAC-SHA256. It will return the decrypted
// filename or an Error type encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *archive.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func Decrypt(encryptedFilename, secret string) (string, error) {
	encryptedArchive, err := os.Open(encryptedFilename)
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeOpeningFile, err))
	}
	defer encryptedArchive.Close()

	archive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeTmpFileCreation, err))
	}
	defer archive.Close()

	encryptedLabelBuffer := make([]byte, len(encryptedLabel))
	if _, err = encryptedArchive.Read(encryptedLabelBuffer); err == io.EOF || string(encryptedLabelBuffer) != encryptedLabel {
		// if we couldn't read the encrypted label, maybe the file isn't encrypted,
		// so let's return it as it is
		return encryptedFilename, nil

	} else if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeReadingLabel, err))
	}

	authHash := make([]byte, sha256.Size)
	if _, err = encryptedArchive.Read(authHash); err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeReadingAuth, err))
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = encryptedArchive.Read(iv); err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeReadingIV, err))
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeInitCipher, err))
	}

	reader := cipher.StreamReader{
		S: cipher.NewOFB(block, iv),
		R: encryptedArchive,
	}

	if _, err = io.Copy(archive, reader); err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeDecryptingFile, err))
	}

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if !hmac.Equal(authHash, hash) {
		return "", errors.WithStack(newError("", ErrorCodeAuthFailed, nil))
	}

	return archive.Name(), nil
}

func hmacSHA256(f *os.File, secret string) ([]byte, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return nil, errors.WithStack(newError(f.Name(), ErrorCodeRewindingFile, err))
	}

	hash := hmac.New(sha256.New, []byte(secret))
	if _, err := io.Copy(hash, f); err != nil {
		return nil, errors.WithStack(newError(f.Name(), ErrorCodeCalculateHMACSHA256, err))
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, errors.WithStack(newError(f.Name(), ErrorCodeRewindingFile, err))
	}

	return hash.Sum(nil), nil
}
