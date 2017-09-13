package cloud_test

import (
	"context"
	"encoding/base64"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	gcscontext "golang.org/x/net/context"
	"google.golang.org/api/iterator"
)

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

func (m mockGCSObjectHandler) Iterate(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
	return m.mockIterate(it)
}
