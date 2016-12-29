package cloud_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/glacier"
	"github.com/kr/pretty"
	"github.com/rafaeljusto/toglacier/internal/cloud"
)

func TestNewAWSCloud(t *testing.T) {
	scenarios := []struct {
		description   string
		accountID     string
		vaultName     string
		debug         bool
		env           map[string]string
		expected      *cloud.AWSCloud
		expectedEnv   map[string]string
		expectedError error
	}{
		{
			description: "it should decrypt secrets and build a AWS cloud instance correctly",
			accountID:   "encrypted:gqUkdJOn1fPhRtDRMwoUfQACs7Ugh3E=",
			vaultName:   "vault",
			debug:       true,
			env: map[string]string{
				"AWS_ACCESS_KEY_ID":     "encrypted:70kFcm/ppZ4tHIspcH3ucbgzSsKQ",
				"AWS_SECRET_ACCESS_KEY": "encrypted:tddsKQECwhIVLDAhXsgaT97NsPlN4Q==",
			},
			expected: &cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
			},
			expectedEnv: map[string]string{
				"AWS_ACCESS_KEY_ID":     "keyid",
				"AWS_SECRET_ACCESS_KEY": "secret",
			},
		},
		{
			description: "it should detect an error while decrypting an invalid AWS access key ID",
			accountID:   "encrypted:gqUkdJOn1fPhRtDRMwoUfQACs7Ugh3E=",
			vaultName:   "vault",
			debug:       true,
			env: map[string]string{
				"AWS_ACCESS_KEY_ID":     "encrypted:xxxxxxxxxxxxxxxxxxxxxxxxx",
				"AWS_SECRET_ACCESS_KEY": "encrypted:tddsKQECwhIVLDAhXsgaT97NsPlN4Q==",
			},
			expectedEnv: map[string]string{
				"AWS_ACCESS_KEY_ID":     "encrypted:xxxxxxxxxxxxxxxxxxxxxxxxx",
				"AWS_SECRET_ACCESS_KEY": "encrypted:tddsKQECwhIVLDAhXsgaT97NsPlN4Q==",
			},
			expectedError: fmt.Errorf("error decrypting aws access key id. details: %s", base64.CorruptInputError(24)),
		},
		{
			description: "it should detect an error while decrypting an invalid AWS secret access key",
			accountID:   "encrypted:gqUkdJOn1fPhRtDRMwoUfQACs7Ugh3E=",
			vaultName:   "vault",
			debug:       true,
			env: map[string]string{
				"AWS_ACCESS_KEY_ID":     "encrypted:70kFcm/ppZ4tHIspcH3ucbgzSsKQ",
				"AWS_SECRET_ACCESS_KEY": "encrypted:xxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			expectedEnv: map[string]string{
				"AWS_ACCESS_KEY_ID":     "keyid",
				"AWS_SECRET_ACCESS_KEY": "encrypted:xxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			expectedError: fmt.Errorf("error decrypting aws secret access key. details: %s", base64.CorruptInputError(24)),
		},
		{
			description: "it should detect an error while decrypting an invalid AWS account ID",
			accountID:   "encrypted:xxxxxxxxxxxxxxxxxxxxxxxxx=",
			vaultName:   "vault",
			debug:       true,
			env: map[string]string{
				"AWS_ACCESS_KEY_ID":     "encrypted:70kFcm/ppZ4tHIspcH3ucbgzSsKQ",
				"AWS_SECRET_ACCESS_KEY": "encrypted:tddsKQECwhIVLDAhXsgaT97NsPlN4Q==",
			},
			expectedEnv: map[string]string{
				"AWS_ACCESS_KEY_ID":     "keyid",
				"AWS_SECRET_ACCESS_KEY": "secret",
			},
			expectedError: fmt.Errorf("error decrypting aws account id. details: %s", base64.CorruptInputError(25)),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			os.Clearenv()
			for key, value := range scenario.env {
				os.Setenv(key, value)
			}

			awsCloud, err := cloud.NewAWSCloud(scenario.accountID, scenario.vaultName, scenario.debug)

			// we are not interested on testing low level structures from AWS library
			// or clock controlling layer
			if scenario.expected != nil {
				scenario.expected.Glacier = awsCloud.Glacier
				scenario.expected.Clock = awsCloud.Clock
			}

			if !reflect.DeepEqual(scenario.expected, awsCloud) {
				t.Errorf("backups don't match.\n%s", pretty.Diff(scenario.expected, awsCloud))
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

	scenarios := []struct {
		description          string
		filename             string
		multipartUploadLimit int64
		partSize             int64
		awsCloud             cloud.AWSCloud
		expected             cloud.Backup
		expectedError        error
	}{
		{
			description:          "it should detect when the file doesn't exist",
			filename:             "toglacier-idontexist.tmp",
			multipartUploadLimit: 102400,
			partSize:             4096,
			expectedError: fmt.Errorf("error opening archive. details: %s", &os.PathError{
				Op:   "open",
				Path: "toglacier-idontexist.tmp",
				Err:  errors.New("no such file or directory"),
			}),
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
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New("error sending archive to aws glacier. details: connection error"),
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
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New("error comparing checksums"),
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
			partSize:             100,
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						return &glacier.UploadMultipartPartOutput{}, nil
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
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New("error initializing multipart upload. details: aws is out"),
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
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func() func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						var i int
						return func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
							i++
							if i >= 5 {
								return nil, errors.New("part rejected")
							} else {
								return &glacier.UploadMultipartPartOutput{}, nil
							}
						}
					}(),
				},
				Clock: fakeClock{
					mockNow: func() time.Time {
						return time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC)
					},
				},
			},
			expectedError: errors.New("error sending an archive part (400). details: part rejected"),
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
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						return &glacier.UploadMultipartPartOutput{}, nil
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
			expectedError: errors.New("error completing multipart upload. details: backup too big"),
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
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
					mockInitiateMultipartUpload: func(*glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
						return &glacier.InitiateMultipartUploadOutput{
							UploadId: aws.String("UPLOAD123"),
						}, nil
					},
					mockUploadMultipartPart: func(*glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
						return &glacier.UploadMultipartPartOutput{}, nil
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
			expectedError: errors.New("error comparing checksums"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			cloud.MultipartUploadLimit(scenario.multipartUploadLimit)
			cloud.PartSize(scenario.partSize)

			backup, err := scenario.awsCloud.Send(scenario.filename)
			if !reflect.DeepEqual(scenario.expected, backup) {
				t.Errorf("backups don't match.\n%s", pretty.Diff(scenario.expected, backup))
			}
			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestAWSCloud_List(t *testing.T) {
	defer cloud.WaitJobTime(time.Minute)
	cloud.WaitJobTime(100 * time.Millisecond)

	scenarios := []struct {
		description   string
		awsCloud      cloud.AWSCloud
		expected      []cloud.Backup
		expectedError error
	}{
		{
			description: "it should list all backups correctly",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
							ArchiveList   cloud.AWSIventoryArchiveList
						}{
							ArchiveList: cloud.AWSIventoryArchiveList{
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
				},
				{
					ID:        "AWSID123",
					CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
					Checksum:  "a75e723eaf6da1db780e0a9b6a2046eba1a6bc20e8e69ffcb7c633e5e51f2502",
					VaultName: "vault",
				},
			},
		},
		{
			description: "it should detect an error while initiating the job",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
					mockInitiateJob: func(*glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
						return nil, errors.New("a crazy error")
					},
				},
			},
			expectedError: errors.New("error initiating the job. details: a crazy error"),
		},
		{
			description: "it should detect when there's an error listing the existing jobs",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New("error retrieving the job from aws. details: another crazy error"),
		},
		{
			description: "it should detect when the job failed",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New("error retrieving the job from aws. details: something went wrong"),
		},
		{
			description: "it should detect when the job was not found",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New("job not found in aws"),
		},
		{
			description: "it should continue checking jobs until it completes",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
							ArchiveList   cloud.AWSIventoryArchiveList
						}{
							ArchiveList: cloud.AWSIventoryArchiveList{
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
				},
			},
		},
		{
			description: "it should detect an error while retrieving the job data",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New("error retrieving the job information. details: job corrupted"),
		},
		{
			description: "it should detect an error while decoding the job data",
			awsCloud: cloud.AWSCloud{
				AccountID: "account",
				VaultName: "vault",
				Glacier: glacierAPIMock{
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
			expectedError: errors.New(`error decoding the iventory. details: invalid character '{' looking for beginning of object key string`),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			backups, err := scenario.awsCloud.List()
			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", pretty.Diff(scenario.expected, backups))
			}
			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

type glacierAPIMock struct {
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

func (g glacierAPIMock) AbortMultipartUploadRequest(a *glacier.AbortMultipartUploadInput) (*request.Request, *glacier.AbortMultipartUploadOutput) {
	return g.mockAbortMultipartUploadRequest(a)
}

func (g glacierAPIMock) AbortMultipartUpload(a *glacier.AbortMultipartUploadInput) (*glacier.AbortMultipartUploadOutput, error) {
	return g.mockAbortMultipartUpload(a)
}

func (g glacierAPIMock) AbortVaultLockRequest(a *glacier.AbortVaultLockInput) (*request.Request, *glacier.AbortVaultLockOutput) {
	return g.mockAbortVaultLockRequest(a)
}

func (g glacierAPIMock) AbortVaultLock(a *glacier.AbortVaultLockInput) (*glacier.AbortVaultLockOutput, error) {
	return g.mockAbortVaultLock(a)
}

func (g glacierAPIMock) AddTagsToVaultRequest(a *glacier.AddTagsToVaultInput) (*request.Request, *glacier.AddTagsToVaultOutput) {
	return g.mockAddTagsToVaultRequest(a)
}

func (g glacierAPIMock) AddTagsToVault(a *glacier.AddTagsToVaultInput) (*glacier.AddTagsToVaultOutput, error) {
	return g.mockAddTagsToVault(a)
}

func (g glacierAPIMock) CompleteMultipartUploadRequest(c *glacier.CompleteMultipartUploadInput) (*request.Request, *glacier.ArchiveCreationOutput) {
	return g.mockCompleteMultipartUploadRequest(c)
}

func (g glacierAPIMock) CompleteMultipartUpload(c *glacier.CompleteMultipartUploadInput) (*glacier.ArchiveCreationOutput, error) {
	return g.mockCompleteMultipartUpload(c)
}

func (g glacierAPIMock) CompleteVaultLockRequest(c *glacier.CompleteVaultLockInput) (*request.Request, *glacier.CompleteVaultLockOutput) {
	return g.mockCompleteVaultLockRequest(c)
}

func (g glacierAPIMock) CompleteVaultLock(c *glacier.CompleteVaultLockInput) (*glacier.CompleteVaultLockOutput, error) {
	return g.mockCompleteVaultLock(c)
}

func (g glacierAPIMock) CreateVaultRequest(c *glacier.CreateVaultInput) (*request.Request, *glacier.CreateVaultOutput) {
	return g.mockCreateVaultRequest(c)
}

func (g glacierAPIMock) CreateVault(c *glacier.CreateVaultInput) (*glacier.CreateVaultOutput, error) {
	return g.mockCreateVault(c)
}

func (g glacierAPIMock) DeleteArchiveRequest(d *glacier.DeleteArchiveInput) (*request.Request, *glacier.DeleteArchiveOutput) {
	return g.mockDeleteArchiveRequest(d)
}

func (g glacierAPIMock) DeleteArchive(d *glacier.DeleteArchiveInput) (*glacier.DeleteArchiveOutput, error) {
	return g.mockDeleteArchive(d)
}

func (g glacierAPIMock) DeleteVaultRequest(d *glacier.DeleteVaultInput) (*request.Request, *glacier.DeleteVaultOutput) {
	return g.mockDeleteVaultRequest(d)
}

func (g glacierAPIMock) DeleteVault(d *glacier.DeleteVaultInput) (*glacier.DeleteVaultOutput, error) {
	return g.mockDeleteVault(d)
}

func (g glacierAPIMock) DeleteVaultAccessPolicyRequest(d *glacier.DeleteVaultAccessPolicyInput) (*request.Request, *glacier.DeleteVaultAccessPolicyOutput) {
	return g.mockDeleteVaultAccessPolicyRequest(d)
}

func (g glacierAPIMock) DeleteVaultAccessPolicy(d *glacier.DeleteVaultAccessPolicyInput) (*glacier.DeleteVaultAccessPolicyOutput, error) {
	return g.mockDeleteVaultAccessPolicy(d)
}

func (g glacierAPIMock) DeleteVaultNotificationsRequest(d *glacier.DeleteVaultNotificationsInput) (*request.Request, *glacier.DeleteVaultNotificationsOutput) {
	return g.mockDeleteVaultNotificationsRequest(d)
}

func (g glacierAPIMock) DeleteVaultNotifications(d *glacier.DeleteVaultNotificationsInput) (*glacier.DeleteVaultNotificationsOutput, error) {
	return g.mockDeleteVaultNotifications(d)
}

func (g glacierAPIMock) DescribeJobRequest(d *glacier.DescribeJobInput) (*request.Request, *glacier.JobDescription) {
	return g.mockDescribeJobRequest(d)
}

func (g glacierAPIMock) DescribeJob(d *glacier.DescribeJobInput) (*glacier.JobDescription, error) {
	return g.mockDescribeJob(d)
}

func (g glacierAPIMock) DescribeVaultRequest(d *glacier.DescribeVaultInput) (*request.Request, *glacier.DescribeVaultOutput) {
	return g.mockDescribeVaultRequest(d)
}

func (g glacierAPIMock) DescribeVault(d *glacier.DescribeVaultInput) (*glacier.DescribeVaultOutput, error) {
	return g.mockDescribeVault(d)
}

func (g glacierAPIMock) GetDataRetrievalPolicyRequest(d *glacier.GetDataRetrievalPolicyInput) (*request.Request, *glacier.GetDataRetrievalPolicyOutput) {
	return g.mockGetDataRetrievalPolicyRequest(d)
}

func (g glacierAPIMock) GetDataRetrievalPolicy(d *glacier.GetDataRetrievalPolicyInput) (*glacier.GetDataRetrievalPolicyOutput, error) {
	return g.mockGetDataRetrievalPolicy(d)
}

func (g glacierAPIMock) GetJobOutputRequest(d *glacier.GetJobOutputInput) (*request.Request, *glacier.GetJobOutputOutput) {
	return g.mockGetJobOutputRequest(d)
}

func (g glacierAPIMock) GetJobOutput(d *glacier.GetJobOutputInput) (*glacier.GetJobOutputOutput, error) {
	return g.mockGetJobOutput(d)
}

func (g glacierAPIMock) GetVaultAccessPolicyRequest(d *glacier.GetVaultAccessPolicyInput) (*request.Request, *glacier.GetVaultAccessPolicyOutput) {
	return g.mockGetVaultAccessPolicyRequest(d)
}

func (g glacierAPIMock) GetVaultAccessPolicy(d *glacier.GetVaultAccessPolicyInput) (*glacier.GetVaultAccessPolicyOutput, error) {
	return g.mockGetVaultAccessPolicy(d)
}

func (g glacierAPIMock) GetVaultLockRequest(d *glacier.GetVaultLockInput) (*request.Request, *glacier.GetVaultLockOutput) {
	return g.mockGetVaultLockRequest(d)
}

func (g glacierAPIMock) GetVaultLock(d *glacier.GetVaultLockInput) (*glacier.GetVaultLockOutput, error) {
	return g.mockGetVaultLock(d)
}

func (g glacierAPIMock) GetVaultNotificationsRequest(d *glacier.GetVaultNotificationsInput) (*request.Request, *glacier.GetVaultNotificationsOutput) {
	return g.mockGetVaultNotificationsRequest(d)
}

func (g glacierAPIMock) GetVaultNotifications(d *glacier.GetVaultNotificationsInput) (*glacier.GetVaultNotificationsOutput, error) {
	return g.mockGetVaultNotifications(d)
}

func (g glacierAPIMock) InitiateJobRequest(i *glacier.InitiateJobInput) (*request.Request, *glacier.InitiateJobOutput) {
	return g.mockInitiateJobRequest(i)
}

func (g glacierAPIMock) InitiateJob(i *glacier.InitiateJobInput) (*glacier.InitiateJobOutput, error) {
	return g.mockInitiateJob(i)
}

func (g glacierAPIMock) InitiateMultipartUploadRequest(i *glacier.InitiateMultipartUploadInput) (*request.Request, *glacier.InitiateMultipartUploadOutput) {
	return g.mockInitiateMultipartUploadRequest(i)
}

func (g glacierAPIMock) InitiateMultipartUpload(i *glacier.InitiateMultipartUploadInput) (*glacier.InitiateMultipartUploadOutput, error) {
	return g.mockInitiateMultipartUpload(i)
}

func (g glacierAPIMock) InitiateVaultLockRequest(i *glacier.InitiateVaultLockInput) (*request.Request, *glacier.InitiateVaultLockOutput) {
	return g.mockInitiateVaultLockRequest(i)
}

func (g glacierAPIMock) InitiateVaultLock(i *glacier.InitiateVaultLockInput) (*glacier.InitiateVaultLockOutput, error) {
	return g.mockInitiateVaultLock(i)
}

func (g glacierAPIMock) ListJobsRequest(l *glacier.ListJobsInput) (*request.Request, *glacier.ListJobsOutput) {
	return g.mockListJobsRequest(l)
}

func (g glacierAPIMock) ListJobs(l *glacier.ListJobsInput) (*glacier.ListJobsOutput, error) {
	return g.mockListJobs(l)
}

func (g glacierAPIMock) ListJobsPages(l *glacier.ListJobsInput, f func(*glacier.ListJobsOutput, bool) bool) error {
	return g.mockListJobsPages(l, f)
}

func (g glacierAPIMock) ListMultipartUploadsRequest(l *glacier.ListMultipartUploadsInput) (*request.Request, *glacier.ListMultipartUploadsOutput) {
	return g.mockListMultipartUploadsRequest(l)
}

func (g glacierAPIMock) ListMultipartUploads(l *glacier.ListMultipartUploadsInput) (*glacier.ListMultipartUploadsOutput, error) {
	return g.mockListMultipartUploads(l)
}

func (g glacierAPIMock) ListMultipartUploadsPages(l *glacier.ListMultipartUploadsInput, f func(*glacier.ListMultipartUploadsOutput, bool) bool) error {
	return g.mockListMultipartUploadsPages(l, f)
}

func (g glacierAPIMock) ListPartsRequest(l *glacier.ListPartsInput) (*request.Request, *glacier.ListPartsOutput) {
	return g.mockListPartsRequest(l)
}

func (g glacierAPIMock) ListParts(l *glacier.ListPartsInput) (*glacier.ListPartsOutput, error) {
	return g.mockListParts(l)
}

func (g glacierAPIMock) ListPartsPages(l *glacier.ListPartsInput, f func(*glacier.ListPartsOutput, bool) bool) error {
	return g.mockListPartsPages(l, f)
}

func (g glacierAPIMock) ListTagsForVaultRequest(l *glacier.ListTagsForVaultInput) (*request.Request, *glacier.ListTagsForVaultOutput) {
	return g.mockListTagsForVaultRequest(l)
}

func (g glacierAPIMock) ListTagsForVault(l *glacier.ListTagsForVaultInput) (*glacier.ListTagsForVaultOutput, error) {
	return g.mockListTagsForVault(l)
}

func (g glacierAPIMock) ListVaultsRequest(l *glacier.ListVaultsInput) (*request.Request, *glacier.ListVaultsOutput) {
	return g.mockListVaultsRequest(l)
}

func (g glacierAPIMock) ListVaults(l *glacier.ListVaultsInput) (*glacier.ListVaultsOutput, error) {
	return g.mockListVaults(l)
}

func (g glacierAPIMock) ListVaultsPages(l *glacier.ListVaultsInput, f func(*glacier.ListVaultsOutput, bool) bool) error {
	return g.mockListVaultsPages(l, f)
}

func (g glacierAPIMock) RemoveTagsFromVaultRequest(r *glacier.RemoveTagsFromVaultInput) (*request.Request, *glacier.RemoveTagsFromVaultOutput) {
	return g.mockRemoveTagsFromVaultRequest(r)
}

func (g glacierAPIMock) RemoveTagsFromVault(r *glacier.RemoveTagsFromVaultInput) (*glacier.RemoveTagsFromVaultOutput, error) {
	return g.mockRemoveTagsFromVault(r)
}

func (g glacierAPIMock) SetDataRetrievalPolicyRequest(s *glacier.SetDataRetrievalPolicyInput) (*request.Request, *glacier.SetDataRetrievalPolicyOutput) {
	return g.mockSetDataRetrievalPolicyRequest(s)
}

func (g glacierAPIMock) SetDataRetrievalPolicy(s *glacier.SetDataRetrievalPolicyInput) (*glacier.SetDataRetrievalPolicyOutput, error) {
	return g.mockSetDataRetrievalPolicy(s)
}

func (g glacierAPIMock) SetVaultAccessPolicyRequest(s *glacier.SetVaultAccessPolicyInput) (*request.Request, *glacier.SetVaultAccessPolicyOutput) {
	return g.mockSetVaultAccessPolicyRequest(s)
}

func (g glacierAPIMock) SetVaultAccessPolicy(s *glacier.SetVaultAccessPolicyInput) (*glacier.SetVaultAccessPolicyOutput, error) {
	return g.mockSetVaultAccessPolicy(s)
}

func (g glacierAPIMock) SetVaultNotificationsRequest(s *glacier.SetVaultNotificationsInput) (*request.Request, *glacier.SetVaultNotificationsOutput) {
	return g.mockSetVaultNotificationsRequest(s)
}

func (g glacierAPIMock) SetVaultNotifications(s *glacier.SetVaultNotificationsInput) (*glacier.SetVaultNotificationsOutput, error) {
	return g.mockSetVaultNotifications(s)
}

func (g glacierAPIMock) UploadArchiveRequest(u *glacier.UploadArchiveInput) (*request.Request, *glacier.ArchiveCreationOutput) {
	return g.mockUploadArchiveRequest(u)
}

func (g glacierAPIMock) UploadArchive(u *glacier.UploadArchiveInput) (*glacier.ArchiveCreationOutput, error) {
	return g.mockUploadArchive(u)
}

func (g glacierAPIMock) UploadMultipartPartRequest(u *glacier.UploadMultipartPartInput) (*request.Request, *glacier.UploadMultipartPartOutput) {
	return g.mockUploadMultipartPartRequest(u)
}

func (g glacierAPIMock) UploadMultipartPart(u *glacier.UploadMultipartPartInput) (*glacier.UploadMultipartPartOutput, error) {
	return g.mockUploadMultipartPart(u)
}

func (g glacierAPIMock) WaitUntilVaultExists(d *glacier.DescribeVaultInput) error {
	return g.mockWaitUntilVaultExists(d)
}

func (g glacierAPIMock) WaitUntilVaultNotExists(d *glacier.DescribeVaultInput) error {
	return g.mockWaitUntilVaultNotExists(d)
}

type fakeClock struct {
	mockNow func() time.Time
}

func (f fakeClock) Now() time.Time {
	return f.mockNow()
}
