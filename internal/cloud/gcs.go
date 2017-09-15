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
	gcscontext "golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSConfig stores all necessary parameters to initialize a GCS session.
type GCSConfig struct {
	Project     string
	Bucket      string
	AccountFile string
}

// GCSClient contains all used methods from the Google Cloud Storage SDK client
// that is used. This is necessary to make it easy to test the components
// locally.
type GCSClient interface {
	Close() error
}

// GCSBucket contains all used methods from the Google Cloud Storage SDK bucket
// that is used. This is necessary to make it easy to test the components
// locally.
type GCSBucket interface {
	Object(name string) *storage.ObjectHandle
	Objects(ctx gcscontext.Context, q *storage.Query) *storage.ObjectIterator
	Attrs(ctx gcscontext.Context) (*storage.BucketAttrs, error)
}

// GCSObjectHandler contains all operations performed with the Google Cloud
// Storage SDK objects. This is necessary to make it easy to test the components
// locally.
type GCSObjectHandler interface {
	Read(ctx gcscontext.Context, obj *storage.ObjectHandle, w io.Writer) error
	Write(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error
	Attrs(ctx gcscontext.Context, obj *storage.ObjectHandle) (*storage.ObjectAttrs, error)
	Delete(ctx gcscontext.Context, obj *storage.ObjectHandle) error
	Iterate(it *storage.ObjectIterator) (*storage.ObjectAttrs, error)
}

type gcsObjectHandler struct{}

func (g gcsObjectHandler) Read(ctx gcscontext.Context, obj *storage.ObjectHandle, w io.Writer) error {
	r, err := obj.NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(w, r)
	return err
}

func (g gcsObjectHandler) Write(ctx gcscontext.Context, obj *storage.ObjectHandle, r io.Reader) error {
	w := obj.NewWriter(ctx)
	w.ContentType = "application/octet-stream"

	if _, err := io.Copy(w, r); err != nil {
		return err
	}

	return w.Close()
}

func (g gcsObjectHandler) Attrs(ctx gcscontext.Context, obj *storage.ObjectHandle) (*storage.ObjectAttrs, error) {
	return obj.Attrs(ctx)
}

func (g gcsObjectHandler) Delete(ctx gcscontext.Context, obj *storage.ObjectHandle) error {
	return obj.Delete(ctx)
}

func (g gcsObjectHandler) Iterate(it *storage.ObjectIterator) (*storage.ObjectAttrs, error) {
	return it.Next()
}

// GCS is the Google solution for storing the backups in the cloud. It uses the
// Google Cloud Storage service, as it can allow large files for a small price
// (coldline recommended).
type GCS struct {
	Logger        log.Logger
	Client        GCSClient
	Bucket        GCSBucket
	BucketName    string
	ObjectHandler GCSObjectHandler
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

	return &GCS{
		Logger:        logger,
		Client:        c,
		Bucket:        c.Bucket(config.Bucket),
		BucketName:    config.Bucket,
		ObjectHandler: gcsObjectHandler{},
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
	defer f.Close()

	// id will be defined as the filename hash with the current epoch, this is
	// important to avoid duplicated ids
	id := fmt.Sprintf("%s%d", sha256.Sum256([]byte(filename)), time.Now().UnixNano())

	if err = g.ObjectHandler.Write(ctx, g.Bucket.Object(id), f); err != nil {
		return Backup{}, errors.WithStack(g.checkCancellation(newError("", ErrorCodeSendingArchive, err)))
	}

	attrs, err := g.ObjectHandler.Attrs(ctx, g.Bucket.Object(id))
	if err != nil {
		// TODO: better error code?
		return Backup{}, errors.WithStack(g.checkCancellation(newError("", ErrorCodeArchiveInfo, err)))
	}

	return Backup{
		ID:        attrs.Name,
		CreatedAt: attrs.Created,
		Checksum:  base64.StdEncoding.EncodeToString(attrs.MD5),
		VaultName: g.BucketName,
		Size:      attrs.Size,
		Location:  LocationGCS,
	}, nil
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

	var backups []Backup
	it := g.Bucket.Objects(ctx, nil)

	for {
		objAttrs, err := g.ObjectHandler.Iterate(it)
		if err == iterator.Done {
			break
		}

		if err != nil {
			return nil, errors.WithStack(g.checkCancellation(newError("", ErrorCodeIterating, err)))
		}

		backups = append(backups, Backup{
			ID:        objAttrs.Name,
			CreatedAt: objAttrs.Created,
			Checksum:  base64.StdEncoding.EncodeToString(objAttrs.MD5),
			VaultName: g.BucketName,
			Size:      objAttrs.Size,
			Location:  LocationGCS,
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

	if err = g.ObjectHandler.Read(ctx, g.Bucket.Object(id), backup); err != nil {
		result <- jobResult{
			id:  id,
			err: errors.WithStack(g.checkCancellation(newError(id, ErrorCodeDownloadingArchive, err))),
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

	if err := g.ObjectHandler.Delete(ctx, g.Bucket.Object(id)); err != nil {
		return errors.WithStack(g.checkCancellation(newError(id, ErrorCodeRemovingArchive, err)))
	}

	g.Logger.Infof("cloud: backup “%s” removed successfully from the google cloud", id)
	return nil
}

// Close ends the Google Cloud session.
func (g *GCS) Close() error {
	if g == nil || g.Client == nil {
		return nil
	}

	if err := g.Client.Close(); err != nil {
		return errors.WithStack(newError("", ErrorCodeClosingConnection, err))
	}

	return nil
}

func (g *GCS) checkCancellation(err error) error {
	v, ok := err.(*Error)
	if !ok {
		return err
	}

	cancellation := errors.Cause(v.Err) == context.Canceled || errors.Cause(v.Err) == context.DeadlineExceeded

	if cancellation {
		g.Logger.Debug("operation cancelled by user")
		return newError(v.ID, ErrorCodeCancelled, v.Err)
	}

	return err
}
