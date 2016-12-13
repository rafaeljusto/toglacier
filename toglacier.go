package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/urfave/cli"
)

func main() {
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
				backup()
				return nil
			},
		},
		{
			Name:      "remove",
			Aliases:   []string{"rm"},
			Usage:     "remove a specific backup from AWS Glacier",
			ArgsUsage: "<archiveID>",
			Action: func(c *cli.Context) error {
				removeBackup(c.Args().First())
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
				backups := listBackups(c.Bool("remote"))
				if len(backups) > 0 {
					fmt.Println("Date             | Vault Name       | Archive ID")
					fmt.Printf("%s-+-%s-+-%s\n", strings.Repeat("-", 16), strings.Repeat("-", 16), strings.Repeat("-", 138))
					for _, result := range backups {
						fmt.Printf("%-16s | %-16s | %-138s\n", result.time.Format("2006-01-02 15:04"), result.vaultName, result.archiveID)
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
				scheduler.Every(1).Weeks().At("01:00").Do(removeOldArchives, nil)
				<-scheduler.Start()
				return nil
			},
		},
	}

	app.Run(os.Args)
}

func backup() {
	archive, err := buildArchive(os.Getenv("TOGLACIER_PATH"))
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		archive.Close()
		// remove the temporary tarball
		os.Remove(archive.Name())
	}()

	backup, err := sendArchive(archive, os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_VAULT_NAME"))
	if err != nil {
		log.Println(err)
		return
	}

	auditFile, err := os.OpenFile(os.Getenv("TOGLACIER_AUDIT"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Printf("error opening the audit file. details: %s", err)
		return
	}
	defer auditFile.Close()

	audit := fmt.Sprintf("%s %s %s %s\n", backup.time.Format(time.RFC3339), backup.vaultName, backup.archiveID, backup.checksum)
	if _, err = auditFile.WriteString(audit); err != nil {
		log.Printf("error writing the audit file. details: %s", err)
		return
	}
}

func listBackups(remote bool) []awsResult {
	if remote {
		backups, err := listArchives(os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_VAULT_NAME"))
		if err != nil {
			log.Printf("error retrieving remote backups. details: %s", err)
		}
		return backups
	}

	auditFile, err := os.Open(os.Getenv("TOGLACIER_AUDIT"))
	if err != nil {
		log.Printf("error opening the audit file. details: %s", err)
		return nil
	}
	defer auditFile.Close()

	var backups []awsResult

	scanner := bufio.NewScanner(auditFile)
	for scanner.Scan() {
		lineParts := strings.Split(scanner.Text(), " ")
		if len(lineParts) != 4 {
			log.Println("corrupted audit file. wrong number of columns")
			return nil
		}

		result := awsResult{
			vaultName: lineParts[1],
			archiveID: lineParts[2],
			checksum:  lineParts[3],
		}

		if result.time, err = time.Parse(time.RFC3339, lineParts[0]); err != nil {
			log.Printf("corrupted audit file. invalid date format. details: %s", err)
			return nil
		}

		backups = append(backups, result)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("error reading the audit file. details: %s", err)
		return nil
	}

	return backups
}

func removeBackup(archiveID string) {
	if err := removeArchive(os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_VAULT_NAME"), archiveID); err != nil {
		log.Println(err)
		return
	}

	// remove the entry from the audit file

	backups := listBackups(false)

	err := os.Rename(os.Getenv("TOGLACIER_AUDIT"), os.Getenv("TOGLACIER_AUDIT")+"."+time.Now().Format("20060102150405"))
	if err != nil {
		log.Printf("error moving audit file. details: %s", err)
		return
	}

	auditFile, err := os.OpenFile(os.Getenv("TOGLACIER_AUDIT"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Printf("error opening the audit file. details: %s", err)
		return
	}
	defer auditFile.Close()

	for _, backup := range backups {
		if backup.archiveID == archiveID {
			continue
		}

		audit := fmt.Sprintf("%s %s %s %s\n", backup.time.Format(time.RFC3339), backup.vaultName, backup.archiveID, backup.checksum)
		if _, err = auditFile.WriteString(audit); err != nil {
			log.Printf("error writing the audit file. details: %s", err)
			return
		}
	}
}

func removeOldBackups() {
	keepBackups := 10
	if os.Getenv("TOGLACIER_KEEP_BACKUPS") != "" {
		var err error
		if keepBackups, err = strconv.Atoi(os.Getenv("TOGLACIER_KEEP_BACKUPS")); err != nil {
			log.Printf("invalid number of backups to keep. details: %s", err)
			return
		}
	}

	if err := removeOldArchives(os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_VAULT_NAME"), keepBackups); err != nil {
		log.Println(err)
		return
	}
}
