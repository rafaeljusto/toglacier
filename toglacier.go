package main

import (
	"archive/tar"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
)

// Environment variables:
// AWS_ACCOUNT_ID=my@email.com
// AWS_ACCESS_KEY_ID=AKID1234567890
// AWS_SECRET_ACCESS_KEY=MY-SECRET-KEY
// AWS_REGION=us-east-1
// TOGLACIER_PATH=/mybackup/data

func main() {
	archive, err := buildArchive(os.Getenv("TOGLACIER_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	defer archive.Close()

	archiveID, location, err := sendArchive(archive, os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_REGION"))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Archive ID: %s", archiveID)
	log.Printf("Location: %s", location)
}

func buildArchive(backupPath string) (*os.File, error) {
	if fileInfo, err := os.Stat(backupPath); err != nil {
		return nil, fmt.Errorf("error checking the path to backup. details: %s", err)

	} else if !fileInfo.IsDir() {
		return nil, fmt.Errorf("path to backup is not a directory")
	}

	files, err := ioutil.ReadDir(backupPath)
	if err != nil {
		return nil, fmt.Errorf("error reading the path to backup. details: %s", err)
	}

	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return nil, fmt.Errorf("error creating the tar file. details: %s", err)
	}

	tarArchive := tar.NewWriter(tarFile)

	for _, file := range files {
		tarHeader := tar.Header{
			Name: file.Name(),
			Mode: 0600,
			Size: file.Size(),
		}

		if err := tarArchive.WriteHeader(&tarHeader); err != nil {
			tarFile.Close()
			return nil, fmt.Errorf("error writing header in tar for file %s. details: %s", file.Name(), err)
		}

		fd, err := os.Open(file.Name())
		if err != nil {
			tarFile.Close()
			return nil, fmt.Errorf("error opening file %s. details: %s", file.Name(), err)
		}

		if n, err := io.Copy(tarArchive, fd); err != nil {
			tarFile.Close()
			return nil, fmt.Errorf("error writing content in tar for file %s. details: %s", file.Name(), err)

		} else if n != file.Size() {
			tarFile.Close()
			return nil, fmt.Errorf("wrong number of bytes writen in file %s", file.Name())
		}

		if err := fd.Close(); err != nil {
			tarFile.Close()
			return nil, fmt.Errorf("error closing file %s. details: %s", file.Name(), err)
		}
	}

	if err := tarArchive.Close(); err != nil {
		tarFile.Close()
		return nil, fmt.Errorf("error generating tar file. details: %s", err)
	}

	return tarFile, nil
}

func sendArchive(file *os.File, awsAccountID, awsRegion string) (archiveID, location string, err error) {
	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(file)

	awsArchive := glacier.UploadArchiveInput{
		AccountId:          aws.String(awsAccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", time.Now().Format(time.RFC3339))),
		Body:               file,
		Checksum:           aws.String(hex.EncodeToString(hash.TreeHash)),
	}

	awsSession, err := session.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("error creating aws session. details: %s", err)
	}

	awsGlacier := glacier.New(awsSession, &aws.Config{
		Region: aws.String(awsRegion),
	})

	response, err := awsGlacier.UploadArchive(&awsArchive)
	if err != nil {
		return "", "", fmt.Errorf("error sending archive to aws glacier. details: %s", err)
	}

	return *response.ArchiveId, *response.Location, nil
}
