package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
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

//// actual implementation of store

type storeGCS struct {
	bkt *storage.BucketHandle
	ctx context.Context
}

func (store *storeGCS) getFile(name string) fileObject {
	return &fileObjectGCS{obj: store.bkt.Object(name), ctx: store.ctx}
}

func (store *storeGCS) getFiles(prefix string) []fileAttributes {
	ctx, _ := context.WithTimeout(store.ctx, contextTimeout)
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

//// actual implementation of fileObject
type fileObjectGCS struct {
	obj *storage.ObjectHandle
	ctx context.Context
}

func (file *fileObjectGCS) getWriter() io.WriteCloser {
	ctx, _ := context.WithTimeout(file.ctx, contextTimeout)
	return file.obj.NewWriter(ctx)
}

func (file *fileObjectGCS) getReader() (io.ReadCloser, error) {
	ctx, _ := context.WithTimeout(file.ctx, contextTimeout)
	return file.obj.NewReader(ctx)
}

func (file *fileObjectGCS) deleteFile() error {
	ctx, _ := context.WithTimeout(file.ctx, contextTimeout)
	return file.obj.Delete(ctx)
}

func (file *fileObjectGCS) getAttrs() (fileAttributes, error) {
	ctx, _ := context.WithTimeout(file.ctx, contextTimeout)
	attr, err := file.obj.Attrs(ctx)
	return &fileAttributesGCS{attr}, err
}

//// actual implementation of fileAttributes

type fileAttributesGCS struct {
	attrs *storage.ObjectAttrs
}

func (fileAttr *fileAttributesGCS) getName() string {
	return fileAttr.attrs.Name
}

func (fileAttr *fileAttributesGCS) getMD5() []byte {
	return fileAttr.attrs.MD5
}

//// implementation of API purely for testing purposes

//// testStore implements the store interface for testing
type testStore struct {
	files map[string]obj
}

func (fsto *testStore) getFile(name string) fileObject {
	if file, ok := fsto.files[name]; ok {
		return file
	}
	return obj{name: name, md5: nil, data: bytes.NewBuffer(nil), fsto: fsto}
}

func (fsto *testStore) getFiles(prefix string) []fileAttributes {
	var attrSlice []fileAttributes = nil
	for key, object := range fsto.files {
		if strings.HasPrefix(key, prefix) {
			attrSlice = append(attrSlice, object)
		}
	}
	return attrSlice

}

//// Obj struct implements both the attrs and the object interfaces for testing
type obj struct {
	name string
	md5  []byte
	data *bytes.Buffer
	fsto *testStore
}

func (file obj) getWriter() io.WriteCloser {
	return file
}

func (file obj) getReader() (io.ReadCloser, error) {
	return file, nil
}

func (file obj) Write(p []byte) (n int, err error) {
	if strings.HasSuffix(file.name, "copyFail") {
		return 0, errors.New("Example Copy Error")
	}
	return file.data.Write(p)
}

func (file obj) Read(p []byte) (n int, err error) {
	return file.data.Read(p)
}

func (file obj) Close() error {
	file.md5 = []byte("NEW FILE")
	file.fsto.files[file.name] = file
	return nil
}

func (file obj) deleteFile() error {
	if strings.HasSuffix(file.name, "deleteFail") {
		return errors.New("Couldn't delete file!")
	}
	return nil
}

func (o obj) getAttrs() (fileAttributes, error) {
	if o.md5 != nil {
		return o, nil
	}
	return nil, errors.New("Expected Error Output")
}

func (file obj) getName() string {
	return file.name
}
func (file obj) getMD5() []byte {
	return file.md5
}

//// End of stubs for testing
