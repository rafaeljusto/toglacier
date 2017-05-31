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

// extractDirectoryPermission defines the permission mode for the directories
// created while extracting a tarball.
const extractDirectoryPermission os.FileMode = 0755

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
// an open file, so the caller is responsible for closing it. If no file was
// written to the tarball, an empty filename is returned. On error it will
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
	hasFiles := false
	for _, path := range backupPaths {
		if path == "" {
			t.logger.Info("archive: empty backup path ignored")
			continue
		}

		t.logger.Debugf("archive: analyzing backup path “%s”", path)

		tmpArchiveInfo, tmpHasFiles, err := t.build(lastArchiveInfo, tarArchive, basePath, path)
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
		archiveInfo.Merge(tmpArchiveInfo)

		if tmpHasFiles {
			hasFiles = true
		}
	}

	// if there're no files in the tar there's no reason to create this backup
	if hasFiles {
		archiveInfo.MergeLast(lastArchiveInfo)
		if err := t.addInfo(archiveInfo, tarArchive, basePath); err != nil {
			return "", nil, errors.WithStack(err)
		}

		statistic := archiveInfo.Statistics()
		t.logger.Infof("archive: %d new files; %d modified files; %d unmodified files; %d deleted files",
			statistic[ItemInfoStatusNew],
			statistic[ItemInfoStatusModified],
			statistic[ItemInfoStatusUnmodified],
			statistic[ItemInfoStatusDeleted],
		)
	}

	if err := tarArchive.Close(); err != nil {
		return "", nil, errors.WithStack(newError(tarFile.Name(), ErrorCodeTARGeneration, err))
	}

	if !hasFiles {
		// force fd close to remove the empty tarball.
		tarFile.Close()
		os.Remove(tarFile.Name())

		t.logger.Info("archive: tar file not created because no files were added")
		return "", nil, nil
	}

	t.logger.Infof("archive: tar file “%s” created successfully", tarFile.Name())
	return tarFile.Name(), archiveInfo, nil
}

func (t TARBuilder) build(lastArchiveInfo Info, tarArchive *tar.Writer, baseDir, source string) (archiveInfo Info, hasFiles bool, err error) {
	var directories []*tar.Header
	archiveInfo = make(Info)

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
			t.logger.Infof("archive: path “%s”, with type “%d”, is not going to be added to the tar", path, header.Typeflag)
			return nil
		}

		// store the full path in the tarball to avoid conflicts when appending
		// multiple backup paths
		header.Name = filepath.Join(baseDir, path)

		if info.IsDir() {
			// tar always use slash as a path separator, even on Windows
			header.Name += "/"

			// forward directory creation to when a file is written
			directories = append(directories, header)
			return nil
		}

		itemInfo, add, err := t.generateItemInfo(path, lastArchiveInfo)
		if err != nil {
			return errors.WithStack(err)
		}
		archiveInfo[path] = itemInfo

		if !add {
			// TODO: if the file is ignored, we should check the directories slice to
			// remove unnecessary entries
			t.logger.Debugf("archive: path “%s” ignored", path)
			return nil
		}

		hasFiles = true

		// only write directory (FIFO order) when we are sure that a file will be
		// written to the tarball. Otherwise we could have a tarball with empty
		// directories
		for _, directory := range directories {
			t.logger.Debugf("archive: writing tar header for directory “%s”", directory.Name)

			if err = tarArchive.WriteHeader(directory); err != nil {
				return errors.WithStack(newPathError(path, PathErrorCodeWritingTARHeader, err))
			}
		}

		// after the directories are created, we can clear the slice for the next
		// round
		directories = nil

		return errors.WithStack(t.writeTarball(path, info, header, tarArchive))
	})

	return archiveInfo, hasFiles, errors.WithStack(walkErr)
}

