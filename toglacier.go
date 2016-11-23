package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jasonlvhit/gocron"
)

func main() {
	scheduler := gocron.NewScheduler()
	scheduler.Every(1).Day().At("00:00").Do(backup, nil)
	scheduler.Every(1).Weeks().At("01:00").Do(removeOldArchives, nil)
	<-scheduler.Start()
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

	result, err := sendArchive(archive, os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_VAULT_NAME"))
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

	audit := fmt.Sprintf("%s %s %s\n", result.time.Format(time.RFC3339), result.location, result.checksum)
	if _, err = auditFile.WriteString(audit); err != nil {
		log.Printf("error writing the audit file. details: %s", err)
		return
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
