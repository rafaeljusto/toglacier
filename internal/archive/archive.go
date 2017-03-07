// Package archive builds the backup archive.
package archive

import (
	"archive/tar"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
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
// for closing it.
func Build(backupPaths ...string) (string, error) {
	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", fmt.Errorf("error creating the tar file. details: %s", err)
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
		return "", fmt.Errorf("error generating tar file. details: %s", err)
	}

	return tarFile.Name(), nil
}

func build(tarArchive *tar.Writer, baseDir, source string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error retrieving path “%s” information. details: %s", path, err)
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("error creating tar header for path “%s”. details: %s", path, err)
		}

		if path == source && !info.IsDir() {
			// when we are building an archive of a single file, we don't need to
			// create a base directory
			header.Name = filepath.Base(path)

		} else {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += string(os.PathSeparator)
		}

		if err = tarArchive.WriteHeader(header); err != nil {
			return fmt.Errorf("error writing header in tar for file %s. details: %s", path, err)
		}

		if info.IsDir() {
			return nil
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("error opening file %s. details: %s", path, err)
			}
			defer file.Close()

			_, err = io.CopyN(tarArchive, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("error writing content in tar for file %s. details: %s", path, err)
			}
		}

		return nil
	})
}

// Encrypt do what we expect, encrypting the content with a shared secret. It
// adds authentication using HMAC-SHA256. It will return the encrypted
// filename or an error.
func Encrypt(filename, secret string) (string, error) {
	archive, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("error opening file %s. details: %s", filename, err)
	}
	defer archive.Close()

	encryptedArchive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", fmt.Errorf("error creating temporary encrypted file. details: %s", err)
	}
	defer encryptedArchive.Close()

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", fmt.Errorf("error calculating HMAC-SHA256 from file %s. details: %s", filename, err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = io.ReadFull(RandomSource, iv); err != nil {
		return "", fmt.Errorf("error filling iv with random numbers. details: %s", err)
	}

	if _, err = encryptedArchive.WriteString(encryptedLabel); err != nil {
		return "", fmt.Errorf("error writing label to encrypted file. details: %s", err)
	}

	if _, err = encryptedArchive.Write(hash); err != nil {
		return "", fmt.Errorf("error writing authentication to encrypted file. details: %s", err)
	}

	if _, err = encryptedArchive.Write(iv); err != nil {
		return "", fmt.Errorf("error writing iv to encrypted file. details: %s", err)
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("error initializing cipher. details: %s", err)
	}

	writer := cipher.StreamWriter{
		S: cipher.NewOFB(block, iv),
		W: encryptedArchive,
	}
	defer writer.Close()

	if _, err = io.Copy(&writer, archive); err != nil {
		return "", fmt.Errorf("error encrypting file. details: %s", err)
	}

	return encryptedArchive.Name(), nil
}

// Decrypt do what we expect, decrypting the content with a shared secret. It
// authenticates the data using HMAC-SHA256. It will return the decrypted
// filename or an error.
func Decrypt(encryptedFilename, secret string) (string, error) {
	encryptedArchive, err := os.Open(encryptedFilename)
	if err != nil {
		return "", fmt.Errorf("error opening encrypted file. details: %s", err)
	}
	defer encryptedArchive.Close()

	archive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", fmt.Errorf("error creating temporary file. details: %s", err)
	}
	defer archive.Close()

	encryptedLabelBuffer := make([]byte, len(encryptedLabel))
	if _, err = encryptedArchive.Read(encryptedLabelBuffer); err == io.EOF || string(encryptedLabelBuffer) != encryptedLabel {
		// if we couldn't read the encrypted label, maybe the file isn't encrypted,
		// so let's return it as it is
		return encryptedFilename, nil

	} else if err != nil {
		return "", fmt.Errorf("error reading encrypted file label. details: %s", err)
	}

	authHash := make([]byte, sha256.Size)
	if _, err = encryptedArchive.Read(authHash); err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = encryptedArchive.Read(iv); err != nil {
		return "", fmt.Errorf("error reading encrypted authentication. details: %s", err)
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("error initializing cipher. details: %s", err)
	}

	reader := cipher.StreamReader{
		S: cipher.NewOFB(block, iv),
		R: encryptedArchive,
	}

	if _, err = io.Copy(archive, reader); err != nil {
		return "", fmt.Errorf("error decrypting file. details: %s", err)
	}

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", fmt.Errorf("error calculating HMAC-SHA256 from file. details: %s", err)
	}

	if !hmac.Equal(authHash, hash) {
		return "", errors.New("encrypted content authentication failed")
	}

	return archive.Name(), nil
}

func hmacSHA256(f *os.File, secret string) ([]byte, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	hash := hmac.New(sha256.New, []byte(secret))
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}
