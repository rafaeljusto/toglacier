package main

import (
	"context"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jasonlvhit/gocron"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/report"
	"github.com/rafaeljusto/toglacier/internal/storage"
	"github.com/urfave/cli"
)

func main() {
	var toGlacier ToGlacier
	var logger *logrus.Logger
	var logFile *os.File
	defer logFile.Close()

	// ctx is used to abort long transactions, such as big files uploads or
	// inventories
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	var cancelFunc func()

	app := cli.NewApp()
	app.Name = "toglacier"
	app.Usage = "Send data to AWS Glacier service"
	app.Version = "2.0.0"
	app.Authors = []cli.Author{
		{
			Name:  "Rafael Dantas Justo",
			Email: "adm@rafael.net.br",
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Usage: "Tool configuration file (YAML)",
		},
	}
	app.Before = func(c *cli.Context) error {
		config.Default()

		var err error

		if c.String("config") != "" {
			if err = config.LoadFromFile(c.String("config")); err != nil {
				fmt.Printf("error loading configuration file. details: %s\n", err)
				return err
			}
		}

		if err = config.LoadFromEnvironment(); err != nil {
			fmt.Printf("error loading configuration from environment variables. details: %s\n", err)
			return err
		}

		logger = logrus.New()
		logger.Out = os.Stdout

		// optionally set logger output file defined in configuration. if not
		// defined stdout will be used
		if config.Current().Log.File != "" {
			if logFile, err = os.OpenFile(config.Current().Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm); err != nil {
				fmt.Printf("error opening log file “%s”. details: %s\n", config.Current().Log.File, err)
				return err
			}

			// writes to the stdout and to the log file
			logger.Out = io.MultiWriter(os.Stdout, logFile)
		}

		switch config.Current().Log.Level {
		case config.LogLevelDebug:
			logger.Level = logrus.DebugLevel
		case config.LogLevelInfo:
			logger.Level = logrus.InfoLevel
		case config.LogLevelWarning:
			logger.Level = logrus.WarnLevel
		case config.LogLevelError:
			logger.Level = logrus.ErrorLevel
		case config.LogLevelFatal:
			logger.Level = logrus.FatalLevel
		case config.LogLevelPanic:
			logger.Level = logrus.PanicLevel
		}

		awsConfig := cloud.AWSConfig{
			AccountID:       config.Current().AWS.AccountID.Value,
			AccessKeyID:     config.Current().AWS.AccessKeyID.Value,
			SecretAccessKey: config.Current().AWS.SecretAccessKey.Value,
			Region:          config.Current().AWS.Region,
			VaultName:       config.Current().AWS.VaultName,
		}

		var awsCloud cloud.Cloud
		if awsCloud, err = cloud.NewAWSCloud(logger, awsConfig, false); err != nil {
			fmt.Printf("error initializing AWS cloud. details: %s\n", err)
			return err
		}

		var localStorage storage.Storage
		switch config.Current().Database.Type {
		case config.DatabaseTypeAuditFile:
			localStorage = storage.NewAuditFile(logger, config.Current().Database.File)
		case config.DatabaseTypeBoltDB:
			localStorage = storage.NewBoltDB(logger, config.Current().Database.File)
		}

		toGlacier = ToGlacier{
			context: ctx,
			builder: archive.NewTARBuilder(logger),
			envelop: archive.NewOFBEnvelop(logger),
			cloud:   awsCloud,
			storage: localStorage,
		}

		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "sync",
			Usage: "backup now the desired paths to AWS Glacier",
			Action: func(c *cli.Context) error {
				if err := toGlacier.Backup(config.Current().Paths, config.Current().BackupSecret.Value); err != nil {
					logger.Error(err)
				}
				return nil
			},
		},
		{
			Name:      "get",
			Usage:     "retrieve a specific backup from AWS Glacier",
			ArgsUsage: "<archiveID>",
			Action: func(c *cli.Context) error {
				if err := toGlacier.RetrieveBackup(c.Args().First(), config.Current().BackupSecret.Value); err != nil {
					logger.Error(err)
				}

				fmt.Println("Backup recovered successfully")
				return nil
			},
		},
		{
			Name:      "remove",
			Aliases:   []string{"rm"},
			Usage:     "remove a specific backup from AWS Glacier",
			ArgsUsage: "<archiveID>",
			Action: func(c *cli.Context) error {
				if err := toGlacier.RemoveBackup(c.Args().First()); err != nil {
					logger.Error(err)
				}
				return nil
			},
		},
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "list all backups sent to AWS Glacier",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "remote,r",
					Usage: "retrieve the list from AWS Glacier (long wait)",
				},
			},
			Action: func(c *cli.Context) error {
				if backups, err := toGlacier.ListBackups(c.Bool("remote")); err != nil {
					logger.Error(err)

				} else if len(backups) > 0 {
					fmt.Println("Date             | Vault Name       | Archive ID")
					fmt.Printf("%s-+-%s-+-%s\n", strings.Repeat("-", 16), strings.Repeat("-", 16), strings.Repeat("-", 138))
					for _, backup := range backups {
						fmt.Printf("%-16s | %-16s | %-138s\n", backup.Backup.CreatedAt.Format("2006-01-02 15:04"), backup.Backup.VaultName, backup.Backup.ID)
					}
				}
				return nil
			},
		},
		{
			Name:  "start",
			Usage: "run the scheduler (will block forever)",
			Action: func(c *cli.Context) error {
				scheduler := gocron.NewScheduler()
				scheduler.Every(1).Day().At("00:00").Do(func() {
					if err := toGlacier.Backup(config.Current().Paths, config.Current().BackupSecret.Value); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(1).Weeks().At("01:00").Do(func() {
					if err := toGlacier.RemoveOldBackups(config.Current().KeepBackups); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(4).Weeks().At("12:00").Do(func() {
					if _, err := toGlacier.ListBackups(true); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(1).Weeks().At("06:00").Do(func() {
					emailInfo := EmailInfo{
						Sender:   EmailSenderFunc(smtp.SendMail),
						Server:   config.Current().Email.Server,
						Port:     config.Current().Email.Port,
						Username: config.Current().Email.Username,
						Password: config.Current().Email.Password.Value,
						From:     config.Current().Email.From,
						To:       config.Current().Email.To,
					}

					if err := toGlacier.SendReport(emailInfo); err != nil {
						logger.Error(err)
					}
				})

				stopped := scheduler.Start()
				cancelFunc = func() {
					close(stopped)
				}

				select {
				case <-stopped:
					// wait a small period just to give time for the scheduler to shutdown
					time.Sleep(time.Second)
				}

				return nil
			},
		},
		{
			Name:  "report",
			Usage: "test report notification",
			Action: func(c *cli.Context) error {
				report.Add(report.NewTest())

				emailInfo := EmailInfo{
					Sender:   EmailSenderFunc(smtp.SendMail),
					Server:   config.Current().Email.Server,
					Port:     config.Current().Email.Port,
					Username: config.Current().Email.Username,
					Password: config.Current().Email.Password.Value,
					From:     config.Current().Email.From,
					To:       config.Current().Email.To,
				}

				if err := toGlacier.SendReport(emailInfo); err != nil {
					logger.Error(err)
				}

				return nil
			},
		},
		{
			Name:      "encrypt",
			Aliases:   []string{"enc"},
			Usage:     "encrypt a password or secret",
			ArgsUsage: "<password>",
			Action: func(c *cli.Context) error {
				if pwd, err := config.PasswordEncrypt(c.Args().First()); err != nil {
					logger.Error(err)
				} else {
					fmt.Printf("encrypted:%s\n", pwd)
				}
				return nil
			},
		},
	}

	// create a graceful shutdown when receiving a signal (SIGINT, SIGKILL,
	// SIGTERM, SIGSTOP)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGSTOP)

	go func() {
		<-sigs
		if cancelFunc != nil {
			cancelFunc()
		}

		cancel()
	}()

	app.Run(os.Args)
}

// ToGlacier manages backups in the cloud.
type ToGlacier struct {
	context context.Context
	builder archive.Builder
	envelop archive.Envelop
	cloud   cloud.Cloud
	storage storage.Storage
}

// Backup create an archive and send it to the cloud.
func (t ToGlacier) Backup(backupPaths []string, backupSecret string) error {
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
	filename, archiveInfo, err := t.builder.Build(archiveInfo, backupPaths...)
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

	if backupSecret != "" {
		var encryptedFilename string

		timeMark = time.Now()
		if encryptedFilename, err = t.envelop.Encrypt(filename, backupSecret); err != nil {
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
	if backupReport.Backup, err = t.cloud.Send(t.context, filename); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}
	backupReport.Durations.Send = time.Now().Sub(timeMark)

	// fill backup id for new and modified files
	for path, itemInfo := range archiveInfo {
		if itemInfo.Status == archive.ItemInfoStatusNew || itemInfo.Status == archive.ItemInfoStatusModified {
			itemInfo.ID = backupReport.Backup.ID
			archiveInfo[path] = itemInfo
		}
	}

	if err := t.storage.Save(storage.Backup{Backup: backupReport.Backup, Info: archiveInfo}); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}

	return nil
}

// ListBackups show the current backups. With the remote flag it is possible to
// list the backups tracked locally or retrieve the cloud inventory.
func (t ToGlacier) ListBackups(remote bool) (storage.Backups, error) {
	if !remote {
		backups, err := t.storage.List()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// TODO: should we sort here or let the library to do that?
		return backups, nil
	}

	listBackupsReport := report.NewListBackups()
	defer func() {
		report.Add(listBackupsReport)
	}()

	timeMark := time.Now()
	remoteBackups, err := t.cloud.List(t.context)
	if err != nil {
		listBackupsReport.Errors = append(listBackupsReport.Errors, err)
		return nil, errors.WithStack(err)
	}
	listBackupsReport.Durations.List = time.Now().Sub(timeMark)

	// retrieve local backups information only after the remote backups, because the
	// remote backups operations can take a while, and a concurrent action could
	// change the local backups during this time

	backups, err := t.storage.List()
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
			continue
		}

		if err := t.storage.Remove(backup.Backup.ID); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	sort.Strings(kept)

	syncBackups := make(storage.Backups, len(remoteBackups))
	for i, remoteBackup := range remoteBackups {
		// check if a recent backup appeared in the inventory
		if j := sort.SearchStrings(kept, remoteBackup.ID); j < len(kept) {
			if err := t.storage.Remove(kept[j]); err != nil {
				listBackupsReport.Errors = append(listBackupsReport.Errors, err)
				return nil, errors.WithStack(err)
			}
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

		syncBackups[i] = storage.Backup{
			Backup: remoteBackup,
			Info:   archiveInfo,
		}

		if err := t.storage.Save(syncBackups[i]); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	sort.Sort(syncBackups)
	return syncBackups, nil
}

// RetrieveBackup recover a specific backup from the cloud.
func (t ToGlacier) RetrieveBackup(id, backupSecret string) error {
	backups, err := t.storage.List()
	if err != nil {
		return errors.WithStack(err)
	}

	var archiveInfo archive.Info
	for _, backup := range backups {
		if backup.Backup.ID == id {
			archiveInfo = backup.Info
			break
		}
	}

	var ids []string
	var ignoreMainBackup bool
	idPaths := make(map[string][]string)

	if archiveInfo == nil {
		// when there's no archive information, retrieve only the desired backup ID.
		// We will extract the archive information saved in the backup to detect all
		// other backup parts that we need. This is important when the local storage
		// got corrupted due to a disaster
		filenames, err := t.cloud.Get(t.context, id)
		if err != nil {
			return errors.WithStack(err)
		}

		// there's only one backup downloaded at this point
		if archiveInfo, err = t.decryptAndExtract(backupSecret, filenames[id], nil); err != nil {
			return errors.WithStack(err)
		}

		// as we already downloaded the main backup, we should avoid downloading it
		// again when retrieving the backup parts
		ignoreMainBackup = true
	}

	for path, itemInfo := range archiveInfo {
		if (ignoreMainBackup && itemInfo.ID == id) || itemInfo.Status == archive.ItemInfoStatusDeleted {
			continue
		}

		ids = append(ids, itemInfo.ID)
		idPaths[itemInfo.ID] = append(idPaths[itemInfo.ID], path)
	}

	filenames, err := t.cloud.Get(t.context, ids...)
	if err != nil {
		return errors.WithStack(err)
	}

	for id, filename := range filenames {
		// there's only one backup downloaded at this point
		if archiveInfo, err = t.decryptAndExtract(backupSecret, filename, idPaths[id]); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (t ToGlacier) decryptAndExtract(backupSecret, filename string, filter []string) (archive.Info, error) {
	var err error

	if backupSecret != "" {
		var decryptedFilename string

		if decryptedFilename, err = t.envelop.Decrypt(filename, backupSecret); err != nil {
			return nil, errors.WithStack(err)
		}

		if err := os.Rename(filename, decryptedFilename); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	archiveInfo, err := t.builder.Extract(filename, filter)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// TODO: Update archive info in the local storage

	return archiveInfo, nil
}

// RemoveBackup delete a specific backup from the cloud.
func (t ToGlacier) RemoveBackup(id string) error {
	if err := t.cloud.Remove(t.context, id); err != nil {
		return errors.WithStack(err)
	}

	return errors.WithStack(t.storage.Remove(id))
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
		if sort.SearchStrings(preserveBackups, backups[i].Backup.ID) < len(preserveBackups) {
			continue
		}

		removeOldBackupsReport.Backups = append(removeOldBackupsReport.Backups, backups[i].Backup)
		if err := t.RemoveBackup(backups[i].Backup.ID); err != nil {
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
	r, err := report.Build()
	if err != nil {
		return errors.WithStack(err)
	}

	body := fmt.Sprintf(`From: %s
To: %s
Subject: toglacier report

%s`, emailInfo.From, strings.Join(emailInfo.To, ","), r)

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
