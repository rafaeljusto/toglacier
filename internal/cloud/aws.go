package cloud

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/aws/aws-sdk-go/service/glacier/glacieriface"
)

// multipartUploadLimit defines the limit where we decide if we will send the
// file in one shot or if we will use multipart upload strategy.
const multipartUploadLimit int64 = 102400 // 100 MB

// partSize the size of each part of the multipart upload except the last, in
// bytes. The last part can be smaller than this part size.
const partSize int64 = 4096 // 4 MB will limit the archive in 40GB

func NewAWSCloud(accountID, vaultName string, debug bool) (*AWSCloud, error) {
	var err error

	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	if strings.HasPrefix(awsAccessKeyID, "encrypted:") {
		awsAccessKeyID, err = passwordDecrypt(strings.TrimPrefix(awsAccessKeyID, "encrypted:"))
		if err != nil {
			return nil, fmt.Errorf("error decrypting aws access key id. details: %s", err)
		}
		// this environment variable is used by the AWS library, so we neede to set
		// it again in plain text
		os.Setenv("AWS_ACCESS_KEY_ID", awsAccessKeyID)
	}

	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if strings.HasPrefix(awsSecretAccessKey, "encrypted:") {
		awsSecretAccessKey, err = passwordDecrypt(strings.TrimPrefix(awsSecretAccessKey, "encrypted:"))
		if err != nil {
			return nil, fmt.Errorf("error decrypting aws secret access key. details: %s", err)
		}
		// this environment variable is used by the AWS library, so we neede to set
		// it again in plain text
		os.Setenv("AWS_SECRET_ACCESS_KEY", awsSecretAccessKey)
	}

	if strings.HasPrefix(accountID, "encrypted:") {
		accountID, err = passwordDecrypt(strings.TrimPrefix(accountID, "encrypted:"))
		if err != nil {
			return nil, fmt.Errorf("error decrypting aws account id. details: %s", err)
		}
	}

	awsSession, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("error initializing aws session. details: %s", err)
	}

	awsGlacier := glacier.New(awsSession)
	if debug {
		awsGlacier.Config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries | aws.LogDebugWithSigning)
	}

	return &AWSCloud{
		accountID: accountID,
		vaultName: vaultName,
		glacier:   awsGlacier,
	}, nil
}

type AWSCloud struct {
	accountID string
	vaultName string
	glacier   glacieriface.GlacierAPI
}

func (a *AWSCloud) Send(filename string) (Backup, error) {
	archive, err := os.Open(filename)
	if err != nil {
		return Backup{}, fmt.Errorf("error opening archive. details: %s", err)
	}

	archiveInfo, err := archive.Stat()
	if err != nil {
		return Backup{}, fmt.Errorf("error retrieving archive information. details: %s", err)
	}

	if archiveInfo.Size() <= multipartUploadLimit {
		return a.sendSmall(archive)
	}

	return a.sendBig(archive, archiveInfo.Size())
}

