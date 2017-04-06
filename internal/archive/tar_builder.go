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
	"github.com/rafaeljusto/toglacier/internal/log"
)

// TARBuilder join all paths into an archive using the TAR computer software
// utility.
type TARBuilder struct {
	logger log.Logger
}

// NewTARBuilder returns a TARBuilder with all necessary initializations.
func NewTARBuilder(logger log.Logger) *TARBuilder {
	return &TARBuilder{
		logger: logger,
	}
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
	t.logger.Debugf("archive: build tar for backup paths %v", backupPaths)

	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", errors.WithStack(newError("", ErrorCodeTARCreation, err))
	}
	defer tarFile.Close()

	tarArchive := tar.NewWriter(tarFile)
	basePath := "backup-" + time.Now().Format("20060102150405")

	for _, path := range backupPaths {
		if path == "" {
			t.logger.Info("archive: empty backup path ignored")
			continue
		}

		t.logger.Debugf("archive: analyzing backup path “%s”", path)
		if err := t.build(tarArchive, basePath, path); err != nil {
			return "", errors.WithStack(err)
		}
	}

	if err := tarArchive.Close(); err != nil {
		return "", errors.WithStack(newError(tarFile.Name(), ErrorCodeTARGeneration, err))
	}

	t.logger.Infof("archive: tar file “%s” created successfully", tarFile.Name())
	return tarFile.Name(), nil
}

func (t TARBuilder) build(tarArchive *tar.Writer, baseDir, source string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeInfo, err))
		}

		t.logger.Debugf("archive: walking into path “%s”", path)

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

		t.logger.Debugf("archive: writing tar header “%s”", header.Name)

		if err = tarArchive.WriteHeader(header); err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeWritingTARHeader, err))
		}

		if info.IsDir() {
			t.logger.Debugf("archive: path “%s” is a directory", path)
			return nil
		}

		if header.Typeflag != tar.TypeReg {
			t.logger.Debugf("archive: path “%s”, with type “%d”, is not going to be added to the tar", path, header.Typeflag)
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeOpeningFile, err))
		}
		defer file.Close()

		written, err := io.CopyN(tarArchive, file, info.Size())
		if err != nil && err != io.EOF {
			return errors.WithStack(newPathError(path, PathErrorCodeWritingFile, err))
		}

		t.logger.Debugf("archive: path “%s” copied to tar (%d bytes)", path, written)
		return nil
	})
}
