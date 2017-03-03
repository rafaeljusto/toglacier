package storage

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rafaeljusto/toglacier/internal/cloud"
)

// AuditFile stores all backup informations in a simple text file.
type AuditFile struct {
	Filename string
}

// NewAuditFile initializes a new AuditFile object.
func NewAuditFile(filename string) *AuditFile {
	return &AuditFile{
		Filename: filename,
	}
}

// Save a backup information. It stores the backup information one per line with
// the following columns:
//
//     [datetime] [vaultName] [archiveID] [checksum]
func (a *AuditFile) Save(backup cloud.Backup) error {
	auditFile, err := os.OpenFile(a.Filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("error opening the audit file. details: %s", err)
	}
	defer auditFile.Close()

	audit := fmt.Sprintf("%s %s %s %s\n", backup.CreatedAt.Format(time.RFC3339), backup.VaultName, backup.ID, backup.Checksum)
	if _, err = auditFile.WriteString(audit); err != nil {
		return fmt.Errorf("error writing the audit file. details: %s", err)
	}

	return nil
}

// List all backup informations in the storage.
func (a *AuditFile) List() ([]cloud.Backup, error) {
	auditFile, err := os.Open(a.Filename)
	if err != nil {
		// if the file doesn't exist we can presume that there's no backups yet
		if pathErr, ok := err.(*os.PathError); ok && os.IsNotExist(pathErr.Err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error opening the audit file. details: %s", err)
	}
	defer auditFile.Close()

	var backups []cloud.Backup

	scanner := bufio.NewScanner(auditFile)
	for scanner.Scan() {
		lineParts := strings.Split(scanner.Text(), " ")
		if len(lineParts) != 4 {
			return nil, errors.New("corrupted audit file. wrong number of columns")
		}

		backup := cloud.Backup{
			VaultName: lineParts[1],
			ID:        lineParts[2],
			Checksum:  lineParts[3],
		}

		if backup.CreatedAt, err = time.Parse(time.RFC3339, lineParts[0]); err != nil {
			return nil, fmt.Errorf("corrupted audit file. invalid date format. details: %s", err)
		}

		backups = append(backups, backup)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading the audit file. details: %s", err)
	}

	return backups, nil
}

// Remove a specific backup information from the storage.
func (a *AuditFile) Remove(id string) error {
	backups, err := a.List()
	if err != nil {
		return err
	}

	if err = os.Rename(a.Filename, a.Filename+"."+time.Now().Format("20060102150405")); err != nil {
		return fmt.Errorf("error moving audit file. details: %s", err)
	}

	auditFile, err := os.OpenFile(a.Filename, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("error opening the audit file. details: %s", err)
	}
	defer auditFile.Close()

	for _, backup := range backups {
		if backup.ID == id {
			continue
		}

		audit := fmt.Sprintf("%s %s %s %s\n", backup.CreatedAt.Format(time.RFC3339), backup.VaultName, backup.ID, backup.Checksum)
		if _, err = auditFile.WriteString(audit); err != nil {
			return fmt.Errorf("error writing the audit file. details: %s", err)
		}
	}

	return nil
}
