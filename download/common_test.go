package download_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	d "github.com/m-lab/downloader/download"
	"github.com/m-lab/downloader/file"
)

//// implementation of API purely for testing purposes

//// testStore implements the store interface for testing
type testStore struct {
	files map[string]*testFileObject
}

func (fsto *testStore) GetFile(name string) file.FileObject {
	if file, ok := fsto.files[name]; ok {
		return file
	}
	return &testFileObject{name: name, md5: nil, data: bytes.NewBuffer(nil), fsto: fsto}
}

func (fsto *testStore) NamesToMD5(prefix string) map[string][]byte {
	var attrMap map[string][]byte = make(map[string][]byte)
	for key, object := range fsto.files {
		if strings.HasPrefix(key, prefix) {
			attrMap[key] = object.md5
		}
	}
	return attrMap

}

//// Obj struct implements both the attrs and the object interfaces for testing
type testFileObject struct {
	name   string
	md5    []byte
	data   *bytes.Buffer
	fsto   *testStore
	copied bool
}

func (file *testFileObject) GetWriter() io.WriteCloser {
	return file
}

func (file *testFileObject) Write(p []byte) (n int, err error) {
	if strings.HasSuffix(file.name, "copyFail") {
		return 0, errors.New("Example Copy Error")
	}
	return file.data.Write(p)
}

func (file *testFileObject) Close() error {
	file.md5 = []byte("NEW FILE")
	file.fsto.files[file.name] = file
	return nil
}

func (file *testFileObject) DeleteFile() error {
	if strings.HasSuffix(file.name, "deleteFail") {
		return errors.New("Couldn't delete file!")
	}
	return nil
}

func (file *testFileObject) CopyTo(filename string) error {
	file.copied = true
	return nil
}

//// End of stubs for testing

func TestGenUniformSleepTime(t *testing.T) {
	rand.Seed(0)
	testVals := make([]time.Duration, 5)
	testVals[0] = time.Duration(35210824549835)
	testVals[1] = time.Duration(25127497228230)
	testVals[2] = time.Duration(31045770218813)
	testVals[3] = time.Duration(22382551290235)
	testVals[4] = time.Duration(26893255775507)
	for i := 0; i < 5; i++ {
		testRes := d.GenUniformSleepTime(8*time.Hour, 4*time.Hour)
		if int64(testVals[i].Seconds()) != int64(testRes.Seconds()) {
			t.Errorf("Expected %s, got %s.", testVals[i], testRes)
		}
	}
}

func TestDownload(t *testing.T) {
	tests := []struct {
		dc      d.Config
		postfix string
		resBool bool
		resErr  error
	}{
		{
			dc: d.Config{
				URL:         "Fill me",
				Store:       &testStore{map[string]*testFileObject{}},
				PathPrefix:  "pre/",
				URLRegexp:   regexp.MustCompile(`.*()(/.*)`),
				DedupRegexp: regexp.MustCompile(`(.*)`),
			},
			postfix: "portGarbage",
			resBool: false,
			resErr:  errors.New("invalid URL port"),
		},
		{
			dc: d.Config{
				URL:         "Fill me",
				Store:       &testStore{map[string]*testFileObject{}},
				PathPrefix:  "pre/",
				URLRegexp:   regexp.MustCompile(`.*()(/.*)`),
				DedupRegexp: regexp.MustCompile(`(.*)`),
			},
			postfix: "/file.error",
			resBool: false,
			resErr:  errors.New("non-200 error"),
		},
		{
			dc: d.Config{
				URL:         "Fill me",
				Store:       &testStore{map[string]*testFileObject{}},
				PathPrefix:  "pre/",
				URLRegexp:   regexp.MustCompile(`.*()(/.*)`),
				DedupRegexp: regexp.MustCompile(`(.*)`),
			},
			postfix: "/file.copyFail",
			resBool: false,
			resErr:  errors.New("File copy error"),
		},
		{
			dc: d.Config{
				URL: "Fill me",
				Store: &testStore{map[string]*testFileObject{
					"pre/file.del/dup": {name: "pre/file.del/dup", data: bytes.NewBuffer(nil), md5: []byte("NEW FILE")},
				}},
				PathPrefix:  "pre/",
				URLRegexp:   regexp.MustCompile(`.*()(/.*)`),
				DedupRegexp: regexp.MustCompile(`(pre/)`),
			},
			postfix: "/file.deleteFail",
			resBool: true,
			resErr:  errors.New("Couldn't Delete File"),
		},
		{
			dc: d.Config{
				URL:         "Fill me",
				Store:       &testStore{map[string]*testFileObject{}},
				PathPrefix:  "pre/",
				URLRegexp:   regexp.MustCompile(`.*()(/.*)`),
				DedupRegexp: regexp.MustCompile(`(.*)`),
			},
			postfix: "/file.success",
			resBool: false,
			resErr:  nil,
		},
	}
	if err, force := d.Download(nil); err == nil || force != true {
		t.Errorf("Expected an improper Config error and unrecoverable, got nil or no recoverable.")
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "error") {
			http.Error(w, "Test Error", 404)
			return
		}
		fmt.Fprint(w, "Stuff")
	}))
	for _, test := range tests {
		test.dc.URL = ts.URL + test.postfix
		err, resBool := d.Download(test.dc)
		if test.resBool != resBool || (err != nil && test.resErr == nil) || (err == nil && test.resErr != nil) {
			t.Errorf("Expected %s, %t got %s, %t", test.resErr, test.resBool, err, resBool)
		}

	}

}

type retryTest struct {
	force    bool
	numError int
}

