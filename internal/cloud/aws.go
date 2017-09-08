package cloud

import (
	"bytes"
	"context"
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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/aws/aws-sdk-go/service/glacier/glacieriface"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
)

var multipartUploadLimit int64 = 104857600 // 100 MB in bytes

// MultipartUploadLimit defines the limit where we decide if we will send the
// file in one shot or if we will use multipart upload strategy. By default we
// use 100 MB.
func MultipartUploadLimit(value int64) {
	atomic.StoreInt64(&multipartUploadLimit, value)
}

var partSize int64 = 4194304 // 4 MB (in bytes) will limit the archive in 40GB

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

// AWSConfig stores all necessary parameters to initialize a AWS session.
type AWSConfig struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	VaultName       string
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

// jobResult contains the result data after a archive download. It is used in
// channels for parallel downloads.
type jobResult struct {
	id       string
	filename string
	err      error
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
func NewAWSCloud(logger log.Logger, config AWSConfig, debug bool) (*AWSCloud, error) {
	var err error

	// this environment variables are used by the AWS library, so we need to set
	// them in plain text
	os.Setenv("AWS_ACCESS_KEY_ID", config.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", config.SecretAccessKey)
	os.Setenv("AWS_REGION", config.Region)

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
		AccountID: config.AccountID,
		VaultName: config.VaultName,
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
func (a *AWSCloud) Send(ctx context.Context, filename string) (Backup, error) {
	a.Logger.Debugf("cloud: sending file “%s” to aws cloud", filename)

	archive, err := os.Open(filename)
	if err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeOpeningArchive, err))
	}

	archiveInfo, err := archive.Stat()
	if err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeArchiveInfo, err))
	}

	var backup Backup

	if archiveInfo.Size() <= multipartUploadLimit {
		a.Logger.Debugf("cloud: using small file strategy (%d)", archiveInfo.Size())
		backup, err = a.sendSmall(ctx, archive)

	} else {
		a.Logger.Debugf("cloud: using big file strategy (%d)", archiveInfo.Size())
		backup, err = a.sendBig(ctx, archive, archiveInfo.Size())
	}

	if err == nil {
		a.Logger.Infof("cloud: file “%s” sent successfully to the aws cloud", filename)
		backup.Size = archiveInfo.Size()
	}

	return backup, err
}

func (a *AWSCloud) sendSmall(ctx context.Context, archive io.ReadSeeker) (Backup, error) {
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

	archiveCreationOutput, err := a.Glacier.UploadArchiveWithContext(ctx, &uploadArchiveInput)
	if err != nil {
		return Backup{}, errors.WithStack(a.checkCancellation(newError("", ErrorCodeSendingArchive, err)))
	}

	if hex.EncodeToString(hash.TreeHash) != *archiveCreationOutput.Checksum {
		a.Logger.Debugf("cloud: local archive checksum (%s) different from remote checksum (%s)", hex.EncodeToString(hash.TreeHash), *archiveCreationOutput.Checksum)
		return Backup{}, errors.WithStack(newError("", ErrorCodeComparingChecksums, nil))
	}

	locationParts := strings.Split(*archiveCreationOutput.Location, "/")
	backup.ID = locationParts[len(locationParts)-1]
	backup.Checksum = *archiveCreationOutput.Checksum
	backup.VaultName = a.VaultName

	return backup, nil
}

