package main

import (
	"archive/tar"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/glacier"
)

// partSize the size of each part of the multipart upload except the last, in
// bytes. The last part can be smaller than this part size.
const partSize int64 = 4096 // 4 MB will limit the archive in 40GB

func main() {
	archive, err := buildArchive(os.Getenv("TOGLACIER_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	defer archive.Close()

	archiveID, location, err := sendArchive(archive, os.Getenv("AWS_ACCOUNT_ID"), os.Getenv("AWS_REGION"), os.Getenv("AWS_VAULT_NAME"))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Archive ID: %s", archiveID)
	log.Printf("Location: %s", location)
}

func buildArchive(backupPath string) (*os.File, error) {
	tarFile, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return nil, fmt.Errorf("error creating the tar file. details: %s", err)
	}

	tarArchive := tar.NewWriter(tarFile)

	if err := buildArchiveLevels(tarArchive, backupPath); err != nil {
		tarFile.Close()
		return nil, err
	}

	if err := tarArchive.Close(); err != nil {
		tarFile.Close()
		return nil, fmt.Errorf("error generating tar file. details: %s", err)
	}

	return tarFile, nil
}

func buildArchiveLevels(tarArchive *tar.Writer, pathLevel string) error {
	files, err := ioutil.ReadDir(pathLevel)
	if err != nil {
		return fmt.Errorf("error reading path “%s”. details: %s", pathLevel, err)
	}

	for _, file := range files {
		if file.IsDir() {
			buildArchiveLevels(tarArchive, path.Join(pathLevel, file.Name()))
			continue
		}

		tarHeader := tar.Header{
			Name: file.Name(),
			Mode: 0600,
			Size: file.Size(),
		}

		if err := tarArchive.WriteHeader(&tarHeader); err != nil {
			return fmt.Errorf("error writing header in tar for file %s. details: %s", file.Name(), err)
		}

		filename := path.Join(pathLevel, file.Name())

		fd, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("error opening file %s. details: %s", filename, err)
		}

		if n, err := io.Copy(tarArchive, fd); err != nil {
			return fmt.Errorf("error writing content in tar for file %s. details: %s", filename, err)

		} else if n != file.Size() {
			return fmt.Errorf("wrong number of bytes writen in file %s", filename)
		}

		if err := fd.Close(); err != nil {
			return fmt.Errorf("error closing file %s. details: %s", filename, err)
		}
	}

	return nil
}

func sendArchive(archive *os.File, awsAccountID, awsRegion, awsVaultName string) (archiveID, location string, err error) {
	archiveInfo, err := archive.Stat()
	if err != nil {
		return "", "", fmt.Errorf("error retrieving archive information. details: %s", err)
	}

	if archiveInfo.Size() <= 1024100 {
		return sendSmallArchive(archive, awsAccountID, awsRegion, awsVaultName)
	}

	return sendBigArchive(archive, archiveInfo.Size(), awsAccountID, awsRegion, awsVaultName)
}

func sendSmallArchive(archive *os.File, awsAccountID, awsRegion, awsVaultName string) (archiveID, location string, err error) {
	// ComputeHashes already rewind the file seek at the beginning and at the end
	// of the function, so we don't need to wore about it
	hash := glacier.ComputeHashes(archive)

	awsArchive := glacier.UploadArchiveInput{
		AccountId:          aws.String(awsAccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", time.Now().Format(time.RFC3339))),
		Body:               archive,
		Checksum:           aws.String(hex.EncodeToString(hash.TreeHash)),
		VaultName:          aws.String(awsVaultName),
	}

	awsSession, err := session.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("error creating aws session. details: %s", err)
	}

	awsGlacier := glacier.New(awsSession, &aws.Config{
		Region: aws.String(awsRegion),
	})

	// Uncomment the line bellow to understand what is going on
	//awsGlacier.Config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries | aws.LogDebugWithSigning)

	response, err := awsGlacier.UploadArchive(&awsArchive)
	if err != nil {
		return "", "", fmt.Errorf("error sending archive to aws glacier. details: %s", err)
	}

	return *response.ArchiveId, *response.Location, nil
}

func sendBigArchive(archive *os.File, archiveSize int64, awsAccountID, awsRegion, awsVaultName string) (archiveID, location string, err error) {
	awsSession, err := session.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("error creating aws session. details: %s", err)
	}

	awsGlacier := glacier.New(awsSession, &aws.Config{
		Region: aws.String(awsRegion),
	})

	// Uncomment the line bellow to understand what is going on
	//awsGlacier.Config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries | aws.LogDebugWithSigning)

	awsInitiate := glacier.InitiateMultipartUploadInput{
		AccountId:          aws.String(awsAccountID),
		ArchiveDescription: aws.String(fmt.Sprintf("backup file from %s", time.Now().Format(time.RFC3339))),
		PartSize:           aws.String(strconv.FormatInt(partSize, 10)),
		VaultName:          aws.String(awsVaultName),
	}

	awsInitiateResponse, err := awsGlacier.InitiateMultipartUpload(&awsInitiate)
	if err != nil {
		return "", "", fmt.Errorf("error initializing multipart upload. details: %s", err)
	}

	var offset int64
	var part = make([]byte, partSize)

	for offset = 0; offset < archiveSize; offset += partSize {
		n, err := archive.Read(part)
		if err != nil {
			return "", "", fmt.Errorf("error reading an archive part (%d). details: %s", offset, err)
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
			return "", "", fmt.Errorf("error sending an archive part (%d). details: %s", offset, err)
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
		return "", "", fmt.Errorf("error completing multipart upload. details: %s", err)
	}

	return *awsCompleteResponse.ArchiveId, *awsCompleteResponse.Location, nil
}
