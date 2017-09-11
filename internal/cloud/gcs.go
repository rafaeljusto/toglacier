package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSConfig stores all necessary parameters to initialize a GCS session.
type GCSConfig struct {
	Project     string
	Bucket      string
	AccountFile string
}

// GCS is the Google solution for storing the backups in the cloud. It uses the
// Google Cloud Storage service, as it can allow large files for a small price
// (coldline recommended).
type GCS struct {
	Logger log.Logger
	client *storage.Client
	bucket *storage.BucketHandle
}

// NewGCS initializes the Google Cloud Storage bucket. On error it will return
// an Error type. To retrieve the desired error you can do:
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
func NewGCS(ctx context.Context, logger log.Logger, config GCSConfig) (*GCS, error) {
	c, err := storage.NewClient(ctx, option.WithServiceAccountFile(config.AccountFile))
	if err != nil {
		return nil, errors.WithStack(newError("", ErrorCodeInitializingSession, err))
	}

	bkt := c.Bucket(config.Bucket)
	return &GCS{
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
func (g *GCS) Send(ctx context.Context, filename string) (Backup, error) {
	g.Logger.Debugf("cloud: sending file “%s” to google cloud", filename)

	f, err := os.Open(filename)
	if err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeOpeningArchive, err))
	}

	// id will be defined as the filename hash with the current epoch, this is
	// important to avoid duplicated ids.
	id := fmt.Sprintf("%s%d", sha256.Sum256([]byte(filename)), time.Now().UnixNano())
	w := g.bucket.Object(id).NewWriter(ctx)
	w.ContentType = "application/octet-stream"

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
func (g *GCS) List(ctx context.Context) ([]Backup, error) {
	g.Logger.Debug("cloud: retrieving list of archives from the google cloud")

	bucketAttrs, err := g.bucket.Attrs(ctx)
	if err != nil {
		return nil, errors.WithStack(newError("", ErrorCodeArchiveInfo, err))
	}

	var backups []Backup
	it := g.bucket.Objects(ctx, nil)

	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			return nil, errors.WithStack(newError("", ErrorCodeIterating, err))
		}

		backups = append(backups, Backup{
			ID:        objAttrs.Name,
			CreatedAt: objAttrs.Created,
			Checksum:  base64.StdEncoding.EncodeToString(objAttrs.MD5),
			VaultName: bucketAttrs.Name,
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
func (g *GCS) Get(ctx context.Context, ids ...string) (map[string]string, error) {
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

func (g *GCS) get(ctx context.Context, id string, waitGroup *sync.WaitGroup, result chan<- jobResult) {
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

	r, err := g.bucket.Object(id).NewReader(ctx)
	if err != nil {
		result <- jobResult{
			id:  id,
			err: errors.WithStack(newError(id, ErrorCodeDownloadingArchive, err)),
		}
		return
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
func (g *GCS) Remove(ctx context.Context, id string) error {
	g.Logger.Debugf("cloud: removing archive %s from the google cloud", id)

	if err := g.bucket.Object(id).Delete(ctx); err != nil {
		return errors.WithStack(newError(id, ErrorCodeRemovingArchive, err))
	}

	g.Logger.Infof("cloud: backup “%s” removed successfully from the google cloud", id)
	return nil
}

// Close ends the Google Cloud session.
func (g *GCS) Close() error {
	if g == nil {
		return nil
	}

	if err := g.client.Close(); err != nil {
		return errors.WithStack(newError("", ErrorCodeClosingConnection, err))
	}

	return nil
}