func (t TARBuilder) generateItemInfo(path string, lastArchiveInfo Info) (itemInfo ItemInfo, add bool, err error) {
	encodedChecksum, err := t.FileChecksum(path)
	if err != nil {
		return itemInfo, true, errors.WithStack(err)
	}

	var ok bool
	itemInfo, ok = lastArchiveInfo[path]

	if !ok {
		add = true
		itemInfo.Status = ItemInfoStatusNew
		itemInfo.Checksum = encodedChecksum
		t.logger.Debugf("archive: path “%s” is new since the last archive", path)

	} else if encodedChecksum == itemInfo.Checksum {
		add = false // don't need to add an unmodified file to the tarball
		itemInfo.Status = ItemInfoStatusUnmodified
		t.logger.Debugf("archive: path “%s” unmodified since the last archive", path)

	} else {
		add = true
		itemInfo.ID = ""
		itemInfo.Status = ItemInfoStatusModified
		itemInfo.Checksum = encodedChecksum
		t.logger.Debugf("archive: path “%s” was modified since the last archive", path)
	}

	return
}

// FileChecksum returns the file SHA256 hash encoded in base64. On error it will
// return a PathError type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *archive.PathError:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (t TARBuilder) FileChecksum(filename string) (string, error) {
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

	encodedChecksum := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	t.logger.Debugf("archive: path “%s” hash calculated over %d bytes: %s", filename, written, encodedChecksum)
	return encodedChecksum, nil
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

// Extract uncompress all files from the tarball to the current path. You can
// select the files that are extracted with the filter parameter, if nil all
// files are extracted. On error it will return an Error type encapsulated in a
// traceable error. To retrieve the desired error you can do:
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
func (t TARBuilder) Extract(filename string, filter []string) (Info, error) {
	t.logger.Debugf("archive: extract tar %s", filename)

	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.WithStack(newError(filename, ErrorCodeOpeningFile, err))
	}
	defer f.Close()

	tarReader := tar.NewReader(f)
	var info Info

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.WithStack(newError(filename, ErrorCodeReadingTAR, err))
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// we will create the directories only when extracting files from the
			// tarball, because not all files will be extracted

		case tar.TypeReg:
			name := normalizeHeaderName(header.Name)

			if name == TARInfoFilename {
				decoder := json.NewDecoder(tarReader)
				if err := decoder.Decode(&info); err != nil {
					return nil, errors.WithStack(newError(filename, ErrorCodeDecodingInfo, err))
				}
				continue
			}

			if filter != nil && !shouldExtract(name, filter) {
				t.logger.Debugf("archive: ignoring extraction of path “%s”", header.Name)
				continue
			}

			dir := filepath.Dir(header.Name)
			if err := os.MkdirAll(dir, extractDirectoryPermission); err != nil {
				return nil, errors.WithStack(newError(filename, ErrorCodeCreatingDirectories, err))
			}

			tarFile, err := os.OpenFile(header.Name, os.O_WRONLY|os.O_CREATE, os.FileMode(header.Mode))
			if err != nil {
				return nil, errors.WithStack(newError(header.Name, ErrorCodeOpeningFile, err))
			}

			written, err := io.Copy(tarFile, tarReader)
			tarFile.Close()

			if err != nil {
				return nil, errors.WithStack(newError(tarFile.Name(), ErrorCodeExtractingFile, err))
			}

			t.logger.Debugf("archive: path “%s” extracted from tar (%d bytes)", tarFile.Name(), written)

		default:
			t.logger.Infof("archive: path “%s”, with type “%d”, is not going to be extracted from the tar", header.Name, header.Typeflag)
		}
	}

	return info, nil
}

// normalizeHeaderName normalize the header name for comparing the tarball file
// with the filter, we need to retrieve the original file path, removing the
// backup directory in the beginning. Tarball path before:
//
//     backup-20170506120000/dir1/dir2/file
//
// and after the magic:
//
//     /dir1/dir2/file
func normalizeHeaderName(name string) string {
	nameParts := strings.Split(name, string(os.PathSeparator))
	if len(nameParts) == 0 {
		return name
	}

	name = strings.Join(nameParts[1:], string(os.PathSeparator))

	// if there's more than one directory level we need to add the root,
	// otherwise is just a simple filename
	if strings.Count(name, string(os.PathSeparator)) > 0 {
		name = string(os.PathSeparator) + name
	}

	return name
}

func shouldExtract(name string, filter []string) bool {
	for _, item := range filter {
		if name == item {
			return true
		}
	}

	return false
}
