package main

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

// buildArchive builds a tarball containing all the desired files that you want
// to backup. On success it will return an open file, so the caller is
// responsible for closing it.
func buildArchive(backupPaths string) (*os.File, error) {
	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return nil, fmt.Errorf("error creating the tar file. details: %s", err)
	}

	tarArchive := tar.NewWriter(tarFile)
	basePath := "backup-" + time.Now().Format("20060102150405")

	for _, currentPath := range strings.Split(backupPaths, ",") {
		if currentPath == "" {
			continue
		}

		if err := buildArchiveLevels(tarArchive, basePath, currentPath); err != nil {
			tarFile.Close()
			return nil, err
		}
	}

	if err := tarArchive.Close(); err != nil {
		tarFile.Close()
		return nil, fmt.Errorf("error generating tar file. details: %s", err)
	}

	return tarFile, nil
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
			return fmt.Errorf("wrong number of bytes writen in file %s", filename)
		}

		if err := fd.Close(); err != nil {
			return fmt.Errorf("error closing file %s. details: %s", filename, err)
		}
	}

	return nil
}
