package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
)

func Test_downloadMaxmindFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.String())
	}))
	tests := []struct {
		urls      []string
		timestamp string
		fsto      store
		res       error
	}{
		{
			urls: []string{
				ts.URL + "/filename",
			},
			timestamp: "2006/01/02/15:04:05-",
			fsto:      &testStore{map[string]obj{}},
			res:       nil,
		},
		{
			urls: []string{
				ts.URL + "/filename",
				ts.URL + "/deleteFail",
			},
			timestamp: "2006/01/02/15:04:05-",
			fsto:      &testStore{map[string]obj{}},
			res:       errors.New(""),
		},
	}
	for _, test := range tests {
		res := downloadMaxmindFiles(test.urls, test.timestamp, test.fsto)
		if (res == nil && test.res != nil) || (res != nil && test.res == nil) {
			t.Errorf("Expected %t, got %t for %+v\n\n, file sto: %+v, fstoaddr: ", test.res, res, test, test.fsto, &test.fsto)
		}
	}

}

func Test_download(t *testing.T) {
	tests := []struct {
		dc      downloadConfig
		postfix string
		resBool bool
		resErr  error
	}{
		{
			dc: downloadConfig{
				url:       "Fill me",
				fileStore: &testStore{map[string]obj{}},
				prefix:    "pre/",
				backChars: 0,
			},
			postfix: "portGarbage",
			resBool: false,
			resErr:  errors.New("invalid URL port"),
		},
		{
			dc: downloadConfig{
				url:       "Fill me",
				fileStore: &testStore{map[string]obj{}},
				prefix:    "pre/",
				backChars: 0,
			},
			postfix: "/file.error",
			resBool: false,
			resErr:  errors.New("non-200 error"),
		},
		{
			dc: downloadConfig{
				url:       "Fill me",
				fileStore: &testStore{map[string]obj{}},
				prefix:    "pre/",
				backChars: 0,
			},
			postfix: "/file.copyFail",
			resBool: false,
			resErr:  errors.New("File copy error"),
		},
		{
			dc: downloadConfig{
				url: "Fill me",
				fileStore: &testStore{map[string]obj{
					"pre/file.del/dup": obj{name: "pre/file.del/dup", data: bytes.NewBuffer(nil), md5: []byte("NEW FILE")},
				}},
				prefix:    "pre/",
				backChars: 0,
			},
			postfix: "/file.deleteFail",
			resBool: true,
			resErr:  errors.New("Couldn't Delete File"),
		},
		{
			dc: downloadConfig{
				url:       "Fill me",
				fileStore: &testStore{map[string]obj{}},
				prefix:    "pre/",
				backChars: 0,
			},
			postfix: "/file.success",
			resBool: false,
			resErr:  nil,
		},
	}
	if err, force := download(nil); err == nil || force != true {
		t.Errorf("FUNCTION DID NOT REJECT INVALID INTERFACE!!!")
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
		test.dc.url = ts.URL + test.postfix
		err, resBool := download(test.dc)
		if test.resBool != resBool || (err != nil && test.resErr == nil) || (err == nil && test.resErr != nil) {
			t.Errorf("Expected %s, %t got %s, %t", test.resErr, test.resBool, err, resBool)
		}

	}

}

type retryTest struct {
	force    bool
	numError int
}

