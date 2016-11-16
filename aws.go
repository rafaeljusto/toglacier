package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/aws/aws-sdk-go/service/glacier/glacieriface"
)

// partSize the size of each part of the multipart upload except the last, in
// bytes. The last part can be smaller than this part size.
const partSize int64 = 4096 // 4 MB will limit the archive in 40GB

// awsResult store all the information that we need to find the backup later.
type awsResult struct {
	time     time.Time
	location string
	checksum string
}

// awsGlacier represents the AWS Glacier API, useful for mocking in unit tests.
var awsGlacier glacieriface.GlacierAPI

func init() {
	awsSession, err := session.NewSession()
	if err != nil {
		log.Printf("error creating aws session. details: %s", err)
		os.Exit(1)
	}

	awsGlacier = glacier.New(awsSession)

	// Uncomment the line bellow to understand what is going on
	//awsGlacier.Config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries | aws.LogDebugWithSigning)
}

func sendArchive(archive *os.File, awsAccountID, awsVaultName string) (awsResult, error) {
	archiveInfo, err := archive.Stat()
	if err != nil {
		return awsResult{}, fmt.Errorf("error retrieving archive information. details: %s", err)
	}

	if archiveInfo.Size() <= 1024100 {
		return sendSmallArchive(archive, awsAccountID, awsVaultName)
	}

	return sendBigArchive(archive, archiveInfo.Size(), awsAccountID, awsVaultName)
}

func sendSmallArchive(archive *os.File, awsAccountID, awsVaultName string) (awsResult, error) {
	result := awsResult{
		time: time.Now(),
	}

	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(archive)

	awsArchive := glacier.UploadArchiveInput{
		AccountId:          aws.String(awsAccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", result.time.Format(time.RFC3339))),
		Body:               archive,
		Checksum:           aws.String(hex.EncodeToString(hash.TreeHash)),
		VaultName:          aws.String(awsVaultName),
	}

	response, err := awsGlacier.UploadArchive(&awsArchive)
	if err != nil {
		return result, fmt.Errorf("error sending archive to aws glacier. details: %s", err)
	}

	result.location = *response.Location
	result.checksum = *response.Checksum
	return result, nil
}

func sendBigArchive(archive *os.File, archiveSize int64, awsAccountID, awsVaultName string) (awsResult, error) {
	result := awsResult{
		time: time.Now(),
	}

	awsInitiate := glacier.InitiateMultipartUploadInput{
		AccountId:          aws.String(awsAccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", result.time.Format(time.RFC3339))),
		PartSize:           aws.String(strconv.FormatInt(partSize, 10)),
		VaultName:          aws.String(awsVaultName),
	}

	awsInitiateResponse, err := awsGlacier.InitiateMultipartUpload(&awsInitiate)
	if err != nil {
		return result, fmt.Errorf("error initializing multipart upload. details: %s", err)
	}

	var offset int64
	var part = make([]byte, partSize)

	for offset = 0; offset < archiveSize; offset += partSize {
		n, err := archive.Read(part)
		if err != nil {
			return result, fmt.Errorf("error reading an archive part (%d). details: %s", offset, err)
		}

		body := bytes.NewReader(part[:n])
		hash := glacier.ComputeHashes(body)

		awsArchivePart := glacier.UploadMultipartPartInput{
			AccountId: aws.String(awsAccountID),
			Body:      body,
			Checksum:  aws.String(hex.EncodeToString(hash.TreeHash)),
			Range:     aws.String(fmt.Sprintf("%d-%d/%d", offset, offset+int64(n), archiveSize)),
			UploadId:  awsInitiateResponse.UploadId,
			VaultName: aws.String(awsVaultName),
		}

		if _, err := awsGlacier.UploadMultipartPart(&awsArchivePart); err != nil {
			return result, fmt.Errorf("error sending an archive part (%d). details: %s", offset, err)
		}
	}

	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(archive)

	awsComplete := glacier.CompleteMultipartUploadInput{
		AccountId:   aws.String(awsAccountID),
		ArchiveSize: aws.String(strconv.FormatInt(archiveSize, 10)),
		Checksum:    aws.String(hex.EncodeToString(hash.TreeHash)),
		UploadId:    awsInitiateResponse.UploadId,
		VaultName:   aws.String(awsVaultName),
	}

	awsCompleteResponse, err := awsGlacier.CompleteMultipartUpload(&awsComplete)
	if err != nil {
		return result, fmt.Errorf("error completing multipart upload. details: %s", err)
	}

	result.location = *awsCompleteResponse.Location
	result.checksum = *awsCompleteResponse.Checksum
	return result, nil
}
