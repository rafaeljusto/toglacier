package cloud

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/aws/aws-sdk-go/service/glacier/glacieriface"
	"github.com/rafaeljusto/toglacier/internal/config"
)

var multipartUploadLimit int64 = 102400 // 100 MB

// MultipartUploadLimit defines the limit where we decide if we will send the
// file in one shot or if we will use multipart upload strategy. By default we
// use 100 MB.
func MultipartUploadLimit(value int64) {
	atomic.StoreInt64(&multipartUploadLimit, value)
}

var partSize int64 = 4096 // 4 MB will limit the archive in 40GB

// PartSize the size of each part of the multipart upload except the last, in
// bytes. The last part can be smaller than this part size. By default we use
// 4MB.
func PartSize(value int64) {
	atomic.StoreInt64(&partSize, value)
}

var waitJobTime = struct {
	time.Duration
	sync.RWMutex
}{
	Duration: time.Minute,
}

// WaitJobTime is the amount of time that we wait for the job to complete, as it
// takes some time, we will sleep for a long time before we check again. By
// default we use 1 minute.
func WaitJobTime(value time.Duration) {
	waitJobTime.Lock()
	defer waitJobTime.Unlock()
	waitJobTime.Duration = value
}

// RandomSource defines from where we are going to read random values to encrypt
// the archives.
var RandomSource = rand.Reader

// encryptedLabel is used to identify if an archive was encrypted or not.
const encryptedLabel = "encrypted:"

// AWSCloud is the Amazon solution for storing the backups in the cloud. It uses
// the Amazon Glacier service, as it allows large files for a small price.
type AWSCloud struct {
	BackupSecret string
	AccountID    string
	VaultName    string
	Glacier      glacieriface.GlacierAPI
	Clock        Clock
}

// NewAWSCloud initializes the Amazon cloud object, defining the account ID and
// vault name that are going to be used in the AWS Glacier service. For more
// details set the debug flag to receive low level information in the standard
// output.
func NewAWSCloud(c *config.Config, debug bool) (*AWSCloud, error) {
	var err error

	// this environment variables are used by the AWS library, so we need to set
	// them in plain text
	os.Setenv("AWS_ACCESS_KEY_ID", c.AWS.AccessKeyID.Value)
	os.Setenv("AWS_SECRET_ACCESS_KEY", c.AWS.SecretAccessKey.Value)
	os.Setenv("AWS_REGION", c.AWS.Region)

	awsSession, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("error initializing aws session. details: %s", err)
	}

	awsGlacier := glacier.New(awsSession)
	if debug {
		awsGlacier.Config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries | aws.LogDebugWithSigning)
	}

	// The key argument should be the AES key, either 16, 24, or 32 bytes to
	// select AES-128, AES-192, or AES-256. We will always force AES-256.
	encryptSecret := c.BackupSecret.Value
	if encryptSecret != "" {
		if len(encryptSecret) < 32 {
			encryptSecret = encryptSecret + strings.Repeat("0", 32-len(encryptSecret))
		} else if len(encryptSecret) > 32 {
			encryptSecret = encryptSecret[:32]
		}
	}

	return &AWSCloud{
		BackupSecret: encryptSecret,
		AccountID:    c.AWS.AccountID.Value,
		VaultName:    c.AWS.VaultName,
		Glacier:      awsGlacier,
		Clock:        realClock{},
	}, nil
}

// Send uploads the file to the cloud and return the backup archive information.
// It already has the logic to send directly if it's a small file or use
// multipart strategy if it's a large file.
func (a *AWSCloud) Send(filename string) (Backup, error) {
	if a.BackupSecret != "" {
		var err error
		if filename, err = encrypt(filename, a.BackupSecret); err != nil {
			return Backup{}, fmt.Errorf("error encrypting archive. details: %s", err)
		}

		defer func() {
			// this function is responsible for removing the encrypted file after
			// uploading it to the cloud
			os.Remove(filename)
		}()
	}

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
		CreatedAt: a.Clock.Now(),
	}

	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(archive)

	uploadArchiveInput := glacier.UploadArchiveInput{
		AccountId:          aws.String(a.AccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", backup.CreatedAt.Format(time.RFC3339))),
		Body:               archive,
		Checksum:           aws.String(hex.EncodeToString(hash.TreeHash)),
		VaultName:          aws.String(a.VaultName),
	}

	archiveCreationOutput, err := a.Glacier.UploadArchive(&uploadArchiveInput)
	if err != nil {
		return Backup{}, fmt.Errorf("error sending archive to aws glacier. details: %s", err)
	}

	if hex.EncodeToString(hash.LinearHash) != *archiveCreationOutput.Checksum {
		return Backup{}, errors.New("error comparing checksums")
	}

	locationParts := strings.Split(*archiveCreationOutput.Location, "/")
	backup.ID = locationParts[len(locationParts)-1]
	backup.Checksum = *archiveCreationOutput.Checksum
	backup.VaultName = a.VaultName
	return backup, nil
}