func (a *AWSCloud) sendSmall(archive *os.File) (Backup, error) {
	backup := Backup{
		CreatedAt: time.Now(),
	}

	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(archive)

	uploadArchiveInput := glacier.UploadArchiveInput{
		AccountId:          aws.String(a.accountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", backup.CreatedAt.Format(time.RFC3339))),
		Body:               archive,
		Checksum:           aws.String(hex.EncodeToString(hash.TreeHash)),
		VaultName:          aws.String(a.vaultName),
	}

	archiveCreationOutput, err := a.glacier.UploadArchive(&uploadArchiveInput)
	if err != nil {
		return Backup{}, fmt.Errorf("error sending archive to aws glacier. details: %s", err)
	}

	if hex.EncodeToString(hash.LinearHash) != *archiveCreationOutput.Checksum {
		return Backup{}, fmt.Errorf("error comparing checksums")
	}

	locationParts := strings.Split(*archiveCreationOutput.Location, "/")
	backup.ID = locationParts[len(locationParts)-1]
	backup.Checksum = *archiveCreationOutput.Checksum
	backup.VaultName = a.vaultName
	return backup, nil
}

func (a *AWSCloud) sendBig(archive *os.File, archiveSize int64) (Backup, error) {
	backup := Backup{
		CreatedAt: time.Now(),
	}

	initiateMultipartUploadInput := glacier.InitiateMultipartUploadInput{
		AccountId:          aws.String(a.accountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", backup.CreatedAt.Format(time.RFC3339))),
		PartSize:           aws.String(strconv.FormatInt(partSize, 10)),
		VaultName:          aws.String(a.vaultName),
	}

	initiateMultipartUploadOutput, err := a.glacier.InitiateMultipartUpload(&initiateMultipartUploadInput)
	if err != nil {
		return Backup{}, fmt.Errorf("error initializing multipart upload. details: %s", err)
	}

	var offset int64
	var part = make([]byte, partSize)

	for offset = 0; offset < archiveSize; offset += partSize {
		n, err := archive.Read(part)
		if err != nil {
			return Backup{}, fmt.Errorf("error reading an archive part (%d). details: %s", offset, err)
		}

		body := bytes.NewReader(part[:n])
		hash := glacier.ComputeHashes(body)

		awsArchivePart := glacier.UploadMultipartPartInput{
			AccountId: aws.String(a.accountID),
			Body:      body,
			Checksum:  aws.String(hex.EncodeToString(hash.TreeHash)),
			Range:     aws.String(fmt.Sprintf("%d-%d/%d", offset, offset+int64(n), archiveSize)),
			UploadId:  initiateMultipartUploadOutput.UploadId,
			VaultName: aws.String(a.vaultName),
		}

		if _, err := a.glacier.UploadMultipartPart(&awsArchivePart); err != nil {
			return Backup{}, fmt.Errorf("error sending an archive part (%d). details: %s", offset, err)
		}
	}

	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(archive)

	completeMultipartUploadInput := glacier.CompleteMultipartUploadInput{
		AccountId:   aws.String(a.accountID),
		ArchiveSize: aws.String(strconv.FormatInt(archiveSize, 10)),
		Checksum:    aws.String(hex.EncodeToString(hash.TreeHash)),
		UploadId:    initiateMultipartUploadOutput.UploadId,
		VaultName:   aws.String(a.vaultName),
	}

	archiveCreationOutput, err := a.glacier.CompleteMultipartUpload(&completeMultipartUploadInput)
	if err != nil {
		return Backup{}, fmt.Errorf("error completing multipart upload. details: %s", err)
	}

	if hex.EncodeToString(hash.LinearHash) != *archiveCreationOutput.Checksum {
		return Backup{}, fmt.Errorf("error comparing checksums")
	}

	locationParts := strings.Split(*archiveCreationOutput.Location, "/")
	backup.ID = locationParts[len(locationParts)-1]
	backup.Checksum = *archiveCreationOutput.Checksum
	backup.VaultName = a.vaultName
	return backup, nil
}

func (a *AWSCloud) List() ([]Backup, error) {
	initiateJobInput := glacier.InitiateJobInput{
		AccountId: aws.String(a.accountID),
		JobParameters: &glacier.JobParameters{
			Format: aws.String("JSON"),
			Type:   aws.String("inventory-retrieval"),
		},
		VaultName: aws.String(a.vaultName),
	}

	initiateJobOutput, err := a.glacier.InitiateJob(&initiateJobInput)
	if err != nil {
		return nil, fmt.Errorf("error initiating the job. details: %s", err)
	}

	if err := a.waitJob(*initiateJobOutput.JobId); err != nil {
		return nil, err
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.accountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(a.vaultName),
	}

	jobOutputOutput, err := a.glacier.GetJobOutput(&jobOutputInput)
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

	var backups []Backup
	for _, archive := range iventory.ArchiveList {
		backups = append(backups, Backup{
			ID:        archive.ArchiveID,
			CreatedAt: archive.CreationDate,
			Checksum:  archive.SHA256TreeHash,
			VaultName: a.vaultName,
		})
	}
	return backups, nil
}

func (a *AWSCloud) Get(id string) (filename string, err error) {
	initiateJobInput := glacier.InitiateJobInput{
		AccountId: aws.String(a.accountID),
		JobParameters: &glacier.JobParameters{
			ArchiveId: aws.String(id),
			Type:      aws.String("archive-retrieval"),
		},
		VaultName: aws.String(a.vaultName),
	}

	initiateJobOutput, err := a.glacier.InitiateJob(&initiateJobInput)
	if err != nil {
		return "", fmt.Errorf("error initiating the job. details: %s", err)
	}

	if err := a.waitJob(*initiateJobOutput.JobId); err != nil {
		return "", err
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.accountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(a.vaultName),
	}

	jobOutputOutput, err := a.glacier.GetJobOutput(&jobOutputInput)
	if err != nil {
		return "", fmt.Errorf("error retrieving the job information. details: %s", err)
	}
	defer jobOutputOutput.Body.Close()

	backup, err := os.Create(path.Join(os.TempDir(), "backup-"+id+".tar"))
	if err != nil {
		return "", fmt.Errorf("error creating backup file. details: %s", err)
	}
	defer backup.Close()

	if _, err := io.Copy(backup, jobOutputOutput.Body); err != nil {
		return "", fmt.Errorf("error copying data to the backup file. details: %s", err)
	}

	return backup.Name(), nil
}

func (a *AWSCloud) Remove(id string) error {
	deleteArchiveInput := glacier.DeleteArchiveInput{
		AccountId: aws.String(a.accountID),
		ArchiveId: aws.String(id),
		VaultName: aws.String(a.vaultName),
	}

	if _, err := a.glacier.DeleteArchive(&deleteArchiveInput); err != nil {
		return fmt.Errorf("error removing old backup. details: %s", err)
	}

	return nil
}

func (a *AWSCloud) waitJob(jobID string) error {
	for {
		listJobsInput := glacier.ListJobsInput{
			AccountId: aws.String(a.accountID),
			VaultName: aws.String(a.vaultName),
		}

		listJobsOutput, err := a.glacier.ListJobs(&listJobsInput)
		if err != nil {
			return fmt.Errorf("error retrieving the job from aws. details: %s", err)
		}

		jobFound := false
		for _, jobDescription := range listJobsOutput.JobList {
			if *jobDescription.JobId == jobID {
				jobFound = true

				if *jobDescription.Completed {
					if *jobDescription.StatusCode == "Succeeded" {
						return nil
					} else if *jobDescription.StatusCode == "Failed" {
						return fmt.Errorf("error retrieving the job from aws. details: %s", *jobDescription.StatusMessage)
					}
				}

				break
			}
		}

		if !jobFound {
			return fmt.Errorf("job not found in aws")
		}

		// Wait for the job to complete, as it takes some time, we will sleep for a
		// long time before we check again
		time.Sleep(1 * time.Minute)
	}
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
