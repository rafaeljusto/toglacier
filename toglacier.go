package main

import (
	"fmt"
	"io"
	"net/smtp"
	"os"
	"strings"
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
	var logger *logrus.Logger
	var tarBuilder archive.Builder
	var ofbEnvelop archive.Envelop
	var awsCloud cloud.Cloud
	var auditFileStorage storage.Storage

	var logFile *os.File
	defer logFile.Close()

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
			if logFile, err = os.OpenFile(config.Current().Log.File, os.O_CREATE|os.O_WRONLY, os.ModePerm); err != nil {
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

		tarBuilder = archive.NewTARBuilder(logger)
		ofbEnvelop = archive.NewOFBEnvelop()

		if awsCloud, err = cloud.NewAWSCloud(config.Current(), false); err != nil {
			fmt.Printf("error initializing AWS cloud. details: %s\n", err)
			return err
		}

		auditFileStorage = storage.NewAuditFile(config.Current().AuditFile)
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "sync",
			Usage: "backup now the desired paths to AWS Glacier",
			Action: func(c *cli.Context) error {
				if err := backup(config.Current().Paths, config.Current().BackupSecret.Value, tarBuilder, ofbEnvelop, awsCloud, auditFileStorage); err != nil {
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
				if backupFile, err := retrieveBackup(c.Args().First(), config.Current().BackupSecret.Value, ofbEnvelop, awsCloud); err != nil {
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
				if err := removeBackup(c.Args().First(), awsCloud, auditFileStorage); err != nil {
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
				if backups, err := listBackups(c.Bool("remote"), awsCloud, auditFileStorage); err != nil {
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
					if err := backup(config.Current().Paths, config.Current().BackupSecret.Value, tarBuilder, ofbEnvelop, awsCloud, auditFileStorage); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(1).Weeks().At("01:00").Do(func() {
					if err := removeOldBackups(config.Current().KeepBackups, awsCloud, auditFileStorage); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(4).Weeks().At("12:00").Do(func() {
					if _, err := listBackups(true, awsCloud, auditFileStorage); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(1).Weeks().At("06:00").Do(func() {
					err := sendReport(emailSenderFunc(smtp.SendMail),
						config.Current().Email.Server,
						config.Current().Email.Port,
						config.Current().Email.Username,
						config.Current().Email.Password.Value,
						config.Current().Email.From,
						config.Current().Email.To)

					if err != nil {
						logger.Error(err)
					}
				})

				// TODO: Create a graceful shutdown when receiving a signal (SIGINT,
				// SIGKILL, SIGTERM, SIGSTOP).
				<-scheduler.Start()
				return nil
			},
		},
		{
			Name:  "report",
			Usage: "test report notification",
			Action: func(c *cli.Context) error {
				report.Add(report.NewTest())

				err := sendReport(emailSenderFunc(smtp.SendMail),
					config.Current().Email.Server,
					config.Current().Email.Port,
					config.Current().Email.Username,
					config.Current().Email.Password.Value,
					config.Current().Email.From,
					config.Current().Email.To)

				if err != nil {
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

	app.Run(os.Args)
}

func backup(backupPaths []string, backupSecret string, b archive.Builder, e archive.Envelop, c cloud.Cloud, s storage.Storage) error {
	backupReport := report.NewSendBackup()
	backupReport.Paths = backupPaths

	defer func() {
		report.Add(backupReport)
	}()

	timeMark := time.Now()
	filename, err := b.Build(backupPaths...)
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
		if encryptedFilename, err = e.Encrypt(filename, backupSecret); err != nil {
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
	if backupReport.Backup, err = c.Send(filename); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}
	backupReport.Durations.Send = time.Now().Sub(timeMark)

	if err := s.Save(backupReport.Backup); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		return errors.WithStack(err)
	}

	return nil
}

func listBackups(remote bool, c cloud.Cloud, s storage.Storage) ([]cloud.Backup, error) {
	if !remote {
		backups, err := s.List()
		return backups, errors.WithStack(err)
	}

	listBackupsReport := report.NewListBackups()
	defer func() {
		report.Add(listBackupsReport)
	}()

	timeMark := time.Now()
	remoteBackups, err := c.List()
	if err != nil {
		listBackupsReport.Errors = append(listBackupsReport.Errors, err)
		return nil, errors.WithStack(err)
	}
	listBackupsReport.Durations.List = time.Now().Sub(timeMark)

	// retrieve local backups information only after the remote backups, because the
	// remote backups operations can take a while, and a concurrent action could
	// change the local backups during this time

	backups, err := s.List()
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
		if err := s.Remove(backup.ID); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	for _, backup := range remoteBackups {
		if err := s.Save(backup); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			return nil, errors.WithStack(err)
		}
	}

	return remoteBackups, nil
}

func retrieveBackup(id, backupSecret string, e archive.Envelop, c cloud.Cloud) (string, error) {
	backupFile, err := c.Get(id)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if backupSecret != "" {
		var filename string
		if filename, err = e.Decrypt(backupFile, backupSecret); err != nil {
			return "", errors.WithStack(err)
		}

		if err := os.Rename(backupFile, filename); err != nil {
			return "", errors.WithStack(err)
		}
	}

	return backupFile, nil
}

func removeBackup(id string, c cloud.Cloud, s storage.Storage) error {
	if err := c.Remove(id); err != nil {
		return errors.WithStack(err)
	}

	return errors.WithStack(s.Remove(id))
}

func removeOldBackups(keepBackups int, c cloud.Cloud, s storage.Storage) error {
	removeOldBackupsReport := report.NewRemoveOldBackups()
	defer func() {
		report.Add(removeOldBackupsReport)
	}()

	timeMark := time.Now()
	backups, err := listBackups(false, c, s)
	removeOldBackupsReport.Durations.List = time.Now().Sub(timeMark)

	if err != nil {
		removeOldBackupsReport.Errors = append(removeOldBackupsReport.Errors, err)
		return errors.WithStack(err)
	}

	timeMark = time.Now()
	for i := keepBackups; i < len(backups); i++ {
		removeOldBackupsReport.Backups = append(removeOldBackupsReport.Backups, backups[i])
		if err := removeBackup(backups[i].ID, c, s); err != nil {
			removeOldBackupsReport.Errors = append(removeOldBackupsReport.Errors, err)
			return errors.WithStack(err)
		}
	}
	removeOldBackupsReport.Durations.Remove = time.Now().Sub(timeMark)

	return nil
}

func sendReport(emailSender emailSender, emailServer string, emailPort int, emailUsername, emailPassword, emailFrom string, emailTo []string) error {
	r, err := report.Build()
	if err != nil {
		return errors.WithStack(err)
	}

	body := fmt.Sprintf(`From: %s
To: %s
Subject: toglacier report

%s`, emailFrom, strings.Join(emailTo, ","), r)

	auth := smtp.PlainAuth("", emailUsername, emailPassword, emailServer)
	err = emailSender.SendMail(fmt.Sprintf("%s:%d", emailServer, emailPort), auth, emailFrom, emailTo, []byte(body))
	return errors.WithStack(err)
}

type emailSender interface {
	SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

type emailSenderFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

func (r emailSenderFunc) SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	return r(addr, a, from, to, msg)
}
