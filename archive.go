package main

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
)

func buildArchive(backupPath string) (*os.File, error) {
	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return nil, fmt.Errorf("error creating the tar file. details: %s", err)
	}

	tarArchive := tar.NewWriter(tarFile)

	if err := buildArchiveLevels(tarArchive, backupPath); err != nil {
		tarFile.Close()
		return nil, err
	}

	if err := tarArchive.Close(); err != nil {
		tarFile.Close()
		return nil, fmt.Errorf("error generating tar file. details: %s", err)
	}

	return tarFile, nil
}

func buildArchiveLevels(tarArchive *tar.Writer, pathLevel string) error {
	files, err := ioutil.ReadDir(pathLevel)
	if err != nil {
		return fmt.Errorf("error reading path “%s”. details: %s", pathLevel, err)
	}

	for _, file := range files {
		if file.IsDir() {
			buildArchiveLevels(tarArchive, path.Join(pathLevel, file.Name()))
			continue
		}

		tarHeader := tar.Header{
			Name: file.Name(),
			Mode: 0600,
			Size: file.Size(),
		}

		if err := tarArchive.WriteHeader(&tarHeader); err != nil {
			return fmt.Errorf("error writing header in tar for file %s. details: %s", file.Name(), err)
		}

		filename := path.Join(pathLevel, file.Name())

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
