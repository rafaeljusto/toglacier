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
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/aws/aws-sdk-go/service/glacier/glacieriface"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/log"
)

var multipartUploadLimit int64 = 102400 // 100 MB

// MultipartUploadLimit defines the limit where we decide if we will send the
// file in one shot or if we will use multipart upload strategy. By default we
// use 100 MB.
func MultipartUploadLimit(value int64) {
	atomic.StoreInt64(&multipartUploadLimit, value)
}

var partSize int64 = 4194304 // 4 MB will limit the archive in 40GB

// PartSize the size of each part of the multipart upload except the last, in
// bytes. The last part can be smaller than this part size. By default we use
// 4MB.
func PartSize(value int64) {
	// TODO: Part size must be a power of two and be between 1048576 and
	// 4294967296 bytes

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

// AWSCloud is the Amazon solution for storing the backups in the cloud. It uses
// the Amazon Glacier service, as it allows large files for a small price.
type AWSCloud struct {
	Logger    log.Logger
	AccountID string
	VaultName string
	Glacier   glacieriface.GlacierAPI
	Clock     Clock
}

// NewAWSCloud initializes the Amazon cloud object, defining the account ID and
// vault name that are going to be used in the AWS Glacier service. For more
// details set the debug flag to receive low level information in the standard
// output. On error it will return an Error type. To retrieve the desired error
// you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *cloud.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func NewAWSCloud(logger log.Logger, c *config.Config, debug bool) (*AWSCloud, error) {
	var err error

	// this environment variables are used by the AWS library, so we need to set
	// them in plain text
	os.Setenv("AWS_ACCESS_KEY_ID", c.AWS.AccessKeyID.Value)
	os.Setenv("AWS_SECRET_ACCESS_KEY", c.AWS.SecretAccessKey.Value)
	os.Setenv("AWS_REGION", c.AWS.Region)

	awsSession, err := session.NewSession()
	if err != nil {
		return nil, errors.WithStack(newError("", ErrorCodeInitializingSession, err))
	}

	awsGlacier := glacier.New(awsSession)
	if debug {
		awsGlacier.Config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries | aws.LogDebugWithSigning)
	}

	return &AWSCloud{
		Logger:    logger,
		AccountID: c.AWS.AccountID.Value,
		VaultName: c.AWS.VaultName,
		Glacier:   awsGlacier,
		Clock:     realClock{},
	}, nil
}

// Send uploads the file to the cloud and return the backup archive information.
// It already has the logic to send directly if it's a small file or use
// multipart strategy if it's a large file. If an error occurs it will be an
// Error or MultipartError type encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *cloud.Error:
//         // handle specifically
//       case *cloud.MultipartError:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AWSCloud) Send(filename string) (Backup, error) {
	a.Logger.Debugf("[cloud] sending file “%s” to aws cloud", filename)

	archive, err := os.Open(filename)
	if err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeOpeningArchive, err))
	}

	archiveInfo, err := archive.Stat()
	if err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeArchiveInfo, err))
	}

	if archiveInfo.Size() <= multipartUploadLimit {
		a.Logger.Debugf("[cloud] using small file strategy (%d)", archiveInfo.Size())
		return a.sendSmall(archive)
	}

	a.Logger.Debugf("[cloud] using big file strategy (%d)", archiveInfo.Size())
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
		return Backup{}, errors.WithStack(newError("", ErrorCodeSendingArchive, err))
	}

	if hex.EncodeToString(hash.TreeHash) != *archiveCreationOutput.Checksum {
		a.Logger.Debugf("[cloud] local archive checksum (%s) different from remote checksum (%s)", hex.EncodeToString(hash.TreeHash), *archiveCreationOutput.Checksum)
		return Backup{}, errors.WithStack(newError("", ErrorCodeComparingChecksums, nil))
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
		return Backup{}, errors.WithStack(newError("", ErrorCodeInitMultipart, err))
	}

	var offset int64
	var part = make([]byte, partSize)

	for offset = 0; offset < archiveSize; offset += partSize {
		a.Logger.Debugf("[cloud] sending part %d/%d", offset, archiveSize)

		n, err := archive.Read(part)
		if err != nil {
			return Backup{}, errors.WithStack(newMultipartError(offset, archiveSize, MultipartErrorCodeReadingArchive, err))
		}

		body := bytes.NewReader(part[:n])
		hash := glacier.ComputeHashes(body)

		uploadMultipartPartInput := glacier.UploadMultipartPartInput{
			AccountId: aws.String(a.AccountID),
			Body:      body,
			Checksum:  aws.String(hex.EncodeToString(hash.TreeHash)),
			Range:     aws.String(fmt.Sprintf("bytes %d-%d/%d", offset, offset+int64(n)-1, archiveSize)),
			UploadId:  initiateMultipartUploadOutput.UploadId,
			VaultName: aws.String(a.VaultName),
		}

		uploadMultipartPartOutput, err := a.Glacier.UploadMultipartPart(&uploadMultipartPartInput)
		if err != nil {
			abortMultipartUploadInput := glacier.AbortMultipartUploadInput{
				AccountId: aws.String(a.AccountID),
				UploadId:  initiateMultipartUploadOutput.UploadId,
				VaultName: aws.String(a.VaultName),
			}

			a.Glacier.AbortMultipartUpload(&abortMultipartUploadInput)
			return Backup{}, errors.WithStack(newMultipartError(offset, archiveSize, MultipartErrorCodeSendingArchive, err))
		}

		// verify checksum of each uploaded part
		if *uploadMultipartPartOutput.Checksum != hex.EncodeToString(hash.TreeHash) {
			a.Logger.Debugf("[cloud] local archive part %d/%d checksum (%s) different from remote checksum (%s)", offset, archiveSize, hex.EncodeToString(hash.TreeHash), *uploadMultipartPartOutput.Checksum)

			abortMultipartUploadInput := glacier.AbortMultipartUploadInput{
				AccountId: aws.String(a.AccountID),
				UploadId:  initiateMultipartUploadOutput.UploadId,
				VaultName: aws.String(a.VaultName),
			}

			a.Glacier.AbortMultipartUpload(&abortMultipartUploadInput)
			return Backup{}, errors.WithStack(newMultipartError(offset, archiveSize, MultipartErrorCodeComparingChecksums, err))
		}
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
		abortMultipartUploadInput := glacier.AbortMultipartUploadInput{
			AccountId: aws.String(a.AccountID),
			UploadId:  initiateMultipartUploadOutput.UploadId,
			VaultName: aws.String(a.VaultName),
		}

		a.Glacier.AbortMultipartUpload(&abortMultipartUploadInput)
		return Backup{}, errors.WithStack(newError(*initiateMultipartUploadOutput.UploadId, ErrorCodeCompleteMultipart, err))
	}

	locationParts := strings.Split(*archiveCreationOutput.Location, "/")
	backup.ID = locationParts[len(locationParts)-1]
	backup.Checksum = *archiveCreationOutput.Checksum
	backup.VaultName = a.VaultName

	if hex.EncodeToString(hash.TreeHash) != *archiveCreationOutput.Checksum {
		a.Logger.Debugf("[cloud] local archive checksum (%s) different from remote checksum (%s)", hex.EncodeToString(hash.TreeHash), *archiveCreationOutput.Checksum)

		// something went wrong with the uploaded archive, better remove it
		if err := a.Remove(backup.ID); err != nil {
			// error while trying to remove the strange backup
			return backup, errors.WithStack(newError(backup.ID, ErrorCodeComparingChecksums, err))
		}

		// strange backup was removed from the cloud
		return backup, errors.WithStack(newError(backup.ID, ErrorCodeComparingChecksums, nil))
	}

	return backup, nil
}

