package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
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
	time      time.Time
	archiveID string
	checksum  string
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

	// Uncomment the lines bellow to understand what is going on
	// if g, ok := awsGlacier.(*glacier.Glacier); ok {
	// 	g.Config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries | aws.LogDebugWithSigning)
	// }
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

	if hex.EncodeToString(hash.LinearHash) != *response.Checksum {
		return result, fmt.Errorf("error comparing checksums")
	}

	locationParts := strings.Split(*response.Location, "/")
	result.archiveID = locationParts[len(locationParts)-1]
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

	if hex.EncodeToString(hash.LinearHash) != *awsCompleteResponse.Checksum {
		return result, fmt.Errorf("error comparing checksums")
	}

	locationParts := strings.Split(*awsCompleteResponse.Location, "/")
	result.archiveID = locationParts[len(locationParts)-1]
	result.checksum = *awsCompleteResponse.Checksum
	return result, nil
}

func listArchives(awsAccountID, awsVaultName string) ([]awsResult, error) {
	initiateJobInput := glacier.InitiateJobInput{
		AccountId: aws.String(awsAccountID),
		JobParameters: &glacier.JobParameters{
			Format: aws.String("JSON"),
			Type:   aws.String("inventory-retrieval"),
		},
		VaultName: aws.String(awsVaultName),
	}

	initiateJobOutput, err := awsGlacier.InitiateJob(&initiateJobInput)
	if err != nil {
		return nil, fmt.Errorf("error initiating the job. details: %s", err)
	}

waitJob:
	for {
		listJobsInput := glacier.ListJobsInput{
			AccountId: aws.String(awsAccountID),
			VaultName: aws.String(awsVaultName),
		}

		listJobsOutput, err := awsGlacier.ListJobs(&listJobsInput)
		if err != nil {
			return nil, fmt.Errorf("error retrieving the job from aws. details: %s", err)
		}

		jobFound := false
		for _, jobDescription := range listJobsOutput.JobList {
			if *jobDescription.JobId == *initiateJobOutput.JobId {
				jobFound = true

				if *jobDescription.Completed {
					if *jobDescription.StatusCode == "Succeeded" {
						break waitJob
					} else if *jobDescription.StatusCode == "Failed" {
						return nil, fmt.Errorf("error retrieving the job from aws. details: %s", *jobDescription.StatusMessage)
					}
				}

				break
			}
		}

		if !jobFound {
			return nil, fmt.Errorf("job not found in aws")
		}

		// Wait for the job to complete, as it takes some time, we will sleep for a
		// long time before we check again
		time.Sleep(1 * time.Minute)
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(awsAccountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(awsVaultName),
	}

	jobOutputOutput, err := awsGlacier.GetJobOutput(&jobOutputInput)
	if err != nil {
		return nil, fmt.Errorf("error retrieving the job information. details: %s", err)
	}
	defer jobOutputOutput.Body.Close()

	// http://docs.aws.amazon.com/amazonglacier/latest/dev/api-job-output-get.html#api-job-output-get-responses-elements
	iventory := struct {
		VaultARN      string `json:"VaultARN"`
		InventoryDate string `json:"InventoryDate"`
		ArchiveList   awsIventoryArchiveList
	}{}

	jsonDecoder := json.NewDecoder(jobOutputOutput.Body)
	if err := jsonDecoder.Decode(&iventory); err != nil {
		return nil, fmt.Errorf("error decoding the iventory. details: %s", err)
	}

	sort.Sort(iventory.ArchiveList)

	var archives []awsResult
	for _, archive := range iventory.ArchiveList {
		archives = append(archives, awsResult{
			time:      archive.CreationDate,
			archiveID: archive.ArchiveID,
			checksum:  archive.SHA256TreeHash,
		})
	}
	return archives, nil
}

func removeArchive(awsAccountID, awsVaultName, archiveID string) error {
	deleteArchiveInput := glacier.DeleteArchiveInput{
		AccountId: aws.String(awsAccountID),
		ArchiveId: aws.String(archiveID),
		VaultName: aws.String(awsVaultName),
	}

	if _, err := awsGlacier.DeleteArchive(&deleteArchiveInput); err != nil {
		return fmt.Errorf("error removing old backup. details: %s", err)
	}

	return nil
}

// removeOldArchives keep only the most 10 recent backups in the AWS Glacier
// service. This is useful to save money in used storage.
func removeOldArchives(awsAccountID, awsVaultName string, keepBackups int) error {
	archives, err := listArchives(awsAccountID, awsVaultName)
	if err != nil {
		return fmt.Errorf("error retrieving remote backups. details: %s", err)
	}

	for i := keepBackups; i < len(archives); i++ {
		if err := removeArchive(awsAccountID, awsVaultName, archives[i].archiveID); err != nil {
			return err
		}
	}

	return nil
}

type awsIventoryArchiveList []struct {
	ArchiveID          string    `json:"ArchiveId"`
	ArchiveDescription string    `json:"ArchiveDescription"`
	CreationDate       time.Time `json:"CreationDate"`
	Size               int       `json:"Size"`
	SHA256TreeHash     string    `json:"SHA256TreeHash"`
}

func (a awsIventoryArchiveList) Len() int {
	return len(a)
}

func (a awsIventoryArchiveList) Less(i, j int) bool {
	return a[i].CreationDate.Before(a[j].CreationDate)
}

func (a awsIventoryArchiveList) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