func Test_runFunctionWithRetry(t *testing.T) {
	tests := []struct {
		data         *retryTest
		retryTimeMin time.Duration
		retryTimeMax time.Duration
		res          error
	}{
		{
			data:         &retryTest{force: false, numError: 0},
			retryTimeMin: 0,
			retryTimeMax: 0,
			res:          nil,
		},
		{
			data:         &retryTest{force: false, numError: 1},
			retryTimeMin: 1,
			retryTimeMax: 0,
			res:          errors.New("runFunction Error 1"),
		},
		{
			data:         &retryTest{force: false, numError: 100},
			retryTimeMin: 1 * time.Nanosecond,
			retryTimeMax: 50 * time.Nanosecond,
			res:          errors.New("runFunction Error 2"),
		},
		{
			data:         &retryTest{force: false, numError: 10},
			retryTimeMin: 1 * time.Nanosecond,
			retryTimeMax: 5000 * time.Nanosecond,
			res:          nil,
		},
		{
			data:         &retryTest{force: true, numError: 10},
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
		res := runFunctionWithRetry(f, test.data, test.retryTimeMin, test.retryTimeMax)
		if (res != nil && test.res == nil) || (res == nil && test.res != nil) {
			t.Errorf("Expected %s, got %s", test.res, res)
		}
	}

}

func Test_determineIfFileIsNew(t *testing.T) {
	tests := []struct {
		fs        *testStore
		directory string
		filename  string
		res       bool
	}{
		{
			fs: &testStore{map[string]obj{
				"search/unique":     obj{name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      obj{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      obj{name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": obj{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       true,
		},
		{
			fs: &testStore{map[string]obj{
				"search/unique":     obj{name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      obj{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      obj{name: "search/stuff", data: nil, md5: []byte("123")},
				"otherDir/ignoreMe": obj{name: "otherDir/ignoreMe", data: nil, md5: []byte("765")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       false,
		},
		{
			fs: &testStore{map[string]obj{
				"search/unique":     obj{name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      obj{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      obj{name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": obj{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "otherDir/ignoreMe",
			res:       false,
		},
		{
			fs: &testStore{map[string]obj{
				"search/unique":     obj{name: "search/unique", data: nil, md5: nil},
				"search/thing":      obj{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      obj{name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": obj{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       true,
		},
		{
			fs: &testStore{map[string]obj{
				"otherDir/ignoreMe": obj{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "otherDir/ignoreMe",
			res:       true,
		},
	}
	for _, test := range tests {
		res := determineIfFileIsNew(test.fs, test.filename, test.directory)
		if res != test.res {
			t.Errorf("Expected %t, got %t for %+v.", test.res, res, test)
		}
	}

}

func Test_getHashOfFile(t *testing.T) {
	tests := []obj{
		{
			md5:  []byte("Moo"),
			name: "foimsd",
			data: bytes.NewBuffer(nil),
		},
		{
			md5:  nil,
			name: "GonnaError",
			data: bytes.NewBuffer(nil),
		},
	}
	for _, test := range tests {
		testRes, err := getHashOfFile(test)
		if (test.md5 != nil && (!reflect.DeepEqual(testRes, test.md5) || err != nil)) || (test.md5 == nil && (testRes != nil || err == nil)) {
			t.Errorf("Expected %s got %s, %v for %+v", test.md5, testRes, err, test)
		}
	}

}

func Test_checkIfHashIsUniqueInList(t *testing.T) {
	tests := []struct {
		md5      []byte
		iter     []fileAttributes
		filename string
		res      bool
	}{
		{
			md5: []byte("cow"),
			iter: []fileAttributes{
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dinkleberg", MD5: []byte("Dinkleberg")}},
			},
			filename: "Unit testing1",
			res:      true,
		},
		{
			md5:      []byte("cow"),
			iter:     []fileAttributes{},
			filename: "Unit testing2",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: []fileAttributes{
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Unit testing3", MD5: []byte("cow")}},
			},
			filename: "Unit testing3",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: []fileAttributes{
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dinkleberg", MD5: []byte("cow")}},
			},
			filename: "Unit testing4",
			res:      false,
		},
		{
			md5: []byte("cow"),
			iter: []fileAttributes{
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dinkleberg", MD5: []byte("Dinkleberg")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Unit testing5", MD5: []byte("cow")}},
			},
			filename: "Unit testing5",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: []fileAttributes{
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dinkleberg", MD5: []byte("Dinkleberg")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Unit te5", MD5: []byte("cw")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dieberg", MD5: []byte("Dinrg")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Unit test", MD5: []byte("ow")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dinkg", MD5: []byte("Din")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Ung5", MD5: []byte("c")}},
			},
			filename: "Unit testing6",
			res:      true,
		},
		{
			md5: []byte("cow"),
			iter: []fileAttributes{
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dinkleberg", MD5: []byte("Dinkleberg")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Unit te5", MD5: []byte("cow")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dieberg", MD5: []byte("Dinrg")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Unit test", MD5: []byte("ow")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Dinkg", MD5: []byte("Din")}},
				&fileAttributesGCS{&storage.ObjectAttrs{Name: "Ung5", MD5: []byte("c")}},
			},
			filename: "Unit testing7",
			res:      false,
		},
	}
	for _, test := range tests {
		testRes := checkIfHashIsUniqueInList(test.md5, test.iter, test.filename)
		if testRes != test.res {
			t.Errorf("Expected %t got %t for %+v", test.res, testRes, test)
		}
	}

}