func (a *AWSCloud) sendBig(archive *os.File, archiveSize int64) (Backup, error) {
	backup := Backup{
		CreatedAt: a.Clock.Now(),
	}

	initiateMultipartUploadInput := glacier.InitiateMultipartUploadInput{
		AccountId:          aws.String(a.AccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", backup.CreatedAt.Format(time.RFC3339))),
		PartSize:           aws.String(strconv.FormatInt(partSize, 10)),
		VaultName:          aws.String(a.VaultName),
	}

	initiateMultipartUploadOutput, err := a.Glacier.InitiateMultipartUpload(&initiateMultipartUploadInput)
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

		uploadMultipartPartInput := glacier.UploadMultipartPartInput{
			AccountId: aws.String(a.AccountID),
			Body:      body,
			Checksum:  aws.String(hex.EncodeToString(hash.TreeHash)),
			Range:     aws.String(fmt.Sprintf("%d-%d/%d", offset, offset+int64(n), archiveSize)),
			UploadId:  initiateMultipartUploadOutput.UploadId,
			VaultName: aws.String(a.VaultName),
		}

		if _, err := a.Glacier.UploadMultipartPart(&uploadMultipartPartInput); err != nil {
			return Backup{}, fmt.Errorf("error sending an archive part (%d). details: %s", offset, err)
		}

		// TODO: Verify checksum of each uploaded part
	}

	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(archive)

	completeMultipartUploadInput := glacier.CompleteMultipartUploadInput{
		AccountId:   aws.String(a.AccountID),
		ArchiveSize: aws.String(strconv.FormatInt(archiveSize, 10)),
		Checksum:    aws.String(hex.EncodeToString(hash.TreeHash)),
		UploadId:    initiateMultipartUploadOutput.UploadId,
		VaultName:   aws.String(a.VaultName),
	}

	archiveCreationOutput, err := a.Glacier.CompleteMultipartUpload(&completeMultipartUploadInput)
	if err != nil {
		return Backup{}, fmt.Errorf("error completing multipart upload. details: %s", err)
	}

	if hex.EncodeToString(hash.LinearHash) != *archiveCreationOutput.Checksum {
		return Backup{}, errors.New("error comparing checksums")
	}

	locationParts := strings.Split(*archiveCreationOutput.Location, "/")
	backup.ID = locationParts[len(locationParts)-1]
	backup.Checksum = *archiveCreationOutput.Checksum
	backup.VaultName = a.VaultName
	return backup, nil
}

// List retrieves all the uploaded backups information in the cloud.
func (a *AWSCloud) List() ([]Backup, error) {
	initiateJobInput := glacier.InitiateJobInput{
		AccountId: aws.String(a.AccountID),
		JobParameters: &glacier.JobParameters{
			Format: aws.String("JSON"),
			Type:   aws.String("inventory-retrieval"),
		},
		VaultName: aws.String(a.VaultName),
	}

	initiateJobOutput, err := a.Glacier.InitiateJob(&initiateJobInput)
	if err != nil {
		return nil, fmt.Errorf("error initiating the job. details: %s", err)
	}

	if err := a.waitJob(*initiateJobOutput.JobId); err != nil {
		return nil, err
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.AccountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(a.VaultName),
	}

	jobOutputOutput, err := a.Glacier.GetJobOutput(&jobOutputInput)
	if err != nil {
		return nil, fmt.Errorf("error retrieving the job information. details: %s", err)
	}
	defer jobOutputOutput.Body.Close()

	// http://docs.aws.amazon.com/amazonglacier/latest/dev/api-job-output-get.html#api-job-output-get-responses-elements
	iventory := struct {
		VaultARN      string `json:"VaultARN"`
		InventoryDate string `json:"InventoryDate"`
		ArchiveList   AWSInventoryArchiveList
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
			VaultName: a.VaultName,
		})
	}
	return backups, nil
}

// Get retrieves a specific backup file and stores it locally in a file. The
// filename storing the location of the file is returned.
func (a *AWSCloud) Get(id string) (string, error) {
	initiateJobInput := glacier.InitiateJobInput{
		AccountId: aws.String(a.AccountID),
		JobParameters: &glacier.JobParameters{
			ArchiveId: aws.String(id),
			Type:      aws.String("archive-retrieval"),
		},
		VaultName: aws.String(a.VaultName),
	}

	initiateJobOutput, err := a.Glacier.InitiateJob(&initiateJobInput)
	if err != nil {
		return "", fmt.Errorf("error initiating the job. details: %s", err)
	}

	if err := a.waitJob(*initiateJobOutput.JobId); err != nil {
		return "", err
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.AccountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(a.VaultName),
	}

	jobOutputOutput, err := a.Glacier.GetJobOutput(&jobOutputInput)
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

	filename := backup.Name()

	if a.BackupSecret != "" {
		var decryptedFilename string
		if decryptedFilename, err = decrypt(filename, a.BackupSecret); err != nil {
			return "", fmt.Errorf("error decrypting archive. details: %s", err)
		}

		if err := os.Rename(decryptedFilename, filename); err != nil {
			return "", fmt.Errorf("error renaming decrypted archive. details: %s", err)
		}
	}

	return filename, nil
}

