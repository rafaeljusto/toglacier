package archive

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// TARBuilder join all paths into an archive using the TAR computer software
// utility.
type TARBuilder struct {
}

// NewTARBuilder returns a TARBuilder with all necessary initializations.
func NewTARBuilder() *TARBuilder {
	return &TARBuilder{}
}

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
func (t TARBuilder) Build(backupPaths ...string) (string, error) {
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

		if header.Typeflag != tar.TypeReg {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeOpeningFile, err))
		}
		defer file.Close()

		_, err = io.CopyN(tarArchive, file, info.Size())
		if err != nil && err != io.EOF {
			return errors.WithStack(newPathError(path, PathErrorCodeWritingFile, err))
		}

		return nil
	})
}