func (a *AWSCloud) sendBig(ctx context.Context, archive io.ReadSeeker, archiveSize int64) (Backup, error) {
	backup := Backup{
		CreatedAt: a.Clock.Now(),
	}

	initiateMultipartUploadInput := glacier.InitiateMultipartUploadInput{
		AccountId:          aws.String(a.AccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", backup.CreatedAt.Format(time.RFC3339))),
		PartSize:           aws.String(strconv.FormatInt(partSize, 10)),
		VaultName:          aws.String(a.VaultName),
	}

	initiateMultipartUploadOutput, err := a.Glacier.InitiateMultipartUploadWithContext(ctx, &initiateMultipartUploadInput)
	if err != nil {
		return Backup{}, errors.WithStack(a.checkCancellation(newError("", ErrorCodeInitMultipart, err)))
	}

	var offset int64
	var part = make([]byte, partSize)

	for offset = 0; offset < archiveSize; offset += partSize {
		a.Logger.Debugf("cloud: sending part %d/%d", offset, archiveSize)

		var n int
		if n, err = archive.Read(part); err != nil {
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

		var uploadMultipartPartOutput *glacier.UploadMultipartPartOutput
		if uploadMultipartPartOutput, err = a.Glacier.UploadMultipartPartWithContext(ctx, &uploadMultipartPartInput); err != nil {
			abortMultipartUploadInput := glacier.AbortMultipartUploadInput{
				AccountId: aws.String(a.AccountID),
				UploadId:  initiateMultipartUploadOutput.UploadId,
				VaultName: aws.String(a.VaultName),
			}

			a.Glacier.AbortMultipartUploadWithContext(ctx, &abortMultipartUploadInput)
			return Backup{}, errors.WithStack(a.checkCancellation(newMultipartError(offset, archiveSize, MultipartErrorCodeSendingArchive, err)))
		}

		// verify checksum of each uploaded part
		if *uploadMultipartPartOutput.Checksum != hex.EncodeToString(hash.TreeHash) {
			a.Logger.Debugf("cloud: local archive part %d/%d checksum (%s) different from remote checksum (%s)", offset, archiveSize, hex.EncodeToString(hash.TreeHash), *uploadMultipartPartOutput.Checksum)

			abortMultipartUploadInput := glacier.AbortMultipartUploadInput{
				AccountId: aws.String(a.AccountID),
				UploadId:  initiateMultipartUploadOutput.UploadId,
				VaultName: aws.String(a.VaultName),
			}

			a.Glacier.AbortMultipartUploadWithContext(ctx, &abortMultipartUploadInput)
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

	archiveCreationOutput, err := a.Glacier.CompleteMultipartUploadWithContext(ctx, &completeMultipartUploadInput)
	if err != nil {
		abortMultipartUploadInput := glacier.AbortMultipartUploadInput{
			AccountId: aws.String(a.AccountID),
			UploadId:  initiateMultipartUploadOutput.UploadId,
			VaultName: aws.String(a.VaultName),
		}

		a.Glacier.AbortMultipartUploadWithContext(ctx, &abortMultipartUploadInput)
		return Backup{}, errors.WithStack(a.checkCancellation(newError(*initiateMultipartUploadOutput.UploadId, ErrorCodeCompleteMultipart, err)))
	}

	locationParts := strings.Split(*archiveCreationOutput.Location, "/")
	backup.ID = locationParts[len(locationParts)-1]
	backup.Checksum = *archiveCreationOutput.Checksum
	backup.VaultName = a.VaultName

	if hex.EncodeToString(hash.TreeHash) != *archiveCreationOutput.Checksum {
		a.Logger.Debugf("cloud: local archive checksum (%s) different from remote checksum (%s)", hex.EncodeToString(hash.TreeHash), *archiveCreationOutput.Checksum)

		// something went wrong with the uploaded archive, better remove it
		if err := a.Remove(ctx, backup.ID); err != nil {
			// error while trying to remove the strange backup
			return backup, errors.WithStack(newError(backup.ID, ErrorCodeComparingChecksums, err))
		}

		// strange backup was removed from the cloud
		return backup, errors.WithStack(newError(backup.ID, ErrorCodeComparingChecksums, nil))
	}

	return backup, nil
}

// List retrieves all the uploaded backups information in the cloud. If an error
// occurs it will be an Error or JobsError type encapsulated in a traceable
// error. To retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *cloud.Error:
//         // handle specifically
//       case *cloud.JobsError:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AWSCloud) List(ctx context.Context) ([]Backup, error) {
	a.Logger.Debug("cloud: retrieving list of archives from the aws cloud")

	initiateJobInput := glacier.InitiateJobInput{
		AccountId: aws.String(a.AccountID),
		JobParameters: &glacier.JobParameters{
			Format: aws.String("JSON"),
			Type:   aws.String("inventory-retrieval"),
		},
		VaultName: aws.String(a.VaultName),
	}

	initiateJobOutput, err := a.Glacier.InitiateJobWithContext(ctx, &initiateJobInput)
	if err != nil {
		return nil, errors.WithStack(a.checkCancellation(newError("", ErrorCodeInitJob, err)))
	}

	if err = a.waitJobs(ctx, *initiateJobOutput.JobId); err != nil {
		return nil, errors.WithStack(err)
	}

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.AccountID),
		JobId:     initiateJobOutput.JobId,
		VaultName: aws.String(a.VaultName),
	}

	jobOutputOutput, err := a.Glacier.GetJobOutputWithContext(ctx, &jobOutputInput)
	if err != nil {
		return nil, errors.WithStack(a.checkCancellation(newError(*initiateJobOutput.JobId, ErrorCodeJobComplete, err)))
	}
	defer jobOutputOutput.Body.Close()

	// http://docs.aws.amazon.com/amazonglacier/latest/dev/api-job-output-get.html#api-job-output-get-responses-elements
	inventory := struct {
		VaultARN      string `json:"VaultARN"`
		InventoryDate string `json:"InventoryDate"`
		ArchiveList   AWSInventoryArchiveList
	}{}

	jsonDecoder := json.NewDecoder(jobOutputOutput.Body)
	if err := jsonDecoder.Decode(&inventory); err != nil {
		return nil, errors.WithStack(newError(*initiateJobOutput.JobId, ErrorCodeDecodingData, err))
	}

	sort.Sort(inventory.ArchiveList)

	var backups []Backup
	for _, archive := range inventory.ArchiveList {
		backups = append(backups, Backup{
			ID:        archive.ArchiveID,
			CreatedAt: archive.CreationDate,
			Checksum:  archive.SHA256TreeHash,
			VaultName: a.VaultName,
			Size:      int64(archive.Size),
		})
	}

	a.Logger.Info("cloud: remote backups listed successfully from the aws cloud")
	return backups, nil
}

// Get retrieves a specific backup file and stores it locally in a file. The
// filename storing the location of the file is returned.  If an error occurs it
// will be an Error or JobsError type encapsulated in a traceable error. To
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
//       case *cloud.JobsError:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *AWSCloud) Get(ctx context.Context, ids ...string) (map[string]string, error) {
	a.Logger.Debugf("cloud: retrieving archives “%v” from the aws cloud", ids)

	jobIDs := make(map[string]string)

	for _, id := range ids {
		initiateJobInput := glacier.InitiateJobInput{
			AccountId: aws.String(a.AccountID),
			JobParameters: &glacier.JobParameters{
				ArchiveId: aws.String(id),
				Type:      aws.String("archive-retrieval"),
			},
			VaultName: aws.String(a.VaultName),
		}

		initiateJobOutput, err := a.Glacier.InitiateJobWithContext(ctx, &initiateJobInput)
		if err != nil {
			return nil, errors.WithStack(a.checkCancellation(newError(id, ErrorCodeInitJob, err)))
		}

		jobIDs[id] = *initiateJobOutput.JobId
	}

	jobs := make([]string, 0, len(jobIDs))
	for _, job := range jobIDs {
		jobs = append(jobs, job)
	}

	if err := a.waitJobs(ctx, jobs...); err != nil {
		return nil, errors.WithStack(err)
	}

	var waitGroup sync.WaitGroup
	jobResults := make(chan jobResult, len(jobIDs))

	for id, jobID := range jobIDs {
		waitGroup.Add(1)
		go a.get(ctx, id, jobID, &waitGroup, jobResults)
	}

	waitGroup.Wait()

	filenames := make(map[string]string)
	for i := 0; i < len(jobIDs); i++ {
		result := <-jobResults
		if result.err != nil {
			// TODO: if only one file failed we will stop it all?
			return nil, errors.WithStack(result.err)
		}
		filenames[result.id] = result.filename
	}
	return filenames, nil
}

