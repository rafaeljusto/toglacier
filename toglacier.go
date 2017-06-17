package toglacier

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/log"
	"github.com/rafaeljusto/toglacier/internal/report"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

// ToGlacier manages backups in the cloud.
type ToGlacier struct {
	Context context.Context
	Archive archive.Archive
	Envelop archive.Envelop
	Cloud   cloud.Cloud
	Storage storage.Storage
	Logger  log.Logger
}

// Backup create an archive and send it to the cloud. Optionally encrypt the
// backup with the backupSecret password, if you leave it blank no encryption
// will be performed. There's also an option to stop the backup if there're to
// many files modified (ransomware detection), the modifyTolerance is the
// percentage (0 - 100) of modified files that is tolerated. If there's no need
// to keep track of the modified files set modifyTolerance to 0 or 100.
func (t ToGlacier) Backup(backupPaths []string, backupSecret string, modifyTolerance float64) error {
	backupReport := report.NewSendBackup()
	defer func() {
		report.Add(backupReport)
	}()

	// retrieve the latest backup so we can analyze the files that changed
	backups, err := t.ListBackups(false)
	if err != nil {
		return errors.WithStack(err)
	}

	var archiveInfo archive.Info
	if len(backups) > 0 {
		// the newest backup is always in the first position
		archiveInfo = backups[0].Info
	}

	timeMark := time.Now()
	filename, archiveInfo, err := t.Archive.Build(archiveInfo, backupPaths...)
	if err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}

	if filename == "" {
		// if the filename is empty, the tarball wasn't created because no files
		// were added, so we just ignore the upload
		backupReport.Durations.Build = time.Now().Sub(timeMark)
		return nil
	}

	defer os.Remove(filename)
	backupReport.Durations.Build = time.Now().Sub(timeMark)

	if t.modifyToleranceReached(archiveInfo, modifyTolerance) {
		return errors.WithStack(newError(backupPaths, ErrorCodeModifyTolerance, nil))
	}

	if backupSecret != "" {
		var encryptedFilename string

		timeMark = time.Now()
		if encryptedFilename, err = t.Envelop.Encrypt(filename, backupSecret); err != nil {
			backupReport.Errors = append(backupReport.Errors, err)
			return errors.WithStack(err)
		}
		backupReport.Durations.Encrypt = time.Now().Sub(timeMark)

		if err = os.Rename(encryptedFilename, filename); err != nil {
			backupReport.Errors = append(backupReport.Errors, err)
			return errors.WithStack(err)
		}
	}

	timeMark = time.Now()
	if backupReport.Backup, err = t.Cloud.Send(t.Context, filename); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}
	backupReport.Durations.Send = time.Now().Sub(timeMark)

	// fill backup id for new and modified files
	for path, itemInfo := range archiveInfo {
		if itemInfo.Status.Useful() {
			itemInfo.ID = backupReport.Backup.ID
			archiveInfo[path] = itemInfo
		}
	}

	if err := t.Storage.Save(storage.Backup{Backup: backupReport.Backup, Info: archiveInfo}); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}

	return nil
}

func (t ToGlacier) modifyToleranceReached(archiveInfo archive.Info, modifyTolerance float64) bool {
	if len(archiveInfo) == 0 || modifyTolerance == 0 || modifyTolerance == 100 {
		return false
	}

	var modified int
	for _, itemInfo := range archiveInfo {
		if itemInfo.Status == archive.ItemInfoStatusModified {
			modified++
		}
	}

	modifyPercentage := float64(modified*100) / float64(len(archiveInfo))
	if modifyPercentage > modifyTolerance {
		t.Logger.Warningf("toglacier: detected %.2f%% of modified files (%d/%d), tolerance limited at %.2f%%, aborting backup",
			modifyPercentage, modified, len(archiveInfo), modifyTolerance)
		return true
	}

	return false
}

// ListBackups show the current backups. With the remote flag it is possible to
// list the backups tracked locally or retrieve the cloud inventory.
func (t ToGlacier) ListBackups(remote bool) (storage.Backups, error) {
	if remote {
		return t.listRemoteBackups()
	}

	backups, err := t.Storage.List()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sort.Sort(backupsByCreationDate(backups))
	return backups, nil
}