// List retrieves all the uploaded backups information in the cloud. If an error
// occurs it will be an Error type encapsulated in a traceable error. To
// retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *cloud.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AWSCloud) List() ([]Backup, error) {
	a.Logger.Debug("[cloud] retrieving list of archives from the aws cloud")

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
		return nil, errors.WithStack(newError("", ErrorCodeInitJob, err))
	}

	if err := a.waitJob(*initiateJobOutput.JobId); err != nil {
		return nil, errors.WithStack(err)
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.AccountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(a.VaultName),
	}

	jobOutputOutput, err := a.Glacier.GetJobOutput(&jobOutputInput)
	if err != nil {
		return nil, errors.WithStack(newError(*initiateJobOutput.JobId, ErrorCodeJobComplete, err))
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
		return nil, errors.WithStack(newError(*initiateJobOutput.JobId, ErrorCodeDecodingData, err))
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
// filename storing the location of the file is returned.  If an error occurs it
// will be an Error type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *cloud.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AWSCloud) Get(id string) (string, error) {
	a.Logger.Debugf("[cloud] retrieving archive %s from the aws cloud", id)

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
		return "", errors.WithStack(newError(id, ErrorCodeInitJob, err))
	}

	if err := a.waitJob(*initiateJobOutput.JobId); err != nil {
		return "", errors.WithStack(err)
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.AccountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(a.VaultName),
	}

	jobOutputOutput, err := a.Glacier.GetJobOutput(&jobOutputInput)
	if err != nil {
		return "", errors.WithStack(newError(*initiateJobOutput.JobId, ErrorCodeJobComplete, err))
	}
	defer jobOutputOutput.Body.Close()

	backup, err := os.Create(path.Join(os.TempDir(), "backup-"+id+".tar"))
	if err != nil {
		return "", errors.WithStack(newError(id, ErrorCodeCreatingArchive, err))
	}
	defer backup.Close()

	if _, err := io.Copy(backup, jobOutputOutput.Body); err != nil {
		return "", errors.WithStack(newError(id, ErrorCodeCopyingData, err))
	}

	return backup.Name(), nil
}

// Remove erase a specific backup from the cloud. If an error occurs it will be
// an Error type encapsulated in a traceable error. To retrieve the desired
// error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *cloud.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AWSCloud) Remove(id string) error {
	a.Logger.Debugf("[cloud] removing archive %s from the aws cloud", id)

	deleteArchiveInput := glacier.DeleteArchiveInput{
		AccountId: aws.String(a.AccountID),
		ArchiveId: aws.String(id),
		VaultName: aws.String(a.VaultName),
	}

	if _, err := a.Glacier.DeleteArchive(&deleteArchiveInput); err != nil {
		return errors.WithStack(newError(id, ErrorCodeRemovingArchive, err))
	}

	return nil
}

func (a *AWSCloud) waitJob(jobID string) error {
	a.Logger.Debugf("[cloud] waiting for job %s", jobID)

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
			return errors.WithStack(newError(jobID, ErrorCodeRetrievingJob, err))
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
				return errors.WithStack(newError(jobID, ErrorCodeJobFailed, errors.New(*jobDescription.StatusMessage)))
			}

			break
		}

		if !jobFound {
			return errors.WithStack(newError(jobID, ErrorCodeJobNotFound, nil))
		}

		a.Logger.Debugf("[cloud] job %s not done, waiting %s for next check", jobID, sleep.String())
		time.Sleep(sleep)
	}
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