func (a *AWSCloud) get(ctx context.Context, id, jobID string, waitGroup *sync.WaitGroup, result chan<- jobResult) {
	defer waitGroup.Done()

	jobOutputInput := glacier.GetJobOutputInput{
		AccountId: aws.String(a.AccountID),
		JobId:     aws.String(jobID),
		VaultName: aws.String(a.VaultName),
	}

	jobOutputOutput, err := a.Glacier.GetJobOutputWithContext(ctx, &jobOutputInput)
	if err != nil {
		result <- jobResult{
			id:  id,
			err: errors.WithStack(a.checkCancellation(newError(id, ErrorCodeJobComplete, err))),
		}
		return
	}
	defer jobOutputOutput.Body.Close()

	backup, err := os.Create(path.Join(os.TempDir(), "backup-"+id+".tar"))
	if err != nil {
		result <- jobResult{
			id:  id,
			err: errors.WithStack(newError(id, ErrorCodeCreatingArchive, err)),
		}
		return
	}
	defer backup.Close()

	if _, err := io.Copy(backup, jobOutputOutput.Body); err != nil {
		result <- jobResult{
			id:  id,
			err: errors.WithStack(newError(id, ErrorCodeCopyingData, err)),
		}
		return
	}

	a.Logger.Infof("cloud: backup “%s” retrieved successfully from the aws cloud and saved in temporary file “%s”", id, backup.Name())

	result <- jobResult{
		id:       id,
		filename: backup.Name(),
	}
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
func (a *AWSCloud) Remove(ctx context.Context, id string) error {
	a.Logger.Debugf("cloud: removing archive %s from the aws cloud", id)

	deleteArchiveInput := glacier.DeleteArchiveInput{
		AccountId: aws.String(a.AccountID),
		ArchiveId: aws.String(id),
		VaultName: aws.String(a.VaultName),
	}

	if _, err := a.Glacier.DeleteArchiveWithContext(ctx, &deleteArchiveInput); err != nil {
		return errors.WithStack(a.checkCancellation(newError(id, ErrorCodeRemovingArchive, err)))
	}

	a.Logger.Infof("cloud: backup “%s” removed successfully from the aws cloud", id)
	return nil
}

// Close ends the AWS session. As there's nothing to close here, this will not
// perform any action.
func (a *AWSCloud) Close() error {
	return nil
}

func (a *AWSCloud) waitJobs(ctx context.Context, jobs ...string) error {
	sort.Strings(jobs)
	a.Logger.Debugf("cloud: waiting for jobs %v", jobs)

	waitJobTime.RLock()
	sleep := waitJobTime.Duration
	waitJobTime.RUnlock()

	for {
		listJobsInput := glacier.ListJobsInput{
			AccountId: aws.String(a.AccountID),
			VaultName: aws.String(a.VaultName),
		}

		listJobsOutput, err := a.Glacier.ListJobsWithContext(ctx, &listJobsInput)
		if err != nil {
			return errors.WithStack(a.checkCancellation(newJobsError(jobs, JobsErrorCodeRetrievingJob, err)))
		}

		jobsRemaining := make([]string, len(jobs))
		copy(jobsRemaining, jobs)

		a.Logger.Debugf("cloud: received jobs list response, will look for jobs %v", jobs)

		for _, jobDescription := range listJobsOutput.JobList {
			a.Logger.Debugf("cloud: job %s returned from cloud", *jobDescription.JobId)

			var i int
			if i = sort.SearchStrings(jobs, *jobDescription.JobId); i >= len(jobs) || jobs[i] != *jobDescription.JobId {
				a.Logger.Debugf("cloud: job %s was not expected", *jobDescription.JobId)
				continue
			}

			// check-out job in result list
			if j := sort.SearchStrings(jobsRemaining, *jobDescription.JobId); j < len(jobsRemaining) && jobsRemaining[j] == *jobDescription.JobId {
				jobsRemaining = append(jobsRemaining[:j], jobsRemaining[j+1:]...)
				a.Logger.Debugf("cloud: remaining jobs to look for %v", jobsRemaining)
			}

			if !*jobDescription.Completed {
				a.Logger.Debugf("cloud: job %s not completed yet", *jobDescription.JobId)
				continue
			}

			if *jobDescription.StatusCode == "Succeeded" {
				// remove the job that already succeeded
				jobs = append(jobs[:i], jobs[i+1:]...)
				a.Logger.Debugf("cloud: job %s succeeded, still need to proccess jobs %v", *jobDescription.JobId, jobs)

			} else if *jobDescription.StatusCode == "Failed" {
				return errors.WithStack(newError(*jobDescription.JobId, ErrorCodeJobFailed, errors.New(*jobDescription.StatusMessage)))
			}
		}

		if len(jobsRemaining) > 0 {
			return errors.WithStack(newJobsError(jobsRemaining, JobsErrorCodeJobNotFound, nil))
		}

		if len(jobs) == 0 {
			a.Logger.Debug("cloud: all jobs processed")
			break
		}

		a.Logger.Debugf("cloud: jobs %v not done, waiting %s for next check", jobs, sleep.String())

		select {
		case <-time.After(sleep):
			continue
		case <-ctx.Done():
			a.Logger.Debugf("cloud: jobs %v cancelled by user", jobs)
			return errors.WithStack(newJobsError(jobs, JobsErrorCodeCancelled, ctx.Err()))
		}
	}

	return nil
}

func (a *AWSCloud) checkCancellation(err error) error {
	switch v := err.(type) {
	case *Error:
		awsErr, ok := errors.Cause(v.Err).(awserr.Error)
		cancellation := ok && awsErr.Code() == request.CanceledErrorCode
		if cancellation {
			a.Logger.Debug("operation cancelled by user")
			return newError(v.ID, ErrorCodeCancelled, v.Err)
		}

	case *MultipartError:
		awsErr, ok := errors.Cause(v.Err).(awserr.Error)
		cancellation := ok && awsErr.Code() == request.CanceledErrorCode
		if cancellation {
			a.Logger.Debug("operation cancelled by user")
			return newMultipartError(v.Offset, v.Size, MultipartErrorCodeCancelled, v.Err)
		}

	case *JobsError:
		awsErr, ok := errors.Cause(v.Err).(awserr.Error)
		cancellation := ok && awsErr.Code() == request.CanceledErrorCode
		if cancellation {
			a.Logger.Debug("operation cancelled by user")
			return newJobsError(v.Jobs, JobsErrorCodeCancelled, v.Err)
		}
	}

	return err
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
