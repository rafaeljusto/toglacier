package main

import (
	"context"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"os/signal"
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
	// iventories
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
				if backupFile, err := toGlacier.RetrieveBackup(c.Args().First(), config.Current().BackupSecret.Value); err != nil {
					logger.Error(err)
				} else if backupFile != "" {
					fmt.Printf("Backup recovered at %s\n", backupFile)
				}
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
						fmt.Printf("%-16s | %-16s | %-138s\n", backup.CreatedAt.Format("2006-01-02 15:04"), backup.VaultName, backup.ID)
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

	timeMark := time.Now()
	filename, err := t.builder.Build(backupPaths...)
	if err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}
	defer os.Remove(filename)
	backupReport.Durations.Build = time.Now().Sub(timeMark)

	if backupSecret != "" {
		var err error
		var encryptedFilename string

		timeMark = time.Now()
		if encryptedFilename, err = t.envelop.Encrypt(filename, backupSecret); err != nil {
			backupReport.Errors = append(backupReport.Errors, err)
			return errors.WithStack(err)
		}
		backupReport.Durations.Encrypt = time.Now().Sub(timeMark)

		if err := os.Rename(encryptedFilename, filename); err != nil {
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

	if err := t.storage.Save(backupReport.Backup); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}

	return nil
}

// ListBackups show the current backups. With the remote flag it is possible to
// list the backups tracked locally or retrieve the cloud inventory.
func (t ToGlacier) ListBackups(remote bool) ([]cloud.Backup, error) {
	if !remote {
		backups, err := t.storage.List()
		return backups, errors.WithStack(err)
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

	for _, backup := range backups {
		if err := t.storage.Remove(backup.ID); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	for _, backup := range remoteBackups {
		if err := t.storage.Save(backup); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	return remoteBackups, nil
}

// RetrieveBackup recover a specific backup from the cloud.
func (t ToGlacier) RetrieveBackup(id, backupSecret string) (string, error) {
	backupFile, err := t.cloud.Get(t.context, id)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if backupSecret != "" {
		var filename string
		if filename, err = t.envelop.Decrypt(backupFile, backupSecret); err != nil {
			return "", errors.WithStack(err)
		}

		if err := os.Rename(backupFile, filename); err != nil {
			return "", errors.WithStack(err)
		}
	}

	return backupFile, nil
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

	timeMark = time.Now()
	for i := keepBackups; i < len(backups); i++ {
		removeOldBackupsReport.Backups = append(removeOldBackupsReport.Backups, backups[i])
		if err := t.RemoveBackup(backups[i].ID); err != nil {
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
