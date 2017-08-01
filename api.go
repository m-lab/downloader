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
	getFiles(prefix string) []fileObject
}

type fileObject interface {
	getWriter() io.WriteCloser
	deleteFile() error
	getName() (string, error)
	getMD5() ([]byte, error)
}

//// actual implementation of store

type storeGCS struct {
	bkt *storage.BucketHandle
}

func (store *storeGCS) getFile(name string) fileObject {
	return &fileObjectGCS{obj: store.bkt.Object(name)}
}

func (store *storeGCS) getFiles(prefix string) []fileObject {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	objects := store.bkt.Objects(ctx, &storage.Query{"", prefix, false})
	var attrs []fileObject = nil
	for object, err := objects.Next(); err != iterator.Done; object, err = objects.Next() {
		if err != nil {
			DownloaderErrorCount.With(prometheus.Labels{"source": "Unkown Error in iterator in checkIfHashIsUniqueInList"}).Inc()
		}
		attrs = append(attrs, store.getFile(object.Name))
	}
	return attrs

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

func (file *fileObjectGCS) getName() (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	attrs, err := file.obj.Attrs(ctx)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Couldn't get GCS File Attributes for filename generation"}).Inc()
		return "", err
	}
	return attrs.Name, nil
}

func (file *fileObjectGCS) getMD5() ([]byte, error) {
	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	attrs, err := file.obj.Attrs(ctx)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Couldn't get GCS File Attributes for hash generation"}).Inc()
		return nil, err
	}
	return attrs.MD5, nil
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

func (fsto *testStore) getFiles(prefix string) []fileObject {
	var attrSlice []fileObject = nil
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

func (file obj) Write(p []byte) (n int, err error) {
	if strings.HasSuffix(file.name, "copyFail") {
		return 0, errors.New("Example Copy Error")
	}
	return file.data.Write(p)
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

func (file obj) getName() (string, error) {
	if file.md5 != nil {
		return file.name, nil
	}
	return "", errors.New("Expected Error Output")
}
func (file obj) getMD5() ([]byte, error) {
	if file.md5 != nil {
		return file.md5, nil
	}
	return nil, errors.New("Expected Error Output")
}

//// End of stubs for testing