func (t ToGlacier) listRemoteBackups() (storage.Backups, error) {
	listBackupsReport := report.NewListBackups()
	defer func() {
		report.Add(listBackupsReport)
	}()

	timeMark := time.Now()
	remoteBackups, err := t.Cloud.List(t.Context)
	if err != nil {
		listBackupsReport.Errors = append(listBackupsReport.Errors, err)
		return nil, errors.WithStack(err)
	}
	listBackupsReport.Durations.List = time.Now().Sub(timeMark)

	// retrieve local backups information only after the remote backups, because the
	// remote backups operations can take a while, and a concurrent action could
	// change the local backups during this time

	backups, err := t.Storage.List()
	if err != nil {
		listBackupsReport.Errors = append(listBackupsReport.Errors, err)
		return nil, errors.WithStack(err)
	}

	// http://docs.aws.amazon.com/amazonglacier/latest/dev/working-with-archives.html#client-side-key-map-concept
	//
	// If you maintain client-side archive metadata, note that Amazon Glacier
	// maintains a vault inventory that includes archive IDs and any
	// descriptions you provided during the archive upload. You might
	// occasionally download the vault inventory to reconcile any issues in your
	// client-side database you maintain for the archive metadata. However,
	// Amazon Glacier takes vault inventory approximately daily. When you
	// request a vault inventory, Amazon Glacier returns the last inventory it
	// prepared, a point in time snapshot.

	// TODO: if the change is greater than 20% something is really wrong, and
	// maybe the best approach is to do nothing and report the problem.

	var kept []string
	for _, backup := range backups {
		// http://docs.aws.amazon.com/amazonglacier/latest/dev/vault-inventory.html#vault-inventory-about
		//
		// Amazon Glacier updates a vault inventory approximately once a day,
		// starting on the day you first upload an archive to the vault. If there
		// have been no archive additions or deletions to the vault since the last
		// inventory, the inventory date is not updated. When you initiate a job for
		// a vault inventory, Amazon Glacier returns the last inventory it
		// generated, which is a point-in-time snapshot and not real-time data. Note
		// that after Amazon Glacier creates the first inventory for the vault, it
		// typically takes half a day and up to a day before that inventory is
		// available for retrieval.
		if backup.Backup.CreatedAt.After(time.Now().Add(-24 * time.Hour)) {
			// recent backups could not be in the inventory yet
			kept = append(kept, backup.Backup.ID)
			t.Logger.Debugf("toglacier: backup id “%s” kept because is to recent", backup.Backup.ID)
			continue
		}

		if err := t.Storage.Remove(backup.Backup.ID); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	sort.Strings(kept)

	syncBackups := make(storage.Backups, 0, len(remoteBackups))
	for i, remoteBackup := range remoteBackups {
		// check if a recent backup appeared in the inventory
		if j := sort.SearchStrings(kept, remoteBackup.ID); j < len(kept) && kept[j] == remoteBackup.ID {
			if err := t.Storage.Remove(kept[j]); err != nil {
				listBackupsReport.Errors = append(listBackupsReport.Errors, err)
				return nil, errors.WithStack(err)
			}

			t.Logger.Debugf("toglacier: backup id “%s” removed because it was found remotely", kept[j])
			kept = append(kept[:j], kept[j+1:]...)
		}

		// we should keep the archive information to be able to build incremental
		// backups again. Another alternative is build the archive information from
		// the uploaded backup, but it is really slow. Anyway, when retrieving the
		// backup, if there's no archive information, we will try to extract it from
		// the backup
		var archiveInfo archive.Info
		for _, backup := range backups {
			if backup.Backup.ID == remoteBackup.ID {
				archiveInfo = backup.Info
				break
			}
		}

		syncBackups = append(syncBackups, storage.Backup{
			Backup: remoteBackup,
			Info:   archiveInfo,
		})

		if err := t.Storage.Save(syncBackups[i]); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	// add backups that were kept
	for _, id := range kept {
		if backup, ok := backups.Search(id); ok {
			syncBackups = append(syncBackups, backup)
		}
	}

	sort.Sort(backupsByCreationDate(syncBackups))
	return syncBackups, nil
}

// RetrieveBackup recover a specific backup from the cloud. If the backup is
// encrypted it can be decrypted if the backupSecret is informed. Also, it is
// possible to avoid downloading backups that contain only unmodified files with
// the skipUnmodified flag.
func (t ToGlacier) RetrieveBackup(id, backupSecret string, skipUnmodified bool) error {
	backups, err := t.Storage.List()
	if err != nil {
		return errors.WithStack(err)
	}

	selectedBackup, ok := backups.Search(id)
	if !ok {
		t.Logger.Warningf("toglacier: backup “%s” not found in local storage")
	}

	var ignoreMainBackup bool

	if selectedBackup.Info == nil {
		var filenames map[string]string

		// when there's no archive information, retrieve only the desired backup ID.
		// We will extract the archive information saved in the backup to detect all
		// other backup parts that we need. This is important when the local storage
		// got corrupted due to a disaster
		if filenames, err = t.Cloud.Get(t.Context, id); err != nil {
			return errors.WithStack(err)
		}

		// there's only one backup downloaded at this point
		if selectedBackup.Info, err = t.decryptAndExtract(backupSecret, filenames[id], nil); err != nil {
			return errors.WithStack(err)
		}

		// synchronize the archive information in the local storage only if the
		// backup exists
		if selectedBackup.Backup.ID != "" {
			if err = t.Storage.Save(selectedBackup); err != nil {
				return errors.WithStack(err)
			}
		}

		// as we already downloaded the main backup, we should avoid downloading it
		// again when retrieving the backup parts
		ignoreMainBackup = true
	}

	ids, idPaths, err := t.extractIDs(id, selectedBackup.Info, ignoreMainBackup, skipUnmodified)
	if err != nil {
		return errors.WithStack(err)
	}

	filenames, err := t.Cloud.Get(t.Context, ids...)
	if err != nil {
		return errors.WithStack(err)
	}

	for id, filename := range filenames {
		if selectedBackup, ok = backups.Search(id); !ok {
			t.Logger.Warningf("toglacier: backup “%s” not found in local storage")
		}

		if selectedBackup.Info, err = t.decryptAndExtract(backupSecret, filename, idPaths[id]); err != nil {
			return errors.WithStack(err)
		}

		if err = t.synchronizeArchiveInfo(selectedBackup, backups); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (t ToGlacier) extractIDs(id string, archiveInfo archive.Info, ignoreMainBackup, skipUnmodified bool) (ids []string, idPaths map[string][]string, err error) {
	idPaths = make(map[string][]string)
	for path, itemInfo := range archiveInfo {
		// if we already downloaded the main backup we don't need to download it
		// again, and we should also avoid downloading backups parts just to
		// retrieve removed files
		ignore := (ignoreMainBackup && itemInfo.ID == id) || itemInfo.Status == archive.ItemInfoStatusDeleted

		if !ignore && skipUnmodified {
			var checksum string
			if checksum, err = t.Archive.FileChecksum(path); err != nil {
				return nil, nil, errors.WithStack(err)
			}

			// file did not change since this backup
			if checksum == itemInfo.Checksum {
				t.Logger.Infof("toglacier: file “%s” unmodified in disk since backup, it will be ignored", path)
				ignore = true
			}
		}

		if !ignore {
			idPaths[itemInfo.ID] = append(idPaths[itemInfo.ID], path)
		}
	}

	for id := range idPaths {
		ids = append(ids, id)
	}
	return
}

func (t ToGlacier) decryptAndExtract(backupSecret, filename string, filter []string) (archive.Info, error) {
	var err error

	if backupSecret != "" {
		var decryptedFilename string

		if decryptedFilename, err = t.Envelop.Decrypt(filename, backupSecret); err != nil {
			return nil, errors.WithStack(err)
		}

		if err = os.Rename(decryptedFilename, filename); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	archiveInfo, err := t.Archive.Extract(filename, filter)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// after extracting the content we don't need the archive anymore, but if
	// there's some error removing it we don't want to stop the process
	if err = os.Remove(filename); err != nil {
		t.Logger.Warningf("toglacier: failed to remove file “%s”. details: %s", filename, err)
	}

	return archiveInfo, nil
}

func (t ToGlacier) synchronizeArchiveInfo(backup storage.Backup, backups storage.Backups) error {
	// synchronize the archive information in the local storage only if the
	// backup exists
	if backup.Backup.ID == "" {
		return nil
	}

	// there're some actions performed locally that aren't synchronized with
	// the cloud. For example, when removing a backup we replace the file
	// references of the removed backup in other backups, and many archive
	// information in the cloud gets outdated. So we will check if all
	// references in the remote archive information are valid before replacing
	// the local version
	if !backups.ValidInfo(backup.Info) {
		t.Logger.Warningf("toglacier: archive information from backup “%s” is outdated and will not be synchronized", backup.Backup.ID)
		return nil
	}

	return errors.WithStack(t.Storage.Save(backup))
}

// RemoveBackups delete a backups identified by ids from the cloud and from the
// local storage. It will also try to replace or remove the reference from the
// removed backup on other backups. When it is possible to replace the reference
// it will try to get the file version right before the removed backup date.
func (t ToGlacier) RemoveBackups(ids ...string) error {
	for _, id := range ids {
		if err := t.removeBackup(id); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (t ToGlacier) removeBackup(id string) error {
	if err := t.Cloud.Remove(t.Context, id); err != nil {
		return errors.WithStack(err)
	}

	if err := t.rearrangeStorage(id); err != nil {
		// TODO: an error here will cause an inconsistency between the cloud and the
		// local storage
		return errors.WithStack(err)
	}

	if err := t.Storage.Remove(id); err != nil {
		// TODO: an error here will cause an inconsistency between the cloud and the
		// local storage
		return errors.WithStack(err)
	}

	return nil
}

func (t ToGlacier) rearrangeStorage(id string) error {
	// remove references from this id from other backups to keep the consistency
	// of the local storage. We will try to replace the reference id by the most
	// recently version of the file when possible

	backups, err := t.Storage.List()
	if err != nil {
		return errors.WithStack(err)
	}

	// order backups by creation date
	sort.Sort(backupsByCreationDate(backups))

	backupIndex := -1
	var archiveInfo archive.Info

	// store the replaceable file references
	fallbackFiles := make(map[string]string)

	// first we move from the most recent backup to the oldest one, looking for
	// the backup position that will be removed, and to store all files that we
	// should look for. After we got the files to look for, we will continue
	// looking older backups to find reference for this files.
	for i := 0; i < len(backups) && (backupIndex == -1 || len(archiveInfo) > 0); i++ {
		if backups[i].Backup.ID == id {
			backupIndex = i

			// we are only interested in modified files, because if it is a new file,
			// there's nothing we can do, if it is a unmodified file, the id is
			// already referencing another backup, and if it is a deleted file it will
			// not appear in newer backups
			archiveInfo = backups[i].Info.FilterByStatuses(archive.ItemInfoStatusModified)
			continue
		}

		if backupIndex > -1 {
			// keep looking in older backups for the desired files
			for filename, itemInfo := range backups[i].Info {
				// we are only interested in new and modified file matches
				if _, ok := archiveInfo[filename]; ok && itemInfo.Status.Useful() {
					fallbackFiles[filename] = backups[i].Backup.ID
					delete(archiveInfo, filename)
				}
			}
		}
	}

	// now we need to look for backups that were created after the removed one, so
	// we can replace the reference of the files or removed if we couldn't find
	// any match
	for i := backupIndex - 1; i >= 0; i-- {
		if t.rearrangeArchiveInfo(id, backups[i].Info, fallbackFiles) {
			if err = t.Storage.Save(backups[i]); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	return nil
}

func (t ToGlacier) rearrangeArchiveInfo(id string, archiveInfo archive.Info, fallbackFiles map[string]string) (modified bool) {
	for filename, itemInfo := range archiveInfo {
		if itemInfo.ID != id {
			continue
		}

		if newID, ok := fallbackFiles[filename]; ok {
			// we don't need to look for the status in the item info because if it
			// references the removed backup it should be the unmodified or deleted
			// statuses
			itemInfo.ID = newID
			archiveInfo[filename] = itemInfo

		} else {
			// https://golang.org/ref/spec#For_range
			// If map entries that have not yet been reached are removed during
			// iteration, the corresponding iteration values will not be produced
			delete(archiveInfo, filename)
		}

		modified = true
	}

	return
}

// RemoveOldBackups delete old backups from the cloud. This will optimize the
// cloud space usage, as too old backups aren't used.
func (t ToGlacier) RemoveOldBackups(keepBackups int) error {
	removeOldBackupsReport := report.NewRemoveOldBackups()
	defer func() {
		report.Add(removeOldBackupsReport)
	}()

	timeMark := time.Now()
	backups, err := t.ListBackups(false)
	removeOldBackupsReport.Durations.List = time.Now().Sub(timeMark)

	if err != nil {
		removeOldBackupsReport.Errors = append(removeOldBackupsReport.Errors, err)
		return errors.WithStack(err)
	}

	sort.Sort(backupsByCreationDate(backups))

	// with the incremental backup we cannot remove backups without checking the
	// archive info to identify partial backup entries
	var preserveBackups []string
	for i := 0; i < keepBackups && i < len(backups); i++ {
		for _, itemInfo := range backups[i].Info {
			if itemInfo.Status != archive.ItemInfoStatusDeleted {
				preserveBackups = append(preserveBackups, itemInfo.ID)
			}
		}
	}
	sort.Strings(preserveBackups)

	timeMark = time.Now()
	for i := keepBackups; i < len(backups); i++ {
		// check if the backup isn't referenced by a active backup
		if j := sort.SearchStrings(preserveBackups, backups[i].Backup.ID); j < len(preserveBackups) && preserveBackups[j] == backups[i].Backup.ID {
			continue
		}

		removeOldBackupsReport.Backups = append(removeOldBackupsReport.Backups, backups[i].Backup)
		if err := t.RemoveBackups(backups[i].Backup.ID); err != nil {
			removeOldBackupsReport.Errors = append(removeOldBackupsReport.Errors, err)
			return errors.WithStack(err)
		}
	}
	removeOldBackupsReport.Durations.Remove = time.Now().Sub(timeMark)

	return nil
}

// SendReport send information from the actions performed by this tool via
// e-mail to an administrator.
func (t ToGlacier) SendReport(emailInfo EmailInfo) error {
	r, err := report.Build(emailInfo.Format)
	if err != nil {
		return errors.WithStack(err)
	}

	body := fmt.Sprintf(`From: %s
To: %s
Subject: toglacier report
MIME-Version: 1.0
Content-Type: %s; charset=utf-8

%s`, emailInfo.From, strings.Join(emailInfo.To, ","), emailInfo.Format, r)

	auth := smtp.PlainAuth("", emailInfo.Username, emailInfo.Password, emailInfo.Server)
	err = emailInfo.Sender.SendMail(fmt.Sprintf("%s:%d", emailInfo.Server, emailInfo.Port), auth, emailInfo.From, emailInfo.To, []byte(body))
	return errors.WithStack(err)
}

// EmailInfo stores all necessary information to send an e-mail.
type EmailInfo struct {
	Sender   EmailSender
	Server   string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	Format   report.Format
}

// EmailSender e-mail API to make it easy to mock the smtp.SendEmail function.
type EmailSender interface {
	SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// EmailSenderFunc helper function to create a fast implementation of the
// EmailSender interface.
type EmailSenderFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

// SendMail sends the e-mail.
func (r EmailSenderFunc) SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	return r(addr, a, from, to, msg)
}

// backupsByCreationDate reorder the backups by reverse creation date.
type backupsByCreationDate storage.Backups

// Len returns the number of backups.
func (b backupsByCreationDate) Len() int { return len(b) }

// Less compares two positions of the slice and verifies the preference. They
// are ordered from the newest backup to the oldest.
func (b backupsByCreationDate) Less(i, j int) bool {
	return b[i].Backup.CreatedAt.After(b[j].Backup.CreatedAt)
}

// Swap change the backups position inside the slice.
func (b backupsByCreationDate) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
