package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jasonlvhit/gocron"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/storage"
	"github.com/urfave/cli"
)

var awsAccountID, awsVaultName, auditFile string
var backupPaths []string
var keepBackups = 10

func init() {
	var err error

	awsAccountID = os.Getenv("AWS_ACCOUNT_ID")
	awsVaultName = os.Getenv("AWS_VAULT_NAME")
	backupPaths = strings.Split(os.Getenv("TOGLACIER_PATH"), ",")
	auditFile = os.Getenv("TOGLACIER_AUDIT")

	if os.Getenv("TOGLACIER_KEEP_BACKUPS") != "" {
		if keepBackups, err = strconv.Atoi(os.Getenv("TOGLACIER_KEEP_BACKUPS")); err != nil {
			fmt.Printf("invalid number of backups to keep. details: %s", err)
			os.Exit(1)
		}
	}
}

func main() {
	awsCloud, err := cloud.NewAWSCloud(awsAccountID, awsVaultName, false)
	if err != nil {
		log.Println(err)
		return
	}
	auditFileStorage := storage.NewAuditFile(auditFile)

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
	app.Commands = []cli.Command{
		{
			Name:  "sync",
			Usage: "backup now the desired paths to AWS Glacier",
			Action: func(c *cli.Context) error {
				backup(awsCloud, auditFileStorage)
				return nil
			},
		},
		{
			Name:      "get",
			Usage:     "retrieve a specific backup from AWS Glacier",
			ArgsUsage: "<archiveID>",
			Action: func(c *cli.Context) error {
				if backupFile := retrieveBackup(c.Args().First(), awsCloud); backupFile != "" {
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
				scheduler.Every(1).Day().At("00:00").Do(backup, nil)
				scheduler.Every(1).Weeks().At("01:00").Do(removeOldBackups, nil)
				<-scheduler.Start()
				return nil
			},
		},
		{
			Name:      "encrypt",
			Aliases:   []string{"enc"},
			Usage:     "encrypt a password or secret",
			ArgsUsage: "<password>",
			Action: func(c *cli.Context) error {
				if pwd, err := cloud.PasswordEncrypt(c.Args().First()); err == nil {
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

func backup(c cloud.Cloud, s storage.Storage) {
	archive, err := archive.Build(backupPaths...)
	if err != nil {
		log.Println(err)
		return
	}
	defer os.Remove(archive)

	backup, err := c.Send(archive)
	if err != nil {
		log.Println(err)
		return
	}

	if err := s.Save(backup); err != nil {
		log.Println(err)
		return
	}
}

func listBackups(remote bool, c cloud.Cloud, s storage.Storage) []cloud.Backup {
	if remote {
		backups, err := c.List()
		if err != nil {
			log.Printf("error retrieving remote backups. details: %s", err)
		}
		return backups
	}

	backups, err := s.List()
	if err != nil {
		log.Println(err)
		return nil
	}
	return backups
}

func retrieveBackup(id string, c cloud.Cloud) string {
	backupFile, err := c.Get(id)
	if err != nil {
		log.Println(err)
		return ""
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

func removeOldBackups(c cloud.Cloud, s storage.Storage) {
	backups := listBackups(true, c, s)
	for i := keepBackups; i < len(backups); i++ {
		removeBackup(backups[i].ID, c, s)
	}
}
