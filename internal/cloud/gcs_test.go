package cloud_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/log"
	gcscontext "golang.org/x/net/context"
	"google.golang.org/api/iterator"
)

func TestNewGCS(t *testing.T) {
	ctx := context.Background()

	scenarios := []struct {
		description   string
		logger        log.Logger
		config        cloud.GCSConfig
		expected      *cloud.GCS
		expectedError error
	}{
		{
			description: "it should build a GCS instance correctly",
			config: cloud.GCSConfig{
				Project: "toglacier",
				Bucket:  "backup",
				AccountFile: func() string {
					f, err := ioutil.TempFile("", "toglacier-test-")
					if err != nil {
						t.Fatalf("error creating file. details: %s", err)
					}
					defer f.Close()

					encoder := json.NewEncoder(f)
					err = encoder.Encode(struct {
						Type                    string `json:"type"`
						ProjectID               string `json:"project_id"`
						PrivateKeyID            string `json:"private_key_id"`
						PrivateKey              string `json:"private_key"`
						ClientEmail             string `json:"client_email"`
						ClientID                string `json:"client_id"`
						AuthURI                 string `json:"auth_uri"`
						TokenURI                string `json:"token_uri"`
						AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
						ClientX509CertURL       string `json:"client_x509_cert_url"`
					}{
						Type:         "service_account",
						ProjectID:    "toglacier",
						PrivateKeyID: "key-id",
						PrivateKey: `-----BEGIN PRIVATE KEY-----
-----END PRIVATE KEY-----
`,
						ClientEmail:             "toglacier@toglacier.com",
						ClientID:                "1234",
						AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
						TokenURI:                "https://accounts.google.com/o/oauth2/token",
						AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
						ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/toglacier%40toglacier.com",
					})

					if err != nil {
						t.Fatalf("error encoding account file. details: %s", err)
					}

					return f.Name()
				}(),
			},
			expected: &cloud.GCS{
				BucketName: "backup",
			},
		},
		{
			description: "it should detect when the account file does not exist",
			config: cloud.GCSConfig{
				Project:     "toglacier",
				Bucket:      "backup",
				AccountFile: "i-dont-exist.json",
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeInitializingSession,
				// there's no type for the following error inside Google library
				Err: fmt.Errorf("dialing: cannot read credentials file: %v", &os.PathError{
					Op:   "open",
					Path: "i-dont-exist.json",
					Err:  errors.New("no such file or directory"),
				}),
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			gcs, err := cloud.NewGCS(ctx, scenario.logger, scenario.config)

			// we are not interested on testing low level structures from GCS library
			if scenario.expected != nil && gcs != nil {
				scenario.expected.Client = gcs.Client
				scenario.expected.Bucket = gcs.Bucket
				scenario.expected.ObjectHandler = gcs.ObjectHandler
			}

			if !reflect.DeepEqual(scenario.expected, gcs) {
				t.Errorf("cloud instances don't match.\n%s", Diff(scenario.expected, gcs))
			}
			if !cloud.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestGCS_Send(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	scenarios := []struct {
		description   string
		filename      string
		gcs           cloud.GCS
		goFunc        func()
		expected      cloud.Backup
		expectedError error
	}{
		{
			description: "it should detect when the file doesn't exist",
			filename:    "toglacier-idontexist.tmp",
			gcs: cloud.GCS{
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
			description: "it should send a backup correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockWrite: func(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error {
						return nil
					},
					mockAttrs: func(ctx gcscontext.Context, obj *storage.ObjectHandle) (*storage.ObjectAttrs, error) {
						return &storage.ObjectAttrs{
							Name: "GCSID123",
							Size: 41,
							MD5: func() []byte {
								hash, err := base64.StdEncoding.DecodeString("cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705")
								if err != nil {
									t.Fatalf("error decoding hash string. details: %s", err)
								}
								return hash
							}(),
							Created: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
						}, nil
					},
				},
			},
			expected: cloud.Backup{
				ID:        "GCSID123",
				CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
				Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
				VaultName: "backup",
				Size:      41,
			},
		},
		{
			description: "it should detect an error uploading the data to the cloud",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockWrite: func(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error {
						return errors.New("error uploading data")
					},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeSendingArchive,
				Err:  errors.New("error uploading data"),
			},
		},
		{
			description: "it should detect an error reading the object attributes",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockWrite: func(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error {
						return nil
					},
					mockAttrs: func(ctx gcscontext.Context, obj *storage.ObjectHandle) (*storage.ObjectAttrs, error) {
						return nil, errors.New("fail to read attrs")
					},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeArchiveInfo,
				Err:  errors.New("fail to read attrs"),
			},
		},
		{
			description: "it should detect when the user cancelled the action while the backup was being sent to the cloud",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-test-")
				if err != nil {
					t.Fatalf("error creating file. details: %s", err)
				}
				defer f.Close()

				f.WriteString("Important information for the test backup")
				return f.Name()
			}(),
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockWrite: func(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error {
						// sleep for a small amount of time to allow the task to be
						// cancelled
						select {
						case <-time.After(200 * time.Millisecond):
						// do nothing
						case <-ctx.Done():
							return ctx.Err()
						}

						return nil
					},
					mockAttrs: func(ctx gcscontext.Context, obj *storage.ObjectHandle) (*storage.ObjectAttrs, error) {
						return &storage.ObjectAttrs{
							Name: "GCSID123",
							Size: 41,
							MD5: func() []byte {
								hash, err := base64.StdEncoding.DecodeString("cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705")
								if err != nil {
									t.Fatalf("error decoding hash string. details: %s", err)
								}
								return hash
							}(),
							Created: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
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

			backup, err := scenario.gcs.Send(ctx, scenario.filename)
			if !reflect.DeepEqual(scenario.expected, backup) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backup))
			}
			if !cloud.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestGCS_List(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	scenarios := []struct {
		description   string
		gcs           cloud.GCS
		goFunc        func()
		expected      []cloud.Backup
		expectedError error
	}{
		{
			description: "it should list all backups correctly",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObjects: func(ctx gcscontext.Context, q *storage.Query) *storage.ObjectIterator {
						return &storage.ObjectIterator{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockIterate: func() func(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
						var i = 0
						return func(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
							i++
							switch i {
							case 1:
								return &storage.ObjectAttrs{
									Name: "GCSID123",
									Size: 41,
									MD5: func() []byte {
										hash, err := base64.StdEncoding.DecodeString("cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705")
										if err != nil {
											t.Fatalf("error decoding hash string. details: %s", err)
										}
										return hash
									}(),
									Created: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								}, nil
							case 2:
								return &storage.ObjectAttrs{
									Name: "GCSID124",
									Size: 72,
									MD5: func() []byte {
										hash, err := base64.StdEncoding.DecodeString("941ba830a21740a0349e9d31e7ac8e6fe20a75fe6ecf0bdc23c9a19a10f2a2e0")
										if err != nil {
											t.Fatalf("error decoding hash string. details: %s", err)
										}
										return hash
									}(),
									Created: time.Date(2017, 9, 13, 13, 27, 53, 0, time.UTC),
								}, nil
							default:
								return nil, iterator.Done
							}
						}
					}(),
				},
			},
			expected: []cloud.Backup{
				{
					ID:        "GCSID123",
					CreatedAt: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
					Checksum:  "cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705",
					VaultName: "backup",
					Size:      41,
				},
				{
					ID:        "GCSID124",
					CreatedAt: time.Date(2017, 9, 13, 13, 27, 53, 0, time.UTC),
					Checksum:  "941ba830a21740a0349e9d31e7ac8e6fe20a75fe6ecf0bdc23c9a19a10f2a2e0",
					VaultName: "backup",
					Size:      72,
				},
			},
		},
		{
			description: "it should detect an error while iterating over the results",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObjects: func(ctx gcscontext.Context, q *storage.Query) *storage.ObjectIterator {
						return &storage.ObjectIterator{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockIterate: func() func(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
						var i = 0
						return func(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
							i++
							switch i {
							case 1:
								return &storage.ObjectAttrs{
									Name: "GCSID123",
									Size: 41,
									MD5: func() []byte {
										hash, err := base64.StdEncoding.DecodeString("cb63324d2c35cdfcb4521e15ca4518bd0ed9dc2364a9f47de75151b3f9b4b705")
										if err != nil {
											t.Fatalf("error decoding hash string. details: %s", err)
										}
										return hash
									}(),
									Created: time.Date(2016, 12, 27, 8, 14, 53, 0, time.UTC),
								}, nil
							default:
								return nil, errors.New("generic error iterating")
							}
						}
					}(),
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeIterating,
				Err:  errors.New("generic error iterating"),
			},
		},
		{
			description: "it should detect when the user cancel the listing",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObjects: func(ctx gcscontext.Context, q *storage.Query) *storage.ObjectIterator {
						return &storage.ObjectIterator{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockIterate: func(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
						// sleep for a small amount of time to allow the task to be
						// cancelled
						select {
						case <-time.After(200 * time.Millisecond):
						// do nothing
						case <-ctx.Done():
							return nil, ctx.Err()
						}

						return nil, iterator.Done
					},
				},
			},
			goFunc: func() {
				// wait for the send task to start
				time.Sleep(100 * time.Millisecond)
				cancel()
			},
			expectedError: &cloud.Error{
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

			backups, err := scenario.gcs.List(ctx)
			if !reflect.DeepEqual(scenario.expected, backups) {
				t.Errorf("backups don't match.\n%s", Diff(scenario.expected, backups))
			}
			if !cloud.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestGCS_Get(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	scenarios := []struct {
		description   string
		ids           []string
		gcs           cloud.GCS
		goFunc        func()
		expected      map[string]string
		expectedError error
	}{
		{
			description: "it should retrieve a backup correctly",
			ids:         []string{"GCSID123"},
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockRead: func(ctx gcscontext.Context, obj *storage.ObjectHandle, w io.Writer) error {
						if _, err := w.Write([]byte("This is a test")); err != nil {
							return err
						}
						return nil
					},
				},
			},
			expected: map[string]string{
				"GCSID123": path.Join(os.TempDir(), "backup-GCSID123.tar"),
			},
		},
		{
			description: "it should detect an error while reading the object",
			ids:         []string{"GCSID123"},
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockRead: func(ctx gcscontext.Context, obj *storage.ObjectHandle, w io.Writer) error {
						return errors.New("error copying object")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "GCSID123",
				Code: cloud.ErrorCodeDownloadingArchive,
				Err:  errors.New("error copying object"),
			},
		},
		{
			description: "it should detect when the download action is cancelled by the user",
			ids:         []string{"GCSID123"},
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockRead: func(ctx gcscontext.Context, obj *storage.ObjectHandle, w io.Writer) error {
						// sleep for a small amount of time to allow the task to be
						// cancelled
						select {
						case <-time.After(200 * time.Millisecond):
						// do nothing
						case <-ctx.Done():
							return ctx.Err()
						}

						if _, err := w.Write([]byte("This is a test")); err != nil {
							return err
						}
						return nil
					},
				},
			},
			goFunc: func() {
				// wait for the send task to start
				time.Sleep(100 * time.Millisecond)
				cancel()
			},
			expectedError: &cloud.Error{
				ID:   "GCSID123",
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

			filenames, err := scenario.gcs.Get(ctx, scenario.ids...)
			if !reflect.DeepEqual(scenario.expected, filenames) {
				t.Errorf("filenames don't match.\n%s", Diff(scenario.expected, filenames))
			}
			if !cloud.ErrorEqual(scenario.expectedError, err) && !cloud.JobsErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestGCS_Remove(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	scenarios := []struct {
		description   string
		id            string
		gcs           cloud.GCS
		goFunc        func()
		expectedError error
	}{
		{
			description: "it should remove a backup correctly",
			id:          "GCSID123",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockDelete: func(ctx gcscontext.Context, obj *storage.ObjectHandle) error {
						return nil
					},
				},
			},
		},
		{
			description: "it should detect an error while removing the object",
			id:          "GCSID123",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockDelete: func(ctx gcscontext.Context, obj *storage.ObjectHandle) error {
						return errors.New("error removing object")
					},
				},
			},
			expectedError: &cloud.Error{
				ID:   "GCSID123",
				Code: cloud.ErrorCodeRemovingArchive,
				Err:  errors.New("error removing object"),
			},
		},
		{
			description: "it should detect when the remove action is cancelled by the user",
			id:          "GCSID123",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
				Bucket: mockGCSBucket{
					mockObject: func(name string) *storage.ObjectHandle {
						return &storage.ObjectHandle{}
					},
				},
				BucketName: "backup",
				ObjectHandler: mockGCSObjectHandler{
					mockDelete: func(ctx gcscontext.Context, obj *storage.ObjectHandle) error {
						// sleep for a small amount of time to allow the task to be
						// cancelled
						select {
						case <-time.After(200 * time.Millisecond):
						// do nothing
						case <-ctx.Done():
							return ctx.Err()
						}

						return nil
					},
				},
			},
			goFunc: func() {
				// wait for the send task to start
				time.Sleep(100 * time.Millisecond)
				cancel()
			},
			expectedError: &cloud.Error{
				ID:   "GCSID123",
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

			err := scenario.gcs.Remove(ctx, scenario.id)
			if !cloud.ErrorEqual(scenario.expectedError, err) && !cloud.JobsErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestGCS_Close(t *testing.T) {
	scenarios := []struct {
		description   string
		gcs           cloud.GCS
		expectedError error
	}{
		{
			description: "it should close gcs connection correctly",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return nil
					},
				},
			},
		},
		{
			description: "it should ignore when instance was not initialized",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
			},
		},
		{
			description: "it should ignore when connection is nil",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
			},
		},
		{
			description: "it should detect an error while closing gcs connection",
			gcs: cloud.GCS{
				Logger: mockLogger{
					mockDebug:  func(args ...interface{}) {},
					mockDebugf: func(format string, args ...interface{}) {},
					mockInfo:   func(args ...interface{}) {},
					mockInfof:  func(format string, args ...interface{}) {},
				},
				Client: mockGCSClient{
					mockClose: func() error {
						return errors.New("error closing connection")
					},
				},
			},
			expectedError: &cloud.Error{
				Code: cloud.ErrorCodeClosingConnection,
				Err:  errors.New("error closing connection"),
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			err := scenario.gcs.Close()
			if !cloud.ErrorEqual(scenario.expectedError, err) && !cloud.JobsErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected: “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

type mockGCSClient struct {
	mockClose func() error
}

func (m mockGCSClient) Close() error {
	return m.mockClose()
}

type mockGCSBucket struct {
	mockObject  func(name string) *storage.ObjectHandle
	mockObjects func(ctx gcscontext.Context, q *storage.Query) *storage.ObjectIterator
	mockAttrs   func(ctx gcscontext.Context) (*storage.BucketAttrs, error)
}

func (m mockGCSBucket) Object(name string) *storage.ObjectHandle {
	return m.mockObject(name)
}

func (m mockGCSBucket) Objects(ctx gcscontext.Context, q *storage.Query) *storage.ObjectIterator {
	return m.mockObjects(ctx, q)
}

func (m mockGCSBucket) Attrs(ctx gcscontext.Context) (*storage.BucketAttrs, error) {
	return m.mockAttrs(ctx)
}

type mockGCSObjectHandler struct {
	mockRead    func(ctx gcscontext.Context, obj *storage.ObjectHandle, w io.Writer) error
	mockWrite   func(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error
	mockAttrs   func(ctx gcscontext.Context, obj *storage.ObjectHandle) (*storage.ObjectAttrs, error)
	mockDelete  func(ctx gcscontext.Context, obj *storage.ObjectHandle) error
	mockIterate func(it *storage.ObjectIterator) (*storage.ObjectAttrs, error)
}

func (m mockGCSObjectHandler) Read(ctx gcscontext.Context, obj *storage.ObjectHandle, w io.Writer) error {
	return m.mockRead(ctx, obj, w)
}

func (m mockGCSObjectHandler) Write(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error {
	return m.mockWrite(ctx, obj, r)
}

func (m mockGCSObjectHandler) Attrs(ctx gcscontext.Context, obj *storage.ObjectHandle) (*storage.ObjectAttrs, error) {
	return m.mockAttrs(ctx, obj)
}

func (m mockGCSObjectHandler) Delete(ctx gcscontext.Context, obj *storage.ObjectHandle) error {
	return m.mockDelete(ctx, obj)
}

func (m mockGCSObjectHandler) Iterate(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
	return m.mockIterate(it)
}
