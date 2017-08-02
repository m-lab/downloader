package main

import (
	"io"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/iterator"
)

func main() {}

const contextTimeout time.Duration = 2 * time.Minute

type fileStore interface {
	getFile(name string) fileObject
	namesToMD5(prefix string) map[string][]byte
}

type fileObject interface {
	getWriter() io.WriteCloser
	deleteFile() error
}

//// actual implementation of store

type storeGCS struct {
	bkt *storage.BucketHandle
}

func (store *storeGCS) getFile(name string) fileObject {
	return &fileObjectGCS{obj: store.bkt.Object(name)}
}

func (store *storeGCS) namesToMD5(prefix string) map[string][]byte {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	objects := store.bkt.Objects(ctx, &storage.Query{"", prefix, false})
	var namesAndMD5s map[string][]byte = make(map[string][]byte)
	for object, err := objects.Next(); err != iterator.Done; object, err = objects.Next() {
		if err != nil {
			metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Unkown Error in iterator in checkIfHashIsUniqueInList"}).Inc()
		}
		namesAndMD5s[object.Name] = object.MD5
	}
	return namesAndMD5s

}

//// actual implementation of fileObject
type fileObjectGCS struct {
	obj *storage.ObjectHandle
}

func (file *fileObjectGCS) getWriter() io.WriteCloser {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	return file.obj.NewWriter(ctx)
}

func (file *fileObjectGCS) deleteFile() error {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	return file.obj.Delete(ctx)
}
