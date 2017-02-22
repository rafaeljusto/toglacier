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
	"path"
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

	for _, currentPath := range backupPaths {
		if currentPath == "" {
			continue
		}

		if err := buildArchiveLevels(tarArchive, basePath, currentPath); err != nil {
			return "", err
		}
	}

	if err := tarArchive.Close(); err != nil {
		return "", fmt.Errorf("error generating tar file. details: %s", err)
	}

	return tarFile.Name(), nil
}

func buildArchiveLevels(tarArchive *tar.Writer, basePath, currentPath string) error {
	files, err := ioutil.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("error reading path “%s”. details: %s", currentPath, err)
	}

	for _, file := range files {
		if file.IsDir() {
			buildArchiveLevels(tarArchive, basePath, path.Join(currentPath, file.Name()))
			continue
		}

		tarHeader := tar.Header{
			Name:    path.Join(basePath, currentPath, file.Name()),
			Mode:    0600,
			Size:    file.Size(),
			ModTime: file.ModTime(),
		}

		if err := tarArchive.WriteHeader(&tarHeader); err != nil {
			return fmt.Errorf("error writing header in tar for file %s. details: %s", file.Name(), err)
		}

		filename := path.Join(currentPath, file.Name())

		fd, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("error opening file %s. details: %s", filename, err)
		}

		if n, err := io.Copy(tarArchive, fd); err != nil {
			return fmt.Errorf("error writing content in tar for file %s. details: %s", filename, err)

		} else if n != file.Size() {
			return fmt.Errorf("wrong number of bytes written in file %s", filename)
		}

		if err := fd.Close(); err != nil {
			return fmt.Errorf("error closing file %s. details: %s", filename, err)
		}
	}

	return nil
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
