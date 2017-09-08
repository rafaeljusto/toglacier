package cloud

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
	"google.golang.org/api/iterator"
)

const googleCloudStorageClass = "COLDLINE"

// GoogleCloudStorageConfig stores all necessary parameters to initialize a GCS
// session.
type GoogleCloudStorageConfig struct {
	Project    string
	BucketName string
}

// GoogleCloudStorage is the Google solution for storing the backups in the
// cloud. It uses the Google Cloud Storage Coldline service, as it allows large
// files for a small price.
type GoogleCloudStorage struct {
	Logger log.Logger
	client *storage.Client
	bucket *storage.BucketHandle
}

// NewGoogleCloudStorage initializes the Google Cloud Storage bucket. On error
// it will return an Error type. To retrieve the desired error you can do:
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
func NewGoogleCloudStorage(ctx context.Context, logger log.Logger, config GoogleCloudStorageConfig) (*GoogleCloudStorage, error) {
	c, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.WithStack(newError("", ErrorCodeInitializingSession, err))
	}

	bkt := c.Bucket(config.BucketName)
	err = bkt.Create(ctx, config.Project, &storage.BucketAttrs{
		StorageClass: googleCloudStorageClass,
	})

	if err != nil {
		return nil, errors.WithStack(newError("", ErrorCodeInitializingSession, err))
	}

	return &GoogleCloudStorage{
		Logger: logger,
		client: c,
		bucket: bkt,
	}, nil
}

// Send uploads the file to the cloud and return the backup archive information.
// If an error occurs it will be an Error type encapsulated in a traceable
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
//       default:
//         // unknown error
//       }
//     }
func (g *GoogleCloudStorage) Send(ctx context.Context, filename string) (Backup, error) {
	g.Logger.Debugf("cloud: sending file “%s” to google cloud", filename)

	f, err := os.Open(filename)
	if err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeOpeningArchive, err))
	}

	// TODO: better id for object?
	obj := g.bucket.Object(strconv.FormatInt(time.Now().UnixNano(), 10))
	w := obj.NewWriter(ctx)

	if _, err = io.Copy(w, f); err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeSendingArchive, err))
	}

	if err = w.Close(); err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeSendingArchive, err))
	}

	return Backup{}, nil
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
func (g *GoogleCloudStorage) List(ctx context.Context) ([]Backup, error) {
	g.Logger.Debug("cloud: retrieving list of archives from the google cloud")

	var backups []Backup

	it := g.bucket.Objects(ctx, nil)
	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			// TODO: define error
		}

		// TODO: vault name?
		backups = append(backups, Backup{
			ID:        objAttrs.Name,
			CreatedAt: objAttrs.Created,
			Checksum:  base64.StdEncoding.EncodeToString(objAttrs.MD5),
			Size:      objAttrs.Size,
		})
	}

	g.Logger.Info("cloud: remote backups listed successfully from the google cloud")
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
func (g *GoogleCloudStorage) Get(ctx context.Context, ids ...string) (map[string]string, error) {
	g.Logger.Debugf("cloud: retrieving archives “%v” from the google cloud", ids)

	var waitGroup sync.WaitGroup
	jobResults := make(chan jobResult, len(ids))

	for _, id := range ids {
		waitGroup.Add(1)
		go g.get(ctx, id, &waitGroup, jobResults)
	}

	waitGroup.Wait()

	filenames := make(map[string]string)
	for i := 0; i < len(ids); i++ {
		result := <-jobResults
		if result.err != nil {
			// TODO: if only one file failed we will stop it all?
			return nil, errors.WithStack(result.err)
		}
		filenames[result.id] = result.filename
	}
	return filenames, nil
}

func (g *GoogleCloudStorage) get(ctx context.Context, id string, waitGroup *sync.WaitGroup, result chan<- jobResult) {
	defer waitGroup.Done()

	backup, err := os.Create(path.Join(os.TempDir(), "backup-"+id+".tar"))
	if err != nil {
		result <- jobResult{
			id:  id,
			err: errors.WithStack(newError(id, ErrorCodeCreatingArchive, err)),
		}
		return
	}
	defer backup.Close()

	obj := g.bucket.Object(id)
	r, err := obj.NewReader(ctx)
	if err != nil {
		// TODO: define error
	}
	defer r.Close()

	if _, err := io.Copy(backup, r); err != nil {
		result <- jobResult{
			id:  id,
			err: errors.WithStack(newError(id, ErrorCodeCopyingData, err)),
		}
		return
	}

	g.Logger.Infof("cloud: backup “%s” retrieved successfully from the google cloud and saved in temporary file “%s”", id, backup.Name())

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
func (g *GoogleCloudStorage) Remove(ctx context.Context, id string) error {
	g.Logger.Debugf("cloud: removing archive %s from the google cloud", id)

	if err := g.bucket.Object(id).Delete(ctx); err != nil {
		return errors.WithStack(newError(id, ErrorCodeRemovingArchive, err))
	}

	g.Logger.Infof("cloud: backup “%s” removed successfully from the google cloud", id)
	return nil
}