// Remove erase a specific backup from the cloud.
func (a *AWSCloud) Remove(id string) error {
	deleteArchiveInput := glacier.DeleteArchiveInput{
		AccountId: aws.String(a.AccountID),
		ArchiveId: aws.String(id),
		VaultName: aws.String(a.VaultName),
	}

	if _, err := a.Glacier.DeleteArchive(&deleteArchiveInput); err != nil {
		return fmt.Errorf("error removing old backup. details: %s", err)
	}

	return nil
}

func (a *AWSCloud) waitJob(jobID string) error {
	waitJobTime.RLock()
	sleep := waitJobTime.Duration
	waitJobTime.RUnlock()

	for {
		listJobsInput := glacier.ListJobsInput{
			AccountId: aws.String(a.AccountID),
			VaultName: aws.String(a.VaultName),
		}

		listJobsOutput, err := a.Glacier.ListJobs(&listJobsInput)
		if err != nil {
			return fmt.Errorf("error retrieving the job from aws. details: %s", err)
		}

		jobFound := false
		for _, jobDescription := range listJobsOutput.JobList {
			if *jobDescription.JobId != jobID {
				continue
			}

			jobFound = true

			if !*jobDescription.Completed {
				break
			}

			if *jobDescription.StatusCode == "Succeeded" {
				return nil
			} else if *jobDescription.StatusCode == "Failed" {
				return fmt.Errorf("error retrieving the job from aws. details: %s", *jobDescription.StatusMessage)
			}

			break
		}

		if !jobFound {
			return errors.New("job not found in aws")
		}

		time.Sleep(sleep)
	}
}

// encrypted do what we expect, encrypting the content with a shared secret. It
// add authentication using an HMAC-SHA256. It will return the encrypted
// filename or an error.
func encrypt(filename string, secret string) (string, error) {
	archive, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer archive.Close()

	encryptedArchive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", err
	}
	defer encryptedArchive.Close()

	hash := hmac.New(sha256.New, []byte(secret))
	if _, err = io.Copy(hash, archive); err != nil {
		return "", err
	}

	if _, err = archive.Seek(0, 0); err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = io.ReadFull(RandomSource, iv); err != nil {
		return "", err
	}

	if _, err = encryptedArchive.WriteString(encryptedLabel); err != nil {
		return "", err
	}

	if _, err = encryptedArchive.Write(hash.Sum(nil)); err != nil {
		return "", err
	}

	if _, err = encryptedArchive.Write(iv); err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", err
	}

	writer := cipher.StreamWriter{
		S: cipher.NewOFB(block, iv),
		W: encryptedArchive,
	}
	defer writer.Close()

	if _, err = io.Copy(&writer, archive); err != nil {
		return "", err
	}

	return encryptedArchive.Name(), nil
}

// decrypt do what we expect, decrypting the content with a shared secret. It
// authenticates the data using an HMAC-SHA256. It will return the decrypted
// filename or an error.
func decrypt(encryptedFilename string, secret string) (string, error) {
	encryptedArchive, err := os.Open(encryptedFilename)
	if err != nil {
		return "", err
	}
	defer encryptedArchive.Close()

	archive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", err
	}
	defer archive.Close()

	encryptedLabelBuffer := make([]byte, len(encryptedLabel))
	if _, err = encryptedArchive.Read(encryptedLabelBuffer); err == io.EOF || string(encryptedLabelBuffer) != encryptedLabel {
		// if we couldn't read the encrypted label, maybe the file isn't encrypted,
		// so let's return it as it is
		return encryptedFilename, nil

	} else if err != nil {
		return "", err
	}

	authHash := make([]byte, sha256.Size)
	if _, err = encryptedArchive.Read(authHash); err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err = encryptedArchive.Read(iv); err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", err
	}

	reader := cipher.StreamReader{
		S: cipher.NewOFB(block, iv),
		R: encryptedArchive,
	}

	if _, err = io.Copy(archive, reader); err != nil {
		return "", err
	}

	if _, err = archive.Seek(0, 0); err != nil {
		return "", err
	}

	hash := hmac.New(sha256.New, []byte(secret))
	if _, err = io.Copy(hash, archive); err != nil {
		return "", err
	}

	if !hmac.Equal(authHash, hash.Sum(nil)) {
		return "", errors.New("encrypted content authentication failed")
	}

	return archive.Name(), nil
}

// AWSInventoryArchiveList stores the archive information retrieved from AWS
// Glacier service.
type AWSInventoryArchiveList []struct {
	ArchiveID          string    `json:"ArchiveId"`
	ArchiveDescription string    `json:"ArchiveDescription"`
	CreationDate       time.Time `json:"CreationDate"`
	Size               int       `json:"Size"`
	SHA256TreeHash     string    `json:"SHA256TreeHash"`
}

func (a AWSInventoryArchiveList) Len() int {
	return len(a)
}

func (a AWSInventoryArchiveList) Less(i, j int) bool {
	return a[i].CreationDate.Before(a[j].CreationDate)
}

func (a AWSInventoryArchiveList) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
