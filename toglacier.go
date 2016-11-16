package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	archive, err := buildArchive(os.Getenv("TOGLACIER_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	defer archive.Close()

	result, err := sendArchive(archive, os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_VAULT_NAME"))
	if err != nil {
		log.Fatal(err)
	}

	auditFile, err := os.OpenFile(os.Getenv("TOGLACIER_AUDIT"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("error opening the audit file. details: %s", err)
	}
	defer auditFile.Close()

	audit := fmt.Sprintf("%s %s %s\n", result.time.Format(time.RFC3339), result.location, result.checksum)
	if _, err = auditFile.WriteString(audit); err != nil {
		log.Fatalf("error writing the audit file. details: %s", err)
	}
}
