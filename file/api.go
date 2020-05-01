// Package file exports a generic file interface that we use to access Google
// Cloud Storage. None of the functions here are unit-testable because they are
// all either interfaces or connect to Google Cloud Storage, which cannot be
// unit tested.
package file

import (
	"flag"
	"io"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/iterator"
)

var (
	gcsCopyTimeout = flag.Duration("file.gcscopytimeout", 2*time.Minute, "Maximum time to wait for a file to copy on GCS")
)

// Store is the mockable interface to the functionality we need from CGS.
type Store interface {
	GetFile(name string) Object
	NamesToMD5(ctx context.Context, prefix string) map[string][]byte
}

// Object is the mockable interface to the functionality we need from a single CGS object.
type Object interface {
	GetWriter(ctx context.Context) io.WriteCloser
	DeleteFile(ctx context.Context) error
	CopyTo(ctx context.Context, filename string) error
}

// GCSStore adapts a bucket handle into a file.Store.
func GCSStore(bkt *storage.BucketHandle) Store {
	return &storeGCS{Bkt: bkt}
}

/// GCS implementation of file.Store

type storeGCS struct {
	Bkt *storage.BucketHandle
}

func (store *storeGCS) GetFile(name string) Object {
	return &fileObjectGCS{bkt: store.Bkt, obj: store.Bkt.Object(name)}
}

func (store *storeGCS) NamesToMD5(ctx context.Context, prefix string) map[string][]byte {
	objects := store.Bkt.Objects(ctx, &storage.Query{Prefix: ""})
	namesAndMD5s := make(map[string][]byte)
	for object, err := objects.Next(); err != iterator.Done; object, err = objects.Next() {
		if err != nil {
			metrics.DownloaderErrorCount.
				With(prometheus.Labels{"source": "Unknown Error in iterator in checkIfHashIsUniqueInList"}).
				Inc()
		}
		namesAndMD5s[object.Name] = object.MD5
	}
	return namesAndMD5s

}

// GCS implementation of file.Object
type fileObjectGCS struct {
	bkt *storage.BucketHandle
	obj *storage.ObjectHandle
}

func (file *fileObjectGCS) GetWriter(ctx context.Context) io.WriteCloser {
	return file.obj.NewWriter(ctx)
}

func (file *fileObjectGCS) DeleteFile(ctx context.Context) error {
	return file.obj.Delete(ctx)
}

func (file *fileObjectGCS) CopyTo(ctx context.Context, filename string) error {
	ctx, cancel := context.WithTimeout(ctx, *gcsCopyTimeout)
	defer cancel()
	dst := file.bkt.Object(filename)
	_, err := dst.CopierFrom(file.obj).Run(ctx)
	return err
}