// RunFunctionWithRetry takes an arbitrary function and an interface{} that will be passed to it.
// So for this test, we create an anonymous function,
// which will return a certain number of errors before the call succeedes.
// The function will also return whether or not the error is unrecoverable,
// based on what we pass into it. This allows us to test all three possible paths for
// RunFunctionWithRetry: Run and succeed, run and fail until timeout,
// run and fail a few times before suceeding, and run and fail with an error that forces an immediate exit
func TestRunFunctionWithRetry(t *testing.T) {
	tests := []struct {
		data         *retryTest
		retryTimeMin time.Duration
		retryTimeMax time.Duration
		res          error
	}{
		{
			data:         &retryTest{force: false, numError: 0}, // Run and succeed
			retryTimeMin: 0,
			retryTimeMax: 0,
			res:          nil,
		},
		{
			data:         &retryTest{force: false, numError: 1}, // Run and succeed
			retryTimeMin: 1,
			retryTimeMax: 0,
			res:          errors.New("runFunction Error 1"),
		},
		{
			data:         &retryTest{force: false, numError: 100}, // Fail and timeout
			retryTimeMin: 1 * time.Nanosecond,
			retryTimeMax: 50 * time.Nanosecond,
			res:          errors.New("runFunction Error 2"),
		},
		{
			data:         &retryTest{force: false, numError: 10}, // Run, fail, then succeed
			retryTimeMin: 1 * time.Nanosecond,
			retryTimeMax: 5000 * time.Nanosecond,
			res:          nil,
		},
		{
			data:         &retryTest{force: true, numError: 10}, // Run, fail, force exit
			retryTimeMin: 1 * time.Nanosecond,
			retryTimeMax: 5000 * time.Nanosecond,
			res:          errors.New("runFunction Error 3"),
		},
	}
	f := func(i interface{}) (error, bool) {
		rt := i.(*retryTest)
		if rt.numError == 0 {
			return nil, rt.force
		}
		rt.numError--
		return errors.New("runFunction Error"), rt.force
	}
	for _, test := range tests {
		res := d.RunFunctionWithRetry(f, test.data, test.retryTimeMin, test.retryTimeMax)
		if (res != nil && test.res == nil) || (res == nil && test.res != nil) {
			t.Errorf("Expected %s, got %s", test.res, res)
		}
	}

}

func TestIsFileNew(t *testing.T) {
	tests := []struct {
		fs        *testStore
		directory string
		filename  string
		res       bool
	}{
		{
			fs: &testStore{map[string]*testFileObject{
				"search/unique":     {name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      {name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      {name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": {name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       true,
		},
		{
			fs: &testStore{map[string]*testFileObject{
				"search/unique":     {name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      {name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      {name: "search/stuff", data: nil, md5: []byte("123")},
				"otherDir/ignoreMe": {name: "otherDir/ignoreMe", data: nil, md5: []byte("765")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       false,
		},
		{
			fs: &testStore{map[string]*testFileObject{
				"search/unique":     {name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      {name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      {name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": {name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "otherDir/ignoreMe",
			res:       false,
		},
		{
			fs: &testStore{map[string]*testFileObject{
				"search/unique":     {name: "search/unique", data: nil, md5: nil},
				"search/thing":      {name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      {name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": {name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       true,
		},
		{
			fs: &testStore{map[string]*testFileObject{
				"otherDir/ignoreMe": {name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "otherDir/ignoreMe",
			res:       true,
		},
	}
	for _, test := range tests {
		res := d.IsFileNew(test.fs, test.filename, test.directory)
		if res != test.res {
			t.Errorf("Expected %t, got %t for %+v.", test.res, res, test)
		}
	}

}

func TestCheckIfHashIsUniqueInList(t *testing.T) {
	tests := []struct {
		md5      []byte
		iter     map[string][]byte
		filename string
		res      bool
	}{
		{
			md5: []byte("cow"),
			iter: map[string][]byte{
				"Dinkleberg": []byte("Dinkleberg"),
			},
			filename: "Unit testing1",
			res:      true,
		},
		{
			md5:      []byte("cow"),
			iter:     map[string][]byte{},
			filename: "Unit testing2",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: map[string][]byte{
				"Unit testing3": []byte("cow"),
			},
			filename: "Unit testing3",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: map[string][]byte{
				"Dinkleberg": []byte("cow"),
			},
			filename: "Unit testing4",
			res:      false,
		},
		{
			md5: []byte("cow"),
			iter: map[string][]byte{
				"Dinkleberg":    []byte("Dinkleberg"),
				"Unit testing5": []byte("cow"),
			},
			filename: "Unit testing5",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: map[string][]byte{
				"Dinkleberg": []byte("Dinkleberg"),
				"Unit te5":   []byte("cw"),
				"Dieberg":    []byte("Dinrg"),
				"Unit test":  []byte("ow"),
				"Dinkg":      []byte("Din"),
				"Ung5":       []byte("c"),
			},
			filename: "Unit testing6",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: map[string][]byte{
				"Dinkleberg": []byte("Dinkleberg"),
				"Unit te5":   []byte("cow"),
				"Dieberg":    []byte("Dinrg"),
				"Unit test":  []byte("ow"),
				"Dinkg":      []byte("Din"),
				"Ung5":       []byte("c"),
			},
			filename: "Unit testing7",
			res:      false,
		},
	}
	for _, test := range tests {
		testRes := d.CheckIfHashIsUniqueInList(test.md5, test.iter, test.filename)
		if testRes != test.res {
			t.Errorf("Expected %t got %t for %+v", test.res, testRes, test)
		}
	}

}
