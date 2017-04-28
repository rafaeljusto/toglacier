package storage

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
)

// AuditFile stores all backup information in a simple text file.
type AuditFile struct {
	logger   log.Logger
	Filename string
}

// NewAuditFile initializes a new AuditFile object.
func NewAuditFile(logger log.Logger, filename string) *AuditFile {
	return &AuditFile{
		logger:   logger,
		Filename: filename,
	}
}

// Save a backup information. It stores the backup information one per line with
// the following columns:
//
//     [datetime] [vaultName] [archiveID] [checksum] [size]
//
// The audit file doesn't store backup extra information. On error it will
// return an Error type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *storage.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AuditFile) Save(backup Backup) error {
	a.logger.Debugf("storage: saving backup “%s” in audit file storage", backup.Backup.ID)

	auditFile, err := os.OpenFile(a.Filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer auditFile.Close()

	audit := fmt.Sprintf("%s %s %s %s %d\n", backup.Backup.CreatedAt.Format(time.RFC3339), backup.Backup.VaultName, backup.Backup.ID, backup.Backup.Checksum, backup.Backup.Size)
	if _, err = auditFile.WriteString(audit); err != nil {
		return errors.WithStack(newError(ErrorCodeWritingFile, err))
	}

	a.logger.Infof("storage: backup “%s” saved successfully in audit file storage", backup.Backup.ID)
	return nil
}

// List all backup information in the storage. As the audit file doesn't store
// backup extra information, it will be always nil. The backups are ordered by
// creation date. On error it will return an Error type encapsulated in a
// traceable error. To retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *storage.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AuditFile) List() (Backups, error) {
	a.logger.Debug("storage: listing backups from audit file storage")

	auditFile, err := os.Open(a.Filename)
	if err != nil {
		// if the file doesn't exist we can presume that there's no backups yet
		if pathErr, ok := err.(*os.PathError); ok && os.IsNotExist(pathErr.Err) {
			return nil, nil
		}

		return nil, errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer auditFile.Close()

	var backups Backups

	scanner := bufio.NewScanner(auditFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineParts := strings.Split(line, " ")

		if len(lineParts) < 4 || len(lineParts) > 5 {
			return nil, errors.WithStack(newError(ErrorCodeFormat, err))
		}

		var backup Backup

		if backup.Backup.CreatedAt, err = time.Parse(time.RFC3339, lineParts[0]); err != nil {
			return nil, errors.WithStack(newError(ErrorCodeDateFormat, err))
		}

		backup.Backup.VaultName = lineParts[1]
		backup.Backup.ID = lineParts[2]
		backup.Backup.Checksum = lineParts[3]

		if len(lineParts) >= 5 {
			backup.Backup.Size, err = strconv.ParseInt(lineParts[4], 10, 64)
			if err != nil {
				return nil, errors.WithStack(newError(ErrorCodeSizeFormat, err))
			}
		}

		backups = append(backups, backup)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WithStack(newError(ErrorCodeReadingFile, err))
	}

	a.logger.Infof("storage: backups listed successfully from audit file storage")
	sort.Sort(backups)
	return backups, nil
}

// Remove a specific backup information from the storage.  On error it will
// return an Error type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *storage.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AuditFile) Remove(id string) error {
	a.logger.Debugf("storage: removing backup “%s” from audit file storage", id)

	backups, err := a.List()
	if err != nil {
		return err
	}

	backupName := a.Filename + "." + time.Now().Format("20060102150405")
	a.logger.Debugf("storage: moving current audit file to “%s”", backupName)
	if err = os.Rename(a.Filename, backupName); err != nil {
		return errors.WithStack(newError(ErrorCodeMovingFile, err))
	}

	auditFile, err := os.OpenFile(a.Filename, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		// TODO: recover backup file
		return errors.WithStack(newError(ErrorCodeOpeningFile, err))
	}
	defer auditFile.Close()

	for _, backup := range backups {
		if backup.Backup.ID == id {
			continue
		}

		audit := fmt.Sprintf("%s %s %s %s %d\n", backup.Backup.CreatedAt.Format(time.RFC3339), backup.Backup.VaultName, backup.Backup.ID, backup.Backup.Checksum, backup.Backup.Size)
		if _, err = auditFile.WriteString(audit); err != nil {
			// TODO: recover backup file
			return errors.WithStack(newError(ErrorCodeWritingFile, err))
		}
	}

	a.logger.Infof("storage: backup “%s” removed successfully from audit file storage", id)
	return nil
}
