package archive

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
)

// TARInfoFilename name of the file that is added to the tarball with the
// necessary information for an incremental archive.
var TARInfoFilename = "toglacier-info.json"

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
// backup. A control file is added to the tarball root so we can control
// incremental archives (send only what was modified). On success it will return
// an open file, so the caller is responsible for closing it. On error it will
// return an Error or PathError type encapsulated in a traceable error. To
// retrieve the desired error you can do:
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
func (t TARBuilder) Build(lastArchiveInfo Info, backupPaths ...string) (string, Info, error) {
	t.logger.Debugf("archive: build tar for backup paths %v", backupPaths)

	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", nil, errors.WithStack(newError("", ErrorCodeTARCreation, err))
	}
	defer tarFile.Close()

	tarArchive := tar.NewWriter(tarFile)
	basePath := "backup-" + time.Now().Format("20060102150405")

	archiveInfo := make(Info)
	for _, path := range backupPaths {
		if path == "" {
			t.logger.Info("archive: empty backup path ignored")
			continue
		}

		t.logger.Debugf("archive: analyzing backup path “%s”", path)

		tmpArchiveInfo, err := t.build(lastArchiveInfo, tarArchive, basePath, path)
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
		archiveInfo.Merge(tmpArchiveInfo)
	}

	if err := t.addInfo(archiveInfo, tarArchive, basePath); err != nil {
		return "", nil, errors.WithStack(err)
	}

	if err := tarArchive.Close(); err != nil {
		return "", nil, errors.WithStack(newError(tarFile.Name(), ErrorCodeTARGeneration, err))
	}

	archiveInfo.MergeLast(lastArchiveInfo)
	t.logger.Infof("archive: tar file “%s” created successfully", tarFile.Name())
	return tarFile.Name(), archiveInfo, nil
}

func (t TARBuilder) build(lastArchiveInfo Info, tarArchive *tar.Writer, baseDir, source string) (Info, error) {
	archiveInfo := make(Info)

	// as we have multiple backup paths in the same tarball, we need to create a
	// difference in the tree directory inside it. For that we are using the name
	// of the backup path
	pathName := filepath.Base(source)

	walkErr := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeInfo, err))
		}

		t.logger.Debugf("archive: walking into path “%s”", path)

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return errors.WithStack(newPathError(path, PathErrorCodeCreateTARHeader, err))
		}

		// we only accept regular files and directories
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			t.logger.Debugf("archive: path “%s”, with type “%d”, is not going to be added to the tar", path, header.Typeflag)
			return nil
		}

		if path == source && !info.IsDir() {
			// when we are building an archive of a single file, we cannot remove the
			// source path
			header.Name = filepath.Join(baseDir, pathName, filepath.Base(path))

		} else {
			header.Name = filepath.Join(baseDir, pathName, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			// tar always use slash as a path separator, even on Windows
			header.Name += "/"

			t.logger.Debugf("archive: writing tar header for directory “%s”", header.Name)

			if err = tarArchive.WriteHeader(header); err != nil {
				return errors.WithStack(newPathError(path, PathErrorCodeWritingTARHeader, err))
			}

			return nil
		}

		itemInfo, add, err := t.generateItemInfo(path, lastArchiveInfo)
		if err != nil {
			return errors.WithStack(err)
		}
		archiveInfo[path] = itemInfo

		if !add {
			t.logger.Debugf("archive: path “%s” ignored", path)
			return nil
		}

		return errors.WithStack(t.writeTarball(path, info, header, tarArchive))
	})

	return archiveInfo, errors.WithStack(walkErr)
}

func (t TARBuilder) generateItemInfo(path string, lastArchiveInfo Info) (itemInfo ItemInfo, add bool, err error) {
	encodedHash, err := t.fileHash(path)
	if err != nil {
		return itemInfo, true, errors.WithStack(err)
	}

	var ok bool
	itemInfo, ok = lastArchiveInfo[path]

	if !ok {
		add = true
		itemInfo.Status = ItemInfoStatusNew
		itemInfo.Hash = encodedHash
		t.logger.Debugf("archive: path “%s” is new since the last archive", path)

	} else if encodedHash == itemInfo.Hash {
		add = false // don't need to add an unmodified file to the tarball
		itemInfo.Status = ItemInfoStatusUnmodified
		t.logger.Debugf("archive: path “%s” unmodified since the last archive", path)

	} else {
		add = true
		itemInfo.ID = ""
		itemInfo.Status = ItemInfoStatusModified
		itemInfo.Hash = encodedHash
		t.logger.Debugf("archive: path “%s” was modified since the last archive", path)
	}

	return
}

func (t TARBuilder) fileHash(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", errors.WithStack(newPathError(filename, PathErrorCodeOpeningFile, err))
	}
	defer file.Close()

	hash := sha256.New()

	written, err := io.Copy(hash, file)
	if err != nil {
		return "", errors.WithStack(newPathError(filename, PathErrorCodeSHA256, err))
	}

	encodedHash := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	t.logger.Debugf("archive: path “%s” hash calculated over %d bytes: %s", filename, written, encodedHash)
	return encodedHash, nil
}

func (t TARBuilder) addInfo(archiveInfo Info, tarArchive *tar.Writer, baseDir string) error {
	content, err := json.Marshal(archiveInfo)
	if err != nil {
		return newError("", ErrorCodeEncodingInfo, err)
	}

	file, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return errors.WithStack(newError("", ErrorCodeTmpFileCreation, err))
	}
	defer file.Close()

	n, err := file.Write(content)
	if err != nil {
		return errors.WithStack(newPathError("", PathErrorCodeWritingFile, err))
	}

	t.logger.Debugf("archive: wrote %d bytes in archive information file “%s”", n, file.Name())

	info, err := file.Stat()
	if err != nil {
		return errors.WithStack(newPathError(file.Name(), PathErrorCodeInfo, err))
	}

	header, err := tar.FileInfoHeader(info, file.Name())
	if err != nil {
		return errors.WithStack(newPathError(file.Name(), PathErrorCodeCreateTARHeader, err))
	}
	header.Name = filepath.Join(baseDir, TARInfoFilename)

	return errors.WithStack(t.writeTarball(file.Name(), info, header, tarArchive))
}

func (t TARBuilder) writeTarball(path string, info os.FileInfo, header *tar.Header, tarArchive *tar.Writer) error {
	t.logger.Debugf("archive: writing tar header “%s”", header.Name)

	if err := tarArchive.WriteHeader(header); err != nil {
		return errors.WithStack(newPathError(path, PathErrorCodeWritingTARHeader, err))
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
}
