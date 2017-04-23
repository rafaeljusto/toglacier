package cloud_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aryann/difflib"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/davecgh/go-spew/spew"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/log"
)

func TestNewAWSCloud(t *testing.T) {
	scenarios := []struct {
		description   string
		logger        log.Logger
		config        *config.Config
		debug         bool
		expected      *cloud.AWSCloud
		expectedEnv   map[string]string
		expectedError error
	}{
		{
			description: "it should build a AWS cloud instance correctly",
			config: func() *config.Config {
				c := new(config.Config)
				c.AWS.AccountID.Value = "account"
				c.AWS.AccessKeyID.Value = "keyid"
				c.AWS.SecretAccessKey.Value = "secret"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "vault"
				return c
			}(),
			debug: true,
			expected: &cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
			},
			expectedEnv: map[string]string{
				"AWS_ACCESS_KEY_ID":     "keyid",
				"AWS_SECRET_ACCESS_KEY": "secret",
				"AWS_REGION":            "us-east-1",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			os.Clearenv()

			awsCloud, err := cloud.NewAWSCloud(scenario.logger, scenario.config, scenario.debug)

			// we are not interested on testing low level structures from AWS library
			// or clock controlling layer
			if scenario.expected != nil {
				scenario.expected.Glacier = awsCloud.Glacier
				scenario.expected.Clock = awsCloud.Clock
			}

			if !reflect.DeepEqual(scenario.expected, awsCloud) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, awsCloud))
			}
			for key, value := range scenario.expectedEnv {
				if env := os.Getenv(key); env != value {
					t.Errorf("environment variable “%s” doesn't match. expected “%s” and got “%s”", key, value, env)
				}
			}
			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAWSCloud_Send(t *testing.T) {
	defer cloud.MultipartUploadLimit(102400)
	defer cloud.PartSize(4096)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	scenarios := []struct {
		description          string
		filename             string
		multipartUploadLimit int64
		partSize             int64
		awsCloud             cloud.AWSCloud
		randomSource         io.Reader
		goFunc               func()
		expected             cloud.Backup
		expectedError        error
	}{
		{
			description:          "it should detect when the file doesn't exist",
			filename:             "toglacier-idontexist.tmp",
			multipartUploadLimit: 102400,
			partSize:             4096,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeOpeningArchive,
				Err: &os.PathError{
					Op:   "open",
					Path: "toglacier-idontexist.tmp",
					Err:  errors.New("no such file or directory"),
				},
			},
		},
		{
			description: "it should send a small backup correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			multipartUploadLimit: 102400,
			partSize:             4096,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockUploadArchive: func(*glacier.UploadArchiveInput) (*glacier.ArchiveCreationOutput, error) {
						return &glacier.ArchiveCreationOutput{
							ArchiveId: aws.String("AWSID123"),
							Checksum:  aws.String("cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705"),
							Location:  aws.String("/archive/AWSID123"),
						}, nil
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expected: cloud.Backup{
				ID:        "AWSID123",
				CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
				Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
				VaultName: "vault",
				Size:      41,
			},
		},
		{
			description: "it should detect an error while sending a small backup",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			multipartUploadLimit: 102400,
			partSize:             4096,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockUploadArchive: func(*glacier.UploadArchiveInput) (*glacier.ArchiveCreationOutput, error) {
						return nil, errors.New("connection error")
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeSendingArchive,
				Err:  errors.New("connection error"),
			},
		},
		{
			description: "it should detect when the hash is different after sending a small backup",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			multipartUploadLimit: 102400,
			partSize:             4096,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockUploadArchive: func(*glacier.UploadArchiveInput) (*glacier.ArchiveCreationOutput, error) {
						return &glacier.ArchiveCreationOutput{
							ArchiveId: aws.String("AWSID123"),
							Checksum:  aws.String("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
							Location:  aws.String("/archive/AWSID123"),
						}, nil
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeComparingChecksums,
			},
		},
		{
			description: "it should send a big backup correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             1048576,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateMultipartUpload: func(i *glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						partSize, err := strconv.ParseInt(*i.PartSize, 10, 64)
						if err != nil {
							return nil, err
						}

						// Part size must be a power of two and be between 1048576 and
						// 4294967296 bytes
						if partSize < 1048576 || partSize > 4294967296 || partSize&(partSize-1) != 0 {
							return nil, errors.New("Invalid part size")
						}

						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						hash := glacier.ComputeHashes(u.Body)
						return &glacier.UploadMultipartPartOutput{
							Checksum: aws.String(hex.EncodeToString(hash.TreeHash)),
						}, nil
					},
					mockCompleteMultipartUpload: func(*glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error) {
						return &glacier.ArchiveCreationOutput{
							ArchiveId: aws.String("AWSID123"),
							Checksum:  aws.String("a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242"),
							Location:  aws.String("/archive/AWSID123"),
						}, nil
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expected: cloud.Backup{
				ID:        "AWSID123",
				CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
				Checksum:  "a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242",
				VaultName: "vault",
				Size:      42000,
			},
		},
		{
			description: "it should detect an error initiating a big backup upload",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             100,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return nil, errors.New("aws is out")
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeInitMultipart,
				Err:  errors.New("aws is out"),
			},
		},
		{
			description: "it should detect an error while sending big backup part",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             100,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockAbortMultipartUpload: func(*glacier.AbortMultipartUploadInput) (*glacier.AbortMultipartUploadOutput, error) {
						return nil, nil
					},
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func() func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						var i int
						return func(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
							i++
							if i >= 5 {
								return nil, errors.New("part rejected")
							}

							hash := glacier.ComputeHashes(u.Body)
							return &glacier.UploadMultipartPartOutput{
								Checksum: aws.String(hex.EncodeToString(hash.TreeHash)),
							}, nil
						}
					}(),
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expectedError: &cloud.MultipartError{
				Offset: 400,
				Size:   42000,
				Code:   cloud.MultipartErrorCodeSendingArchive,
				Err:    errors.New("part rejected"),
			},
		},
		{
			description: "it should detect when backup part checksum don't match",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             100,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockAbortMultipartUpload: func(*glacier.AbortMultipartUploadInput) (*glacier.AbortMultipartUploadOutput, error) {
						return nil, nil
					},
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						return &glacier.UploadMultipartPartOutput{
							Checksum: aws.String("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
						}, nil
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expectedError: &cloud.MultipartError{
				Offset: 0,
				Size:   42000,
				Code:   cloud.MultipartErrorCodeComparingChecksums,
			},
		},
		{
			description: "it should detect an error while completing a big backup upload",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             100,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockAbortMultipartUpload: func(*glacier.AbortMultipartUploadInput) (*glacier.AbortMultipartUploadOutput, error) {
						return nil, nil
					},
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						hash := glacier.ComputeHashes(u.Body)
						return &glacier.UploadMultipartPartOutput{
							Checksum: aws.String(hex.EncodeToString(hash.TreeHash)),
						}, nil
					},
					mockCompleteMultipartUpload: func(*glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error) {
						return nil, errors.New("backup too big")
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "UPLOAD123",
				Code: cloud.ErrorCodeCompleteMultipart,
				Err:  errors.New("backup too big"),
			},
		},
		{
			description: "it should detect when a big backup checksum don't match",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             100,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockDeleteArchive: func(d *glacier.DeleteArchiveInput) (*glacier.DeleteArchiveOutput, error) {
						if *d.ArchiveId != "AWSID123" {
							return nil, fmt.Errorf("unexpected id %s", *d.ArchiveId)
						}

						return &glacier.DeleteArchiveOutput{}, nil
					},
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						hash := glacier.ComputeHashes(u.Body)
						return &glacier.UploadMultipartPartOutput{
							Checksum: aws.String(hex.EncodeToString(hash.TreeHash)),
						}, nil
					},
					mockCompleteMultipartUpload: func(*glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error) {
						return &glacier.ArchiveCreationOutput{
							ArchiveId: aws.String("AWSID123"),
							Checksum:  aws.String("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
							Location:  aws.String("/archive/AWSID123"),
						}, nil
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expected: cloud.Backup{
				ID:        "AWSID123",
				CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
				Checksum:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				VaultName: "vault",
			},
			expectedError: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeComparingChecksums,
			},
		},
		{
			description: "it should detect when a big backup checksum don't match and fail to remove it",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             100,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockDeleteArchive: func(*glacier.DeleteArchiveInput) (*glacier.DeleteArchiveOutput, error) {
						return nil, errors.New("connection error")
					},
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						hash := glacier.ComputeHashes(u.Body)
						return &glacier.UploadMultipartPartOutput{
							Checksum: aws.String(hex.EncodeToString(hash.TreeHash)),
						}, nil
					},
					mockCompleteMultipartUpload: func(*glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error) {
						return &glacier.ArchiveCreationOutput{
							ArchiveId: aws.String("AWSID123"),
							Checksum:  aws.String("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
							Location:  aws.String("/archive/AWSID123"),
						}, nil
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expected: cloud.Backup{
				ID:        "AWSID123",
				CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
				Checksum:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				VaultName: "vault",
			},
			expectedError: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeComparingChecksums,
				Err: &cloud.Error{
					ID:   "AWSID123",
					Code: cloud.ErrorCodeRemovingArchive,
					Err:  errors.New("connection error"),
				},
			},
		},
		{
			description: "it should detect when a big backup is cancelled",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString(strings.Repeat("Important information for the test backup\n", 1000))
				return f.Name()
			}(),
			multipartUploadLimit: 1024,
			partSize:             1048576,
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateMultipartUpload: func(i *glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						partSize, err := strconv.ParseInt(*i.PartSize, 10, 64)
						if err != nil {
							return nil, err
						}

						// Part size must be a power of two and be between 1048576 and
						// 4294967296 bytes
						if partSize < 1048576 || partSize > 4294967296 || partSize&(partSize-1) != 0 {
							return nil, errors.New("Invalid part size")
						}

						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						// sleep for a small amount of time to allow the task to be
						// cancelled
						time.Sleep(200 * time.Millisecond)

						hash := glacier.ComputeHashes(u.Body)
						return &glacier.UploadMultipartPartOutput{
							Checksum: aws.String(hex.EncodeToString(hash.TreeHash)),
						}, nil
					},
					mockCompleteMultipartUpload: func(*glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error) {
						return &glacier.ArchiveCreationOutput{
							ArchiveId: aws.String("AWSID123"),
							Checksum:  aws.String("a6d392677577af12fb1f4ceb510940374c3378455a1485b0226a35ef5ad65242"),
							Location:  aws.String("/archive/AWSID123"),
						}, nil
					},
					mockAbortMultipartUpload: func(*glacier.AbortMultipartUploadInput) (*glacier.AbortMultipartUploadOutput, error) {
						return nil, nil
					},
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			goFunc: func() {
				// wait for the send task to start
				time.Sleep(100 * time.Millisecond)
				cancel()
			},
			expectedError: &cloud.MultipartError{
				Offset: 0,
				Size:   42000,
				Code:   cloud.MultipartErrorCodeCancelled,
				Err:    context.Canceled,
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			cloud.MultipartUploadLimit(scenario.multipartUploadLimit)
			cloud.PartSize(scenario.partSize)

			if scenario.goFunc != nil {
				go scenario.goFunc()
			}

			backup, err := scenario.awsCloud.Send(ctx, scenario.filename)
			if !reflect.DeepEqual(scenario.expected, backup) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backup))
			}
			if !cloud.ErrorEqual(scenario.expectedError, err) && !cloud.MultipartErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAWSCloud_List(t *testing.T) {
	defer cloud.WaitJobTime(time.Minute)
	cloud.WaitJobTime(100 * time.Millisecond)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	scenarios := []struct {
		description   string
		awsCloud      cloud.AWSCloud
		goFunc        func()
		expected      []cloud.Backup
		expectedError error
	}{
		{
			description: "it should list all backups correctly",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:      aws.String("JOBID123"),
									Completed:  aws.Bool(true),
									StatusCode: aws.String("Succeeded"),
								},
							},
						}, nil
					},
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						iventory := struct {
							VaultARN      string `json:"VaultARN"`
							InventoryDate string `json:"InventoryDate"`
							ArchiveList   cloud.AWSInventoryArchiveList
						}{
							ArchiveList: cloud.AWSInventoryArchiveList{
								{
									ArchiveID:          "AWSID123",
									ArchiveDescription: "another test backup",
									CreationDate:       time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
									Size:               4000,
									SHA256TreeHash:     "a75e723eaf6da1db780e0a9b6a2046eba1a6bc20e8e69ffcb7c633e5e51f2502",
								},
								{
									ArchiveID:          "AWSID122",
									ArchiveDescription: "great test",
									CreationDate:       time.Date(2016, 11, 7, 12, 0, 0, 0, time.UTC),
									Size:               2456,
									SHA256TreeHash:     "223072246f6eedbf1271bd1576f01b4b67c8e1cb1142599d5ef615673f513a5f",
								},
							},
						}

						body, err := json.Marshal(iventory)
						if err != nil {
							t.Fatalf("error build job output response. details: %s", err)
						}

						return &glacier.GetJobOutputOutput{
							Body: ioutil.NopCloser(bytes.NewBuffer(body)),
						}, nil
					},
				},
			},
			expected: []cloud.Backup{
				{
					ID:        "AWSID122",
					CreatedAt: time.Date(2016, 11, 7, 12, 0, 0, 0, time.UTC),
					Checksum:  "223072246f6eedbf1271bd1576f01b4b67c8e1cb1142599d5ef615673f513a5f",
					VaultName: "vault",
					Size:      2456,
				},
				{
					ID:        "AWSID123",
					CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
					Checksum:  "a75e723eaf6da1db780e0a9b6a2046eba1a6bc20e8e69ffcb7c633e5e51f2502",
					VaultName: "vault",
					Size:      4000,
				},
			},
		},
		{
			description: "it should detect an error while initiating the job",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return nil, errors.New("a crazy error")
					},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeInitJob,
				Err:  errors.New("a crazy error"),
			},
		},
		{
			description: "it should detect when there's an error listing the existing jobs",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return nil, errors.New("another crazy error")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeRetrievingJob,
				Err:  errors.New("another crazy error"),
			},
		},
		{
			description: "it should detect when the job failed",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:         aws.String("JOBID123"),
									Completed:     aws.Bool(true),
									StatusCode:    aws.String("Failed"),
									StatusMessage: aws.String("something went wrong"),
								},
							},
						}, nil
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeJobFailed,
				Err:  errors.New("something went wrong"),
			},
		},
		{
			description: "it should detect when the job was not found",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:      aws.String("JOBID321"),
									Completed:  aws.Bool(true),
									StatusCode: aws.String("Succeeded"),
								},
							},
						}, nil
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeJobNotFound,
			},
		},
		{
			description: "it should continue checking jobs until it completes",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func() func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						var i int
						return func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
							i++
							return &glacier.ListJobsOutput{
								JobList: []*glacier.JobDescription{
									{
										JobId:      aws.String("JOBID123"),
										Completed:  aws.Bool(i == 2),
										StatusCode: aws.String("Succeeded"),
									},
								},
							}, nil
						}
					}(),
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						iventory := struct {
							VaultARN      string `json:"VaultARN"`
							InventoryDate string `json:"InventoryDate"`
							ArchiveList   cloud.AWSInventoryArchiveList
						}{
							ArchiveList: cloud.AWSInventoryArchiveList{
								{
									ArchiveID:          "AWSID123",
									ArchiveDescription: "another test backup",
									CreationDate:       time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
									Size:               4000,
									SHA256TreeHash:     "a75e723eaf6da1db780e0a9b6a2046eba1a6bc20e8e69ffcb7c633e5e51f2502",
								},
							},
						}

						body, err := json.Marshal(iventory)
						if err != nil {
							t.Fatalf("error build job output response. details: %s", err)
						}

						return &glacier.GetJobOutputOutput{
							Body: ioutil.NopCloser(bytes.NewBuffer(body)),
						}, nil
					},
				},
			},
			expected: []cloud.Backup{
				{
					ID:        "AWSID123",
					CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
					Checksum:  "a75e723eaf6da1db780e0a9b6a2046eba1a6bc20e8e69ffcb7c633e5e51f2502",
					VaultName: "vault",
					Size:      4000,
				},
			},
		},
		{
			description: "it should detect an error while retrieving the job data",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:      aws.String("JOBID123"),
									Completed:  aws.Bool(true),
									StatusCode: aws.String("Succeeded"),
								},
							},
						}, nil
					},
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						return nil, errors.New("job corrupted")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeJobComplete,
				Err:  errors.New("job corrupted"),
			},
		},
		{
			description: "it should detect an error while decoding the job data",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:      aws.String("JOBID123"),
									Completed:  aws.Bool(true),
									StatusCode: aws.String("Succeeded"),
								},
							},
						}, nil
					},
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						return &glacier.GetJobOutputOutput{
							Body: ioutil.NopCloser(bytes.NewBufferString(`{{{invalid json`)),
						}, nil
					},
				},
			},
			// *json.SyntaxError doesn't export the msg attribute, so we need to
			// hard-coded the error message here
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeDecodingData,
				Err:  errors.New("invalid character '{' looking for beginning of object key string"),
			},
		},
		{
			description: "it should detect when the action is cancelled by the user",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func() func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						var i int
						return func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
							// sleep for a small amount of time to allow the task to be
							// cancelled
							time.Sleep(200 * time.Millisecond)

							i++
							return &glacier.ListJobsOutput{
								JobList: []*glacier.JobDescription{
									{
										JobId:      aws.String("JOBID123"),
										Completed:  aws.Bool(i == 2),
										StatusCode: aws.String("Succeeded"),
									},
								},
							}, nil
						}
					}(),
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						iventory := struct {
							VaultARN      string `json:"VaultARN"`
							InventoryDate string `json:"InventoryDate"`
							ArchiveList   cloud.AWSInventoryArchiveList
						}{
							ArchiveList: cloud.AWSInventoryArchiveList{
								{
									ArchiveID:          "AWSID123",
									ArchiveDescription: "another test backup",
									CreationDate:       time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
									Size:               4000,
									SHA256TreeHash:     "a75e723eaf6da1db780e0a9b6a2046eba1a6bc20e8e69ffcb7c633e5e51f2502",
								},
								{
									ArchiveID:          "AWSID122",
									ArchiveDescription: "great test",
									CreationDate:       time.Date(2016, 11, 7, 12, 0, 0, 0, time.UTC),
									Size:               2456,
									SHA256TreeHash:     "223072246f6eedbf1271bd1576f01b4b67c8e1cb1142599d5ef615673f513a5f",
								},
							},
						}

						body, err := json.Marshal(iventory)
						if err != nil {
							t.Fatalf("error build job output response. details: %s", err)
						}

						return &glacier.GetJobOutputOutput{
							Body: ioutil.NopCloser(bytes.NewBuffer(body)),
						}, nil
					},
				},
			},
			goFunc: func() {
				// wait for the send task to start
				time.Sleep(100 * time.Millisecond)
				cancel()
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeCancelled,
				Err:  context.Canceled,
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if scenario.goFunc != nil {
				go scenario.goFunc()
			}

			backups, err := scenario.awsCloud.List(ctx)
			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backups))
			}
			if !cloud.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAWSCloud_Get(t *testing.T) {
	defer cloud.WaitJobTime(time.Minute)
	cloud.WaitJobTime(100 * time.Millisecond)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	scenarios := []struct {
		description   string
		id            string
		awsCloud      cloud.AWSCloud
		goFunc        func()
		expected      string
		expectedError error
	}{
		{
			description: "it should retrieve a backup correctly",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:      aws.String("JOBID123"),
									Completed:  aws.Bool(true),
									StatusCode: aws.String("Succeeded"),
								},
							},
						}, nil
					},
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						return &glacier.GetJobOutputOutput{
							Body: ioutil.NopCloser(bytes.NewBufferString("Important information for the test backup")),
						}, nil
					},
				},
			},
			expected: path.Join(os.TempDir(), "backup-AWSID123.tar"),
		},
		{
			description: "it should detect an error while initiating the job",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return nil, errors.New("a crazy error")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeInitJob,
				Err:  errors.New("a crazy error"),
			},
		},
		{
			description: "it should detect when there's an error listing the existing jobs",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return nil, errors.New("another crazy error")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeRetrievingJob,
				Err:  errors.New("another crazy error"),
			},
		},
		{
			description: "it should detect when the job failed",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:         aws.String("JOBID123"),
									Completed:     aws.Bool(true),
									StatusCode:    aws.String("Failed"),
									StatusMessage: aws.String("something went wrong"),
								},
							},
						}, nil
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeJobFailed,
				Err:  errors.New("something went wrong"),
			},
		},
		{
			description: "it should detect when the job was not found",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:      aws.String("JOBID321"),
									Completed:  aws.Bool(true),
									StatusCode: aws.String("Succeeded"),
								},
							},
						}, nil
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeJobNotFound,
			},
		},
		{
			description: "it should continue checking jobs until it completes",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func() func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						var i int
						return func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
							i++
							return &glacier.ListJobsOutput{
								JobList: []*glacier.JobDescription{
									{
										JobId:      aws.String("JOBID123"),
										Completed:  aws.Bool(i == 2),
										StatusCode: aws.String("Succeeded"),
									},
								},
							}, nil
						}
					}(),
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						return &glacier.GetJobOutputOutput{
							Body: ioutil.NopCloser(bytes.NewBufferString("Important information for the test backup")),
						}, nil
					},
				},
			},
			expected: path.Join(os.TempDir(), "backup-AWSID123.tar"),
		},
		{
			description: "it should detect an error while retrieving the job data",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						return &glacier.ListJobsOutput{
							JobList: []*glacier.JobDescription{
								{
									JobId:      aws.String("JOBID123"),
									Completed:  aws.Bool(true),
									StatusCode: aws.String("Succeeded"),
								},
							},
						}, nil
					},
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						return nil, errors.New("job corrupted")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeJobComplete,
				Err:  errors.New("job corrupted"),
			},
		},
		{
			description: "it should detect when the task was cancelled by the user",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return &glacier.InitiateJobOutput{
							JobId: aws.String("JOBID123"),
						}, nil
					},
					mockListJobs: func() func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
						var i int
						return func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
							// sleep for a small amount of time to allow the task to be
							// cancelled
							time.Sleep(200 * time.Millisecond)

							i++
							return &glacier.ListJobsOutput{
								JobList: []*glacier.JobDescription{
									{
										JobId:      aws.String("JOBID123"),
										Completed:  aws.Bool(i == 2),
										StatusCode: aws.String("Succeeded"),
									},
								},
							}, nil
						}
					}(),
					mockGetJobOutput: func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
						return &glacier.GetJobOutputOutput{
							Body: ioutil.NopCloser(bytes.NewBufferString("Important information for the test backup")),
						}, nil
					},
				},
			},
			goFunc: func() {
				// wait for the send task to start
				time.Sleep(100 * time.Millisecond)
				cancel()
			},
			expectedError: &cloud.Error{
				ID:   "JOBID123",
				Code: cloud.ErrorCodeCancelled,
				Err:  context.Canceled,
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			if scenario.goFunc != nil {
				go scenario.goFunc()
			}

			filename, err := scenario.awsCloud.Get(ctx, scenario.id)
			if !reflect.DeepEqual(scenario.expected, filename) {
				t.Errorf("filenames don't match.\n%s", Diff(scenario.expected, filename))
			}
			if !cloud.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAWSCloud_Remove(t *testing.T) {
	scenarios := []struct {
		description   string
		id            string
		awsCloud      cloud.AWSCloud
		expectedError error
	}{
		{
			description: "it should remove a backup correctly",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockDeleteArchive: func(*glacier.DeleteArchiveInput) (*glacier.DeleteArchiveOutput, error) {
						return &glacier.DeleteArchiveOutput{}, nil
					},
				},
			},
		},
		{
			description: "it should detect an error while removing a backup",
			id:          "AWSID123",
			awsCloud: cloud.AWSCloud{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				AccountID: "account",
				VaultName: "vault",
				Glacier: mockGlacierAPI{
					mockDeleteArchive: func(*glacier.DeleteArchiveInput) (*glacier.DeleteArchiveOutput, error) {
						return nil, errors.New("no backup here")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "AWSID123",
				Code: cloud.ErrorCodeRemovingArchive,
				Err:  errors.New("no backup here"),
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			err := scenario.awsCloud.Remove(context.Background(), scenario.id)
			if !cloud.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

type mockGlacierAPI struct {
	mockAbortMultipartUploadRequest     func(*glacier.AbortMultipartUploadInput) (*request.Request, *glacier.AbortMultipartUploadOutput)
	mockAbortMultipartUpload            func(*glacier.AbortMultipartUploadInput) (*glacier.AbortMultipartUploadOutput, error)
	mockAbortVaultLockRequest           func(*glacier.AbortVaultLockInput) (*request.Request, *glacier.AbortVaultLockOutput)
	mockAbortVaultLock                  func(*glacier.AbortVaultLockInput) (*glacier.AbortVaultLockOutput, error)
	mockAddTagsToVaultRequest           func(*glacier.AddTagsToVaultInput) (*request.Request, *glacier.AddTagsToVaultOutput)
	mockAddTagsToVault                  func(*glacier.AddTagsToVaultInput) (*glacier.AddTagsToVaultOutput, error)
	mockCompleteMultipartUploadRequest  func(*glacier.CompleteMultipartUploadInput) (*request.Request, *glacier.ArchiveCreationOutput)
	mockCompleteMultipartUpload         func(*glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error)
	mockCompleteVaultLockRequest        func(*glacier.CompleteVaultLockInput) (*request.Request, *glacier.CompleteVaultLockOutput)
	mockCompleteVaultLock               func(*glacier.CompleteVaultLockInput) (*glacier.CompleteVaultLockOutput, error)
	mockCreateVaultRequest              func(*glacier.CreateVaultInput) (*request.Request, *glacier.CreateVaultOutput)
	mockCreateVault                     func(*glacier.CreateVaultInput) (*glacier.CreateVaultOutput, error)
	mockDeleteArchiveRequest            func(*glacier.DeleteArchiveInput) (*request.Request, *glacier.DeleteArchiveOutput)
	mockDeleteArchive                   func(*glacier.DeleteArchiveInput) (*glacier.DeleteArchiveOutput, error)
	mockDeleteVaultRequest              func(*glacier.DeleteVaultInput) (*request.Request, *glacier.DeleteVaultOutput)
	mockDeleteVault                     func(*glacier.DeleteVaultInput) (*glacier.DeleteVaultOutput, error)
	mockDeleteVaultAccessPolicyRequest  func(*glacier.DeleteVaultAccessPolicyInput) (*request.Request, *glacier.DeleteVaultAccessPolicyOutput)
	mockDeleteVaultAccessPolicy         func(*glacier.DeleteVaultAccessPolicyInput) (*glacier.DeleteVaultAccessPolicyOutput, error)
	mockDeleteVaultNotificationsRequest func(*glacier.DeleteVaultNotificationsInput) (*request.Request, *glacier.DeleteVaultNotificationsOutput)
	mockDeleteVaultNotifications        func(*glacier.DeleteVaultNotificationsInput) (*glacier.DeleteVaultNotificationsOutput, error)
	mockDescribeJobRequest              func(*glacier.DescribeJobInput) (*request.Request, *glacier.JobDescription)
	mockDescribeJob                     func(*glacier.DescribeJobInput) (*glacier.JobDescription, error)
	mockDescribeVaultRequest            func(*glacier.DescribeVaultInput) (*request.Request, *glacier.DescribeVaultOutput)
	mockDescribeVault                   func(*glacier.DescribeVaultInput) (*glacier.DescribeVaultOutput, error)
	mockGetDataRetrievalPolicyRequest   func(*glacier.GetDataRetrievalPolicyInput) (*request.Request, *glacier.GetDataRetrievalPolicyOutput)
	mockGetDataRetrievalPolicy          func(*glacier.GetDataRetrievalPolicyInput) (*glacier.GetDataRetrievalPolicyOutput, error)
	mockGetJobOutputRequest             func(*glacier.GetJobOutputInput) (*request.Request, *glacier.GetJobOutputOutput)
	mockGetJobOutput                    func(*glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error)
	mockGetVaultAccessPolicyRequest     func(*glacier.GetVaultAccessPolicyInput) (*request.Request, *glacier.GetVaultAccessPolicyOutput)
	mockGetVaultAccessPolicy            func(*glacier.GetVaultAccessPolicyInput) (*glacier.GetVaultAccessPolicyOutput, error)
	mockGetVaultLockRequest             func(*glacier.GetVaultLockInput) (*request.Request, *glacier.GetVaultLockOutput)
	mockGetVaultLock                    func(*glacier.GetVaultLockInput) (*glacier.GetVaultLockOutput, error)
	mockGetVaultNotificationsRequest    func(*glacier.GetVaultNotificationsInput) (*request.Request, *glacier.GetVaultNotificationsOutput)
	mockGetVaultNotifications           func(*glacier.GetVaultNotificationsInput) (*glacier.GetVaultNotificationsOutput, error)
	mockInitiateJobRequest              func(*glacier.InitiateJobInput) (*request.Request, *glacier.InitiateJobOutput)
	mockInitiateJob                     func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error)
	mockInitiateMultipartUploadRequest  func(*glacier.InitiateMultipartUploadInput) (*request.Request, *glacier.InitiateMultipartUploadOutput)
	mockInitiateMultipartUpload         func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error)
	mockInitiateVaultLockRequest        func(*glacier.InitiateVaultLockInput) (*request.Request, *glacier.InitiateVaultLockOutput)
	mockInitiateVaultLock               func(*glacier.InitiateVaultLockInput) (*glacier.InitiateVaultLockOutput, error)
	mockListJobsRequest                 func(*glacier.ListJobsInput) (*request.Request, *glacier.ListJobsOutput)
	mockListJobs                        func(*glacier.ListJobsInput) (*glacier.ListJobsOutput, error)
	mockListJobsPages                   func(*glacier.ListJobsInput, func(*glacier.ListJobsOutput, bool) bool) error
	mockListMultipartUploadsRequest     func(*glacier.ListMultipartUploadsInput) (*request.Request, *glacier.ListMultipartUploadsOutput)
	mockListMultipartUploads            func(*glacier.ListMultipartUploadsInput) (*glacier.ListMultipartUploadsOutput, error)
	mockListMultipartUploadsPages       func(*glacier.ListMultipartUploadsInput, func(*glacier.ListMultipartUploadsOutput, bool) bool) error
	mockListPartsRequest                func(*glacier.ListPartsInput) (*request.Request, *glacier.ListPartsOutput)
	mockListParts                       func(*glacier.ListPartsInput) (*glacier.ListPartsOutput, error)
	mockListPartsPages                  func(*glacier.ListPartsInput, func(*glacier.ListPartsOutput, bool) bool) error
	mockListTagsForVaultRequest         func(*glacier.ListTagsForVaultInput) (*request.Request, *glacier.ListTagsForVaultOutput)
	mockListTagsForVault                func(*glacier.ListTagsForVaultInput) (*glacier.ListTagsForVaultOutput, error)
	mockListVaultsRequest               func(*glacier.ListVaultsInput) (*request.Request, *glacier.ListVaultsOutput)
	mockListVaults                      func(*glacier.ListVaultsInput) (*glacier.ListVaultsOutput, error)
	mockListVaultsPages                 func(*glacier.ListVaultsInput, func(*glacier.ListVaultsOutput, bool) bool) error
	mockRemoveTagsFromVaultRequest      func(*glacier.RemoveTagsFromVaultInput) (*request.Request, *glacier.RemoveTagsFromVaultOutput)
	mockRemoveTagsFromVault             func(*glacier.RemoveTagsFromVaultInput) (*glacier.RemoveTagsFromVaultOutput, error)
	mockSetDataRetrievalPolicyRequest   func(*glacier.SetDataRetrievalPolicyInput) (*request.Request, *glacier.SetDataRetrievalPolicyOutput)
	mockSetDataRetrievalPolicy          func(*glacier.SetDataRetrievalPolicyInput) (*glacier.SetDataRetrievalPolicyOutput, error)
	mockSetVaultAccessPolicyRequest     func(*glacier.SetVaultAccessPolicyInput) (*request.Request, *glacier.SetVaultAccessPolicyOutput)
	mockSetVaultAccessPolicy            func(*glacier.SetVaultAccessPolicyInput) (*glacier.SetVaultAccessPolicyOutput, error)
	mockSetVaultNotificationsRequest    func(*glacier.SetVaultNotificationsInput) (*request.Request, *glacier.SetVaultNotificationsOutput)
	mockSetVaultNotifications           func(*glacier.SetVaultNotificationsInput) (*glacier.SetVaultNotificationsOutput, error)
	mockUploadArchiveRequest            func(*glacier.UploadArchiveInput) (*request.Request, *glacier.ArchiveCreationOutput)
	mockUploadArchive                   func(*glacier.UploadArchiveInput) (*glacier.ArchiveCreationOutput, error)
	mockUploadMultipartPartRequest      func(*glacier.UploadMultipartPartInput) (*request.Request, *glacier.UploadMultipartPartOutput)
	mockUploadMultipartPart             func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error)
	mockWaitUntilVaultExists            func(*glacier.DescribeVaultInput) error
	mockWaitUntilVaultNotExists         func(*glacier.DescribeVaultInput) error
}

func (g mockGlacierAPI) AbortMultipartUploadRequest(a *glacier.AbortMultipartUploadInput) (*request.Request, *glacier.AbortMultipartUploadOutput) {
	return g.mockAbortMultipartUploadRequest(a)
}

func (g mockGlacierAPI) AbortMultipartUpload(a *glacier.AbortMultipartUploadInput) (*glacier.AbortMultipartUploadOutput, error) {
	return g.mockAbortMultipartUpload(a)
}

func (g mockGlacierAPI) AbortVaultLockRequest(a *glacier.AbortVaultLockInput) (*request.Request, *glacier.AbortVaultLockOutput) {
	return g.mockAbortVaultLockRequest(a)
}

func (g mockGlacierAPI) AbortVaultLock(a *glacier.AbortVaultLockInput) (*glacier.AbortVaultLockOutput, error) {
	return g.mockAbortVaultLock(a)
}

func (g mockGlacierAPI) AddTagsToVaultRequest(a *glacier.AddTagsToVaultInput) (*request.Request, *glacier.AddTagsToVaultOutput) {
	return g.mockAddTagsToVaultRequest(a)
}

func (g mockGlacierAPI) AddTagsToVault(a *glacier.AddTagsToVaultInput) (*glacier.AddTagsToVaultOutput, error) {
	return g.mockAddTagsToVault(a)
}

func (g mockGlacierAPI) CompleteMultipartUploadRequest(c *glacier.CompleteMultipartUploadInput) (*request.Request, *glacier.ArchiveCreationOutput) {
	return g.mockCompleteMultipartUploadRequest(c)
}

func (g mockGlacierAPI) CompleteMultipartUpload(c *glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error) {
	return g.mockCompleteMultipartUpload(c)
}

func (g mockGlacierAPI) CompleteVaultLockRequest(c *glacier.CompleteVaultLockInput) (*request.Request, *glacier.CompleteVaultLockOutput) {
	return g.mockCompleteVaultLockRequest(c)
}

func (g mockGlacierAPI) CompleteVaultLock(c *glacier.CompleteVaultLockInput) (*glacier.CompleteVaultLockOutput, error) {
	return g.mockCompleteVaultLock(c)
}

func (g mockGlacierAPI) CreateVaultRequest(c *glacier.CreateVaultInput) (*request.Request, *glacier.CreateVaultOutput) {
	return g.mockCreateVaultRequest(c)
}

func (g mockGlacierAPI) CreateVault(c *glacier.CreateVaultInput) (*glacier.CreateVaultOutput, error) {
	return g.mockCreateVault(c)
}

func (g mockGlacierAPI) DeleteArchiveRequest(d *glacier.DeleteArchiveInput) (*request.Request, *glacier.DeleteArchiveOutput) {
	return g.mockDeleteArchiveRequest(d)
}

func (g mockGlacierAPI) DeleteArchive(d *glacier.DeleteArchiveInput) (*glacier.DeleteArchiveOutput, error) {
	return g.mockDeleteArchive(d)
}

func (g mockGlacierAPI) DeleteVaultRequest(d *glacier.DeleteVaultInput) (*request.Request, *glacier.DeleteVaultOutput) {
	return g.mockDeleteVaultRequest(d)
}

func (g mockGlacierAPI) DeleteVault(d *glacier.DeleteVaultInput) (*glacier.DeleteVaultOutput, error) {
	return g.mockDeleteVault(d)
}

func (g mockGlacierAPI) DeleteVaultAccessPolicyRequest(d *glacier.DeleteVaultAccessPolicyInput) (*request.Request, *glacier.DeleteVaultAccessPolicyOutput) {
	return g.mockDeleteVaultAccessPolicyRequest(d)
}

func (g mockGlacierAPI) DeleteVaultAccessPolicy(d *glacier.DeleteVaultAccessPolicyInput) (*glacier.DeleteVaultAccessPolicyOutput, error) {
	return g.mockDeleteVaultAccessPolicy(d)
}

func (g mockGlacierAPI) DeleteVaultNotificationsRequest(d *glacier.DeleteVaultNotificationsInput) (*request.Request, *glacier.DeleteVaultNotificationsOutput) {
	return g.mockDeleteVaultNotificationsRequest(d)
}

func (g mockGlacierAPI) DeleteVaultNotifications(d *glacier.DeleteVaultNotificationsInput) (*glacier.DeleteVaultNotificationsOutput, error) {
	return g.mockDeleteVaultNotifications(d)
}

func (g mockGlacierAPI) DescribeJobRequest(d *glacier.DescribeJobInput) (*request.Request, *glacier.JobDescription) {
	return g.mockDescribeJobRequest(d)
}

func (g mockGlacierAPI) DescribeJob(d *glacier.DescribeJobInput) (*glacier.JobDescription, error) {
	return g.mockDescribeJob(d)
}

func (g mockGlacierAPI) DescribeVaultRequest(d *glacier.DescribeVaultInput) (*request.Request, *glacier.DescribeVaultOutput) {
	return g.mockDescribeVaultRequest(d)
}

func (g mockGlacierAPI) DescribeVault(d *glacier.DescribeVaultInput) (*glacier.DescribeVaultOutput, error) {
	return g.mockDescribeVault(d)
}

func (g mockGlacierAPI) GetDataRetrievalPolicyRequest(d *glacier.GetDataRetrievalPolicyInput) (*request.Request, *glacier.GetDataRetrievalPolicyOutput) {
	return g.mockGetDataRetrievalPolicyRequest(d)
}

func (g mockGlacierAPI) GetDataRetrievalPolicy(d *glacier.GetDataRetrievalPolicyInput) (*glacier.GetDataRetrievalPolicyOutput, error) {
	return g.mockGetDataRetrievalPolicy(d)
}

func (g mockGlacierAPI) GetJobOutputRequest(d *glacier.GetJobOutputInput) (*request.Request, *glacier.GetJobOutputOutput) {
	return g.mockGetJobOutputRequest(d)
}

func (g mockGlacierAPI) GetJobOutput(d *glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
	return g.mockGetJobOutput(d)
}

func (g mockGlacierAPI) GetVaultAccessPolicyRequest(d *glacier.GetVaultAccessPolicyInput) (*request.Request, *glacier.GetVaultAccessPolicyOutput) {
	return g.mockGetVaultAccessPolicyRequest(d)
}

func (g mockGlacierAPI) GetVaultAccessPolicy(d *glacier.GetVaultAccessPolicyInput) (*glacier.GetVaultAccessPolicyOutput, error) {
	return g.mockGetVaultAccessPolicy(d)
}

func (g mockGlacierAPI) GetVaultLockRequest(d *glacier.GetVaultLockInput) (*request.Request, *glacier.GetVaultLockOutput) {
	return g.mockGetVaultLockRequest(d)
}

func (g mockGlacierAPI) GetVaultLock(d *glacier.GetVaultLockInput) (*glacier.GetVaultLockOutput, error) {
	return g.mockGetVaultLock(d)
}

func (g mockGlacierAPI) GetVaultNotificationsRequest(d *glacier.GetVaultNotificationsInput) (*request.Request, *glacier.GetVaultNotificationsOutput) {
	return g.mockGetVaultNotificationsRequest(d)
}

func (g mockGlacierAPI) GetVaultNotifications(d *glacier.GetVaultNotificationsInput) (*glacier.GetVaultNotificationsOutput, error) {
	return g.mockGetVaultNotifications(d)
}

func (g mockGlacierAPI) InitiateJobRequest(i *glacier.InitiateJobInput) (*request.Request, *glacier.InitiateJobOutput) {
	return g.mockInitiateJobRequest(i)
}

func (g mockGlacierAPI) InitiateJob(i *glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
	return g.mockInitiateJob(i)
}

func (g mockGlacierAPI) InitiateMultipartUploadRequest(i *glacier.InitiateMultipartUploadInput) (*request.Request, *glacier.InitiateMultipartUploadOutput) {
	return g.mockInitiateMultipartUploadRequest(i)
}

func (g mockGlacierAPI) InitiateMultipartUpload(i *glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
	return g.mockInitiateMultipartUpload(i)
}

func (g mockGlacierAPI) InitiateVaultLockRequest(i *glacier.InitiateVaultLockInput) (*request.Request, *glacier.InitiateVaultLockOutput) {
	return g.mockInitiateVaultLockRequest(i)
}

func (g mockGlacierAPI) InitiateVaultLock(i *glacier.InitiateVaultLockInput) (*glacier.InitiateVaultLockOutput, error) {
	return g.mockInitiateVaultLock(i)
}

func (g mockGlacierAPI) ListJobsRequest(l *glacier.ListJobsInput) (*request.Request, *glacier.ListJobsOutput) {
	return g.mockListJobsRequest(l)
}

func (g mockGlacierAPI) ListJobs(l *glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
	return g.mockListJobs(l)
}

func (g mockGlacierAPI) ListJobsPages(l *glacier.ListJobsInput, f func(*glacier.ListJobsOutput, bool) bool) error {
	return g.mockListJobsPages(l, f)
}

func (g mockGlacierAPI) ListMultipartUploadsRequest(l *glacier.ListMultipartUploadsInput) (*request.Request, *glacier.ListMultipartUploadsOutput) {
	return g.mockListMultipartUploadsRequest(l)
}

func (g mockGlacierAPI) ListMultipartUploads(l *glacier.ListMultipartUploadsInput) (*glacier.ListMultipartUploadsOutput, error) {
	return g.mockListMultipartUploads(l)
}

func (g mockGlacierAPI) ListMultipartUploadsPages(l *glacier.ListMultipartUploadsInput, f func(*glacier.ListMultipartUploadsOutput, bool) bool) error {
	return g.mockListMultipartUploadsPages(l, f)
}

func (g mockGlacierAPI) ListPartsRequest(l *glacier.ListPartsInput) (*request.Request, *glacier.ListPartsOutput) {
	return g.mockListPartsRequest(l)
}

func (g mockGlacierAPI) ListParts(l *glacier.ListPartsInput) (*glacier.ListPartsOutput, error) {
	return g.mockListParts(l)
}

func (g mockGlacierAPI) ListPartsPages(l *glacier.ListPartsInput, f func(*glacier.ListPartsOutput, bool) bool) error {
	return g.mockListPartsPages(l, f)
}

func (g mockGlacierAPI) ListTagsForVaultRequest(l *glacier.ListTagsForVaultInput) (*request.Request, *glacier.ListTagsForVaultOutput) {
	return g.mockListTagsForVaultRequest(l)
}

func (g mockGlacierAPI) ListTagsForVault(l *glacier.ListTagsForVaultInput) (*glacier.ListTagsForVaultOutput, error) {
	return g.mockListTagsForVault(l)
}

func (g mockGlacierAPI) ListVaultsRequest(l *glacier.ListVaultsInput) (*request.Request, *glacier.ListVaultsOutput) {
	return g.mockListVaultsRequest(l)
}

func (g mockGlacierAPI) ListVaults(l *glacier.ListVaultsInput) (*glacier.ListVaultsOutput, error) {
	return g.mockListVaults(l)
}

func (g mockGlacierAPI) ListVaultsPages(l *glacier.ListVaultsInput, f func(*glacier.ListVaultsOutput, bool) bool) error {
	return g.mockListVaultsPages(l, f)
}

func (g mockGlacierAPI) RemoveTagsFromVaultRequest(r *glacier.RemoveTagsFromVaultInput) (*request.Request, *glacier.RemoveTagsFromVaultOutput) {
	return g.mockRemoveTagsFromVaultRequest(r)
}

func (g mockGlacierAPI) RemoveTagsFromVault(r *glacier.RemoveTagsFromVaultInput) (*glacier.RemoveTagsFromVaultOutput, error) {
	return g.mockRemoveTagsFromVault(r)
}

func (g mockGlacierAPI) SetDataRetrievalPolicyRequest(s *glacier.SetDataRetrievalPolicyInput) (*request.Request, *glacier.SetDataRetrievalPolicyOutput) {
	return g.mockSetDataRetrievalPolicyRequest(s)
}

func (g mockGlacierAPI) SetDataRetrievalPolicy(s *glacier.SetDataRetrievalPolicyInput) (*glacier.SetDataRetrievalPolicyOutput, error) {
	return g.mockSetDataRetrievalPolicy(s)
}

func (g mockGlacierAPI) SetVaultAccessPolicyRequest(s *glacier.SetVaultAccessPolicyInput) (*request.Request, *glacier.SetVaultAccessPolicyOutput) {
	return g.mockSetVaultAccessPolicyRequest(s)
}

func (g mockGlacierAPI) SetVaultAccessPolicy(s *glacier.SetVaultAccessPolicyInput) (*glacier.SetVaultAccessPolicyOutput, error) {
	return g.mockSetVaultAccessPolicy(s)
}

func (g mockGlacierAPI) SetVaultNotificationsRequest(s *glacier.SetVaultNotificationsInput) (*request.Request, *glacier.SetVaultNotificationsOutput) {
	return g.mockSetVaultNotificationsRequest(s)
}

func (g mockGlacierAPI) SetVaultNotifications(s *glacier.SetVaultNotificationsInput) (*glacier.SetVaultNotificationsOutput, error) {
	return g.mockSetVaultNotifications(s)
}

func (g mockGlacierAPI) UploadArchiveRequest(u *glacier.UploadArchiveInput) (*request.Request, *glacier.ArchiveCreationOutput) {
	return g.mockUploadArchiveRequest(u)
}

func (g mockGlacierAPI) UploadArchive(u *glacier.UploadArchiveInput) (*glacier.ArchiveCreationOutput, error) {
	return g.mockUploadArchive(u)
}

func (g mockGlacierAPI) UploadMultipartPartRequest(u *glacier.UploadMultipartPartInput) (*request.Request, *glacier.UploadMultipartPartOutput) {
	return g.mockUploadMultipartPartRequest(u)
}

func (g mockGlacierAPI) UploadMultipartPart(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
	return g.mockUploadMultipartPart(u)
}

func (g mockGlacierAPI) WaitUntilVaultExists(d *glacier.DescribeVaultInput) error {
	return g.mockWaitUntilVaultExists(d)
}

func (g mockGlacierAPI) WaitUntilVaultNotExists(d *glacier.DescribeVaultInput) error {
	return g.mockWaitUntilVaultNotExists(d)
}

type fakeClock struct {
	mockNow func() time.Time
}

func (f fakeClock) Now() time.Time {
	return f.mockNow()
}

type mockReader struct {
	mockRead func(p []byte) (n int, err error)
}

func (m mockReader) Read(p []byte) (n int, err error) {
	return m.mockRead(p)
}

type mockLogger struct {
	mockDebug  func(args ...interface{})
	mockDebugf func(format string, args ...interface{})
	mockInfo   func(args ...interface{})
	mockInfof  func(format string, args ...interface{})
}

func (m mockLogger) Debug(args ...interface{}) {
	m.mockDebug(args...)
}
func (m mockLogger) Debugf(format string, args ...interface{}) {
	m.mockDebugf(format, args...)
}
func (m mockLogger) Info(args ...interface{}) {
	m.mockInfo(args...)
}
func (m mockLogger) Infof(format string, args ...interface{}) {
	m.mockInfof(format, args...)
}

// Diff is useful to see the difference when comparing two complex types.
func Diff(a, b interface{}) []difflib.DiffRecord {
	return difflib.Diff(strings.SplitAfter(spew.Sdump(a), "\n"), strings.SplitAfter(spew.Sdump(b), "\n"))
}
