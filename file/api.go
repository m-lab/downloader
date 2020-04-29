// The file package exports a generic file interface that we use to
// access Google Cloud Storage. None of the functions here are
// unit-testable because they are all either interfaces or connect to
// Google Cloud Storage, which cannot be unit tested.
package file

import (
	"io"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/iterator"
)

const contextTimeout time.Duration = 2 * time.Minute

type FileStore interface {
	GetFile(name string) FileObject
	NamesToMD5(prefix string) map[string][]byte
}

type FileObject interface {
	GetWriter() io.WriteCloser
	DeleteFile() error
	CopyTo(filename string) error
}

//// actual implementation of store

type StoreGCS struct {
	Bkt *storage.BucketHandle
}

func (store *StoreGCS) GetFile(name string) FileObject {
	return &FileObjectGCS{bkt: store.Bkt, obj: store.Bkt.Object(name)}
}

func (store *StoreGCS) NamesToMD5(prefix string) map[string][]byte {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	objects := store.Bkt.Objects(ctx, &storage.Query{Prefix: ""})
	var namesAndMD5s map[string][]byte = make(map[string][]byte)
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

//// actual implementation of fileObject
type FileObjectGCS struct {
	bkt *storage.BucketHandle
	obj *storage.ObjectHandle
}

func (file *FileObjectGCS) GetWriter() io.WriteCloser {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	return file.obj.NewWriter(ctx)
}

func (file *FileObjectGCS) DeleteFile() error {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	return file.obj.Delete(ctx)
}

func (file *FileObjectGCS) CopyTo(filename string) error {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	dst := file.bkt.Object(filename)
	_, err := dst.CopierFrom(file.obj).Run(ctx)
	return err
}
