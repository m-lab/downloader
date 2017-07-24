package main

import (
	"io"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/iterator"
)

const contextTimeout time.Duration = 2 * time.Minute

type store interface {
	getFile(name string) fileObject
	getFiles(prefix string) []fileAttributes
}

type fileObject interface {
	getWriter() io.WriteCloser
	getReader() (io.ReadCloser, error)
	deleteFile() error
	getAttrs() (fileAttributes, error)
}

type fileAttributes interface {
	getName() string
	getMD5() []byte
}

//// Implementation of store

type storeGCS struct {
	bkt *storage.BucketHandle
	ctx context.Context
}

func (store *storeGCS) getFile(name string) fileObject {
	return &fileObjectGCS{obj: store.bkt.Object(name), ctx: store.ctx}
}

func (store *storeGCS) getFiles(prefix string) []fileAttributes {
	ctx, cancel := context.WithTimeout(store.ctx, contextTimeout)
	defer cancel()
	objects := store.bkt.Objects(ctx, &storage.Query{"", prefix, false})
	var attrs []fileAttributes = nil
	for object, err := objects.Next(); err != iterator.Done; object, err = objects.Next() {
		if err != nil {
			DownloaderErrorCount.With(prometheus.Labels{"source": "Unkown Error in iterator in checkIfHashIsUniqueInList"}).Inc()
		}
		attrs = append(attrs, &fileAttributesGCS{object})
	}
	return attrs

}

//// Implementation of fileObject
type fileObjectGCS struct {
	obj *storage.ObjectHandle
	ctx context.Context
}

func (file *fileObjectGCS) getWriter() io.WriteCloser {
	ctx, cancel := context.WithTimeout(file.ctx, contextTimeout)
	defer cancel()
	return file.obj.NewWriter(ctx)
}

func (file *fileObjectGCS) getReader() (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(file.ctx, contextTimeout)
	defer cancel()
	return file.obj.NewReader(ctx)
}

func (file *fileObjectGCS) deleteFile() error {
	ctx, cancel := context.WithTimeout(file.ctx, contextTimeout)
	defer cancel()
	return file.obj.Delete(ctx)
}

func (file *fileObjectGCS) getAttrs() (fileAttributes, error) {
	ctx, cancel := context.WithTimeout(file.ctx, contextTimeout)
	defer cancel()
	attr, err := file.obj.Attrs(ctx)
	return &fileAttributesGCS{attr}, err
}

//// Implementation of fileAttributes

type fileAttributesGCS struct {
	attrs *storage.ObjectAttrs
}

func (fileAttr *fileAttributesGCS) getName() string {
	return fileAttr.attrs.Name
}

func (fileAttr *fileAttributesGCS) getMD5() []byte {
	return fileAttr.attrs.MD5
}
