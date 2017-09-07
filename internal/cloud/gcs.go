package cloud

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
	"google.golang.org/api/iterator"
)

type GoogleCloudStorage struct {
	Logger log.Logger
	client *storage.Client
	bucket *storage.BucketHandle
}

type GoogleCloudStorageConfig struct {
	Project    string
	BucketName string
}

func NewGoogleCloudStorage(logger log.Logger, ctx context.Context, config GoogleCloudStorageConfig) (*GoogleCloudStorage, error) {
	c, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.WithStack(newError("", ErrorCodeInitializingSession, err))
	}

	bkt := c.Bucket(config.BucketName)
	err = bkt.Create(ctx, config.Project, &storage.BucketAttrs{
		StorageClass: "COLDLINE",
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

func (g *GoogleCloudStorage) Send(ctx context.Context, filename string) (Backup, error) {
	g.Logger.Debugf("cloud: sending file “%s” to google cloud", filename)

	f, err := os.Open(filename)
	if err != nil {
		return Backup{}, errors.WithStack(newError("", ErrorCodeOpeningArchive, err))
	}

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
			// TODO
		}

		// TODO: Vault name?
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

func (g *GoogleCloudStorage) Get(ctx context.Context, ids ...string) (map[string]string, error) {
	g.Logger.Debugf("cloud: retrieving archives “%v” from the google cloud", ids)

	// TODO

	return nil, nil
}

func (g *GoogleCloudStorage) Remove(ctx context.Context, id string) error {
	g.Logger.Debugf("cloud: removing archive %s from the google cloud", id)

	// TODO

	g.Logger.Infof("cloud: backup “%s” removed successfully from the google cloud", id)
	return nil
}
