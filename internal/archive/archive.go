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
)

// RandomSource defines from where we are going to read random values to encrypt
// the archives.
var RandomSource = rand.Reader

// encryptedLabel is used to identify if an archive was encrypted or not.
const encryptedLabel = "encrypted:"

// Build builds a tarball containing all the desired files that you want to
// backup. On success it will return an open file, so the caller is responsible
// for closing it. On error it will return an ArchiveError or a PathError.
func Build(backupPaths ...string) (string, error) {
	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", newArchiveError("", ArchiveErrorCodeTARCreation, err)
	}
	defer tarFile.Close()

	tarArchive := tar.NewWriter(tarFile)
	basePath := "backup-" + time.Now().Format("20060102150405")

	for _, path := range backupPaths {
		if path == "" {
			continue
		}

		if err := build(tarArchive, basePath, path); err != nil {
			return "", err
		}
	}

	if err := tarArchive.Close(); err != nil {
		return "", newArchiveError(tarFile.Name(), ArchiveErrorCodeTARGeneration, err)
	}

	return tarFile.Name(), nil
}

func build(tarArchive *tar.Writer, baseDir, source string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return newPathError(path, PathErrorCodeInfo, err)
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return newPathError(path, PathErrorCodeCreateTARHeader, err)
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
			return newPathError(path, PathErrorCodeWritingTARHeader, err)
		}

		if info.IsDir() {
			return nil
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return newPathError(path, PathErrorCodeOpeningFile, err)
			}
			defer file.Close()

			_, err = io.CopyN(tarArchive, file, info.Size())
			if err != nil && err != io.EOF {
				return newPathError(path, PathErrorCodeWritingFile, err)
			}
		}

		return nil
	})
}

// Encrypt do what we expect, encrypting the content with a shared secret. It
// adds authentication using HMAC-SHA256. It will return the encrypted
// filename or an ArchiveError.
func Encrypt(filename, secret string) (string, error) {
	archive, err := os.Open(filename)
	if err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeOpeningFile, err)
	}
	defer archive.Close()

	encryptedArchive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeTmpFileCreation, err)
	}
	defer encryptedArchive.Close()

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = io.ReadFull(RandomSource, iv); err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeGenerateRandomNumbers, err)
	}

	if _, err = encryptedArchive.WriteString(encryptedLabel); err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeWritingLabel, err)
	}

	if _, err = encryptedArchive.Write(hash); err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeWritingAuth, err)
	}

	if _, err = encryptedArchive.Write(iv); err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeWritingIV, err)
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeInitCipher, err)
	}

	writer := cipher.StreamWriter{
		S: cipher.NewOFB(block, iv),
		W: encryptedArchive,
	}
	defer writer.Close()

	if _, err = io.Copy(&writer, archive); err != nil {
		return "", newArchiveError(filename, ArchiveErrorCodeEncryptingFile, err)
	}

	return encryptedArchive.Name(), nil
}

// Decrypt do what we expect, decrypting the content with a shared secret. It
// authenticates the data using HMAC-SHA256. It will return the decrypted
// filename or an ArchiveError.
func Decrypt(encryptedFilename, secret string) (string, error) {
	encryptedArchive, err := os.Open(encryptedFilename)
	if err != nil {
		return "", newArchiveError(encryptedFilename, ArchiveErrorCodeOpeningFile, err)
	}
	defer encryptedArchive.Close()

	archive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", newArchiveError(encryptedFilename, ArchiveErrorCodeTmpFileCreation, err)
	}
	defer archive.Close()

	encryptedLabelBuffer := make([]byte, len(encryptedLabel))
	if _, err = encryptedArchive.Read(encryptedLabelBuffer); err == io.EOF || string(encryptedLabelBuffer) != encryptedLabel {
		// if we couldn't read the encrypted label, maybe the file isn't encrypted,
		// so let's return it as it is
		return encryptedFilename, nil

	} else if err != nil {
		return "", newArchiveError(encryptedFilename, ArchiveErrorCodeReadingLabel, err)
	}

	authHash := make([]byte, sha256.Size)
	if _, err = encryptedArchive.Read(authHash); err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = encryptedArchive.Read(iv); err != nil {
		return "", newArchiveError(encryptedFilename, ArchiveErrorCodeReadingAuth, err)
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", newArchiveError(encryptedFilename, ArchiveErrorCodeInitCipher, err)
	}

	reader := cipher.StreamReader{
		S: cipher.NewOFB(block, iv),
		R: encryptedArchive,
	}

	if _, err = io.Copy(archive, reader); err != nil {
		return "", newArchiveError(encryptedFilename, ArchiveErrorCodeDecryptingFile, err)
	}

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", err
	}

	if !hmac.Equal(authHash, hash) {
		return "", newArchiveError("", ArchiveErrorCodeAuthFailed, nil)
	}

	return archive.Name(), nil
}

func hmacSHA256(f *os.File, secret string) ([]byte, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return nil, newArchiveError(f.Name(), ArchiveErrorCodeRewindingFile, err)
	}

	hash := hmac.New(sha256.New, []byte(secret))
	if _, err := io.Copy(hash, f); err != nil {
		return nil, newArchiveError(f.Name(), ArchiveErrorCodeCalculateHMACSHA256, err)
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, newArchiveError(f.Name(), ArchiveErrorCodeRewindingFile, err)
	}

	return hash.Sum(nil), nil
}
