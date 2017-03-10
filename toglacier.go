package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/report"
	"github.com/rafaeljusto/toglacier/internal/storage"
	"github.com/urfave/cli"
)

func main() {
	var awsCloud cloud.Cloud
	var auditFileStorage storage.Storage

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
				backup(config.Current().Paths, config.Current().BackupSecret.Value, awsCloud, auditFileStorage)
				return nil
			},
		},
		{
			Name:      "get",
			Usage:     "retrieve a specific backup from AWS Glacier",
			ArgsUsage: "<archiveID>",
			Action: func(c *cli.Context) error {
				if backupFile := retrieveBackup(c.Args().First(), config.Current().BackupSecret.Value, awsCloud); backupFile != "" {
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
				removeBackup(c.Args().First(), awsCloud, auditFileStorage)
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
				backups := listBackups(c.Bool("remote"), awsCloud, auditFileStorage)
				if len(backups) > 0 {
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
				scheduler.Every(1).Day().At("00:00").Do(backup, config.Current().Paths, awsCloud, auditFileStorage)
				scheduler.Every(1).Weeks().At("01:00").Do(removeOldBackups, config.Current().KeepBackups, awsCloud, auditFileStorage)
				scheduler.Every(4).Weeks().At("12:00").Do(listBackups, true, awsCloud, auditFileStorage)
				scheduler.Every(1).Weeks().At("06:00").Do(sendReport, config.Current().Email.Server, config.Current().Email.Port, config.Current().Email.Username, config.Current().Email.Password.Value, config.Current().Email.From, config.Current().Email.To)

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
				sendReport(config.Current().Email.Server, config.Current().Email.Port, config.Current().Email.Username, config.Current().Email.Password.Value, config.Current().Email.From, config.Current().Email.To)
				return nil
			},
		},
		{
			Name:      "encrypt",
			Aliases:   []string{"enc"},
			Usage:     "encrypt a password or secret",
			ArgsUsage: "<password>",
			Action: func(c *cli.Context) error {
				if pwd, err := config.PasswordEncrypt(c.Args().First()); err == nil {
					fmt.Printf("encrypted:%s\n", pwd)
				} else {
					fmt.Printf("error encrypting password. details: %s\n", err)
				}
				return nil
			},
		},
	}

	app.Run(os.Args)
}

func backup(backupPaths []string, backupSecret string, c cloud.Cloud, s storage.Storage) {
	backupReport := report.NewSendBackup()
	backupReport.Paths = backupPaths

	defer func() {
		report.Add(backupReport)
	}()

	timeMark := time.Now()
	filename, err := archive.Build(backupPaths...)
	if err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		log.Println(err)
		return
	}
	defer os.Remove(filename)
	backupReport.Durations.Build = time.Now().Sub(timeMark)

	if backupSecret != "" {
		var err error
		var encryptedFilename string

		timeMark = time.Now()
		if encryptedFilename, err = archive.Encrypt(filename, backupSecret); err != nil {
			backupReport.Errors = append(backupReport.Errors, err)
			log.Println(err)
			return
		}
		backupReport.Durations.Encrypt = time.Now().Sub(timeMark)

		if err := os.Rename(encryptedFilename, filename); err != nil {
			backupReport.Errors = append(backupReport.Errors, err)
			log.Println(err)
			return
		}
	}

	timeMark = time.Now()
	if backupReport.Backup, err = c.Send(filename); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		log.Println(err)
		return
	}
	backupReport.Durations.Send = time.Now().Sub(timeMark)

	if err := s.Save(backupReport.Backup); err != nil {
		backupReport.Errors = append(backupReport.Errors, err)
		log.Println(err)
		return
	}
}

func listBackups(remote bool, c cloud.Cloud, s storage.Storage) []cloud.Backup {
	if !remote {
		backups, err := s.List()
		if err != nil {
			log.Println(err)
			return nil
		}

		return backups
	}

	listBackupsReport := report.NewListBackups()
	defer func() {
		report.Add(listBackupsReport)
	}()

	timeMark := time.Now()
	remoteBackups, err := c.List()
	if err != nil {
		listBackupsReport.Errors = append(listBackupsReport.Errors, err)
		log.Println(err)
		return nil
	}
	listBackupsReport.Durations.List = time.Now().Sub(timeMark)

	// retrieve local backups information only after the remote backups, because the
	// remote backups operations can take a while, and a concurrent action could
	// change the local backups during this time

	backups, err := s.List()
	if err != nil {
		listBackupsReport.Errors = append(listBackupsReport.Errors, err)
		log.Println(err)
		return nil
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
			log.Println(err)
			return nil
		}
	}

	for _, backup := range remoteBackups {
		if err := s.Save(backup); err != nil {
			listBackupsReport.Errors = append(listBackupsReport.Errors, err)
			log.Println(err)
			return nil
		}
	}

	return remoteBackups
}

func retrieveBackup(id, backupSecret string, c cloud.Cloud) string {
	backupFile, err := c.Get(id)
	if err != nil {
		log.Println(err)
		return ""
	}

	if backupSecret != "" {
		var filename string
		if filename, err = archive.Decrypt(backupFile, backupSecret); err != nil {
			log.Println(err)
			return ""
		}

		if err := os.Rename(backupFile, filename); err != nil {
			log.Println(err)
			return ""
		}
	}

	return backupFile
}

func removeBackup(id string, c cloud.Cloud, s storage.Storage) {
	if err := c.Remove(id); err != nil {
		log.Println(err)
		return
	}

	if err := s.Remove(id); err != nil {
		log.Println(err)
		return
	}
}

func removeOldBackups(keepBackups int, c cloud.Cloud, s storage.Storage) {
	removeOldBackupsReport := report.NewRemoveOldBackups()
	defer func() {
		report.Add(removeOldBackupsReport)
	}()

	timeMark := time.Now()
	backups := listBackups(false, c, s)
	removeOldBackupsReport.Durations.List = time.Now().Sub(timeMark)

	timeMark = time.Now()
	for i := keepBackups; i < len(backups); i++ {
		removeOldBackupsReport.Backups = append(removeOldBackupsReport.Backups, backups[i])
		removeBackup(backups[i].ID, c, s)
	}
	removeOldBackupsReport.Durations.Remove = time.Now().Sub(timeMark)
}

func sendReport(emailServer string, emailPort int, emailUsername, emailPassword, emailFrom string, emailTo []string) {
	r, err := report.Build()
	if err != nil {
		log.Println(err)
		return
	}

	body := fmt.Sprintf(`From: %s
To: %s
Subject: toglacier report

%s`, emailFrom, strings.Join(emailTo, ","), r)

	auth := smtp.PlainAuth("", emailUsername, emailPassword, emailServer)

	if err := smtp.SendMail(fmt.Sprintf("%s:%d", emailServer, emailPort), auth, emailFrom, emailTo, []byte(body)); err != nil {
		log.Println(err)
		return
	}
}
