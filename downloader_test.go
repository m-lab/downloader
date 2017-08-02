package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

//// implementation of API purely for testing purposes

//// testStore implements the store interface for testing
type testStore struct {
	files map[string]testFileObject
}

func (fsto *testStore) getFile(name string) fileObject {
	if file, ok := fsto.files[name]; ok {
		return file
	}
	return testFileObject{name: name, md5: nil, data: bytes.NewBuffer(nil), fsto: fsto}
}

func (fsto *testStore) namesToMD5(prefix string) map[string][]byte {
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
	name string
	md5  []byte
	data *bytes.Buffer
	fsto *testStore
}

func (file testFileObject) getWriter() io.WriteCloser {
	return file
}

func (file testFileObject) Write(p []byte) (n int, err error) {
	if strings.HasSuffix(file.name, "copyFail") {
		return 0, errors.New("Example Copy Error")
	}
	return file.data.Write(p)
}

func (file testFileObject) Close() error {
	file.md5 = []byte("NEW FILE")
	file.fsto.files[file.name] = file
	return nil
}

func (file testFileObject) deleteFile() error {
	if strings.HasSuffix(file.name, "deleteFail") {
		return errors.New("Couldn't delete file!")
	}
	return nil
}

//// End of stubs for testing

func Test_downloadMaxmindFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.String())
	}))
	tests := []struct {
		urls      []string
		timestamp string
		fsto      fileStore
		res       error
	}{
		{
			urls: []string{
				ts.URL + "/filename",
			},
			timestamp: "2006/01/02/15:04:05-",
			fsto:      &testStore{map[string]testFileObject{}},
			res:       nil,
		},
		{
			urls: []string{
				ts.URL + "/filename",
				ts.URL + "/deleteFail",
			},
			timestamp: "2006/01/02/15:04:05-",
			fsto:      &testStore{map[string]testFileObject{}},
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

func Test_downloadRouteviewsFiles(t *testing.T) {
	tests := []struct {
		logFile string
		dir     string
		lastD   int
		lastS   int
		fsto    fileStore
		res     error
	}{
		{
			logFile: "/logFile1",
			dir:     "test1/",
			lastD:   0,
			lastS:   3365,
			fsto:    &testStore{map[string]testFileObject{}},
			res:     nil,
		},
		{
			logFile: "/logFile2",
			dir:     "test2/",
			lastD:   0,
			lastS:   3364,
			fsto:    &testStore{map[string]testFileObject{}},
			res:     errors.New(""),
		},
		{
			logFile: "portGarbage",
			dir:     "test3/",
			lastD:   0,
			lastS:   0,
			fsto:    &testStore{map[string]testFileObject{}},
			res:     errors.New(""),
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "logFile1") {
			fmt.Fprint(w, `# Format: 1
# Fields: seqnum timestamp path
# Generated: 2017-07-16 09:26:29 -0700
# --------------------------------------------------------------------------
# Check this log regularly (once or twice a day) to keep up with the
# generation of daily files.  The easiest way to find the newest files is
# to compare the last seqnum you downloaded to the seqnum of all entries.
#
# The timestamp column gives the time that a daily pfx2as file was
# generated.  Please note that the timestamp will _not_ necessarily match
# the date in the filename, since file generation intentionally lags behind
# a bit.
# --------------------------------------------------------------------------
3363	1497717708	2017/06/routeviews-rv2-20170616-1200.pfx2as.gz
3364	1497803191	2017/06/routeviews-rv2-20170617-1200.pfx2as.gz
3365	1497889838	2018/06/routeviews-rv2-20170617-1000.pfx2as.gz`)
			return
		}
		if strings.HasSuffix(path, "logFile2") {
			fmt.Fprint(w, `# Format: 1
# Fields: seqnum timestamp path
# Generated: 2017-07-16 09:26:29 -0700
# --------------------------------------------------------------------------
# Check this log regularly (once or twice a day) to keep up with the
# generation of daily files.  The easiest way to find the newest files is
# to compare the last seqnum you downloaded to the seqnum of all entries.
#
# The timestamp column gives the time that a daily pfx2as file was
# generated.  Please note that the timestamp will _not_ necessarily match
# the date in the filename, since file generation intentionally lags behind
# a bit.
# --------------------------------------------------------------------------
3363	1497717708	2017/06/routeviews-rv2-20170616-1200.pfx2as.gz
3364	1497803191	2017/06/routeviews-rv2-20170617-1200.pfx2as.gz
3365	1497889838	2017/06/deleteFail`)
			return
		}
		fmt.Fprint(w, r.URL.String())
	}))
	for _, test := range tests {
		res := downloadRouteviewsFiles(ts.URL+test.logFile, test.dir, &test.lastD, test.fsto)
		if (res == nil && test.res != nil) || (res != nil && test.res == nil) {
			t.Errorf("Expected %t, got %t!!!", test.res, res)
		}
		if test.lastD != test.lastS {
			t.Errorf("Expected %d, got %d", test.lastS, test.lastD)
		}
	}
}

func Test_genUniformSleepTime(t *testing.T) {
	rand.Seed(0)
	testVals := make([]float64, 5)
	testVals[0] = 9.780784597176465
	testVals[1] = 6.979860341175119
	testVals[2] = 8.623825060781620
	testVals[3] = 6.217375358398802
	testVals[4] = 7.470348826529834
	for i := 0; i < 5; i++ {
		testRes := genUniformSleepTime(8, 4)
		if testVals[i] != testRes {
			t.Errorf("Expected %s, got %s.", testVals[i], testRes)
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
				store:     &testStore{map[string]testFileObject{}},
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
				store:     &testStore{map[string]testFileObject{}},
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
				store:     &testStore{map[string]testFileObject{}},
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
				store: &testStore{map[string]testFileObject{
					"pre/file.del/dup": testFileObject{name: "pre/file.del/dup", data: bytes.NewBuffer(nil), md5: []byte("NEW FILE")},
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
				store:     &testStore{map[string]testFileObject{}},
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
			fs: &testStore{map[string]testFileObject{
				"search/unique":     testFileObject{name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      testFileObject{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      testFileObject{name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": testFileObject{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       true,
		},
		{
			fs: &testStore{map[string]testFileObject{
				"search/unique":     testFileObject{name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      testFileObject{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      testFileObject{name: "search/stuff", data: nil, md5: []byte("123")},
				"otherDir/ignoreMe": testFileObject{name: "otherDir/ignoreMe", data: nil, md5: []byte("765")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       false,
		},
		{
			fs: &testStore{map[string]testFileObject{
				"search/unique":     testFileObject{name: "search/unique", data: nil, md5: []byte("123")},
				"search/thing":      testFileObject{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      testFileObject{name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": testFileObject{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "otherDir/ignoreMe",
			res:       false,
		},
		{
			fs: &testStore{map[string]testFileObject{
				"search/unique":     testFileObject{name: "search/unique", data: nil, md5: nil},
				"search/thing":      testFileObject{name: "search/thing", data: nil, md5: []byte("000")},
				"search/stuff":      testFileObject{name: "search/stuff", data: nil, md5: []byte("765")},
				"otherDir/ignoreMe": testFileObject{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
			}},
			directory: "search/",
			filename:  "search/unique",
			res:       true,
		},
		{
			fs: &testStore{map[string]testFileObject{
				"otherDir/ignoreMe": testFileObject{name: "otherDir/ignoreMe", data: nil, md5: []byte("123")},
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

func Test_checkIfHashIsUniqueInList(t *testing.T) {
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
		testRes := checkIfHashIsUniqueInList(test.md5, test.iter, test.filename)
		if testRes != test.res {
			t.Errorf("Expected %t got %t for %+v", test.res, testRes, test)
		}
	}

}

func Test_genRouteViewURLs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "error") {
			http.Error(w, "Test Error", 404)
			return
		}
		fmt.Fprint(w, `# Format: 1
# Fields: seqnum timestamp path
# Generated: 2017-07-16 09:26:29 -0700
# --------------------------------------------------------------------------
# Check this log regularly (once or twice a day) to keep up with the
# generation of daily files.  The easiest way to find the newest files is
# to compare the last seqnum you downloaded to the seqnum of all entries.
#
# The timestamp column gives the time that a daily pfx2as file was
# generated.  Please note that the timestamp will _not_ necessarily match
# the date in the filename, since file generation intentionally lags behind
# a bit.
# --------------------------------------------------------------------------
3363	1497717708	2017/06/routeviews-rv2-20170616-1200.pfx2as.gz
3364	1497803191	2017/06/routeviews-rv2-20170617-1200.pfx2as.gz
3365	1497889838	2017/06/routeviews-rv2-20170618-1000.pfx2as.gz
3366	1497976220	2017/06/routeviews-rv2-20170619-1200.pfx2as.gz
3367	1498062848	2017/06/routeviews-rv2-20170620-1200.pfx2as.gz
3368	1498149227	2017/06/routeviews-rv2-20170621-1000.pfx2as.gz
3369	1498235751	2017/06/routeviews-rv2-20170622-0400.pfx2as.gz
3370	1498321618	2017/06/routeviews-rv2-20170623-1200.pfx2as.gz
3371	1498408147	2017/06/routeviews-rv2-20170624-1200.pfx2as.gz
3372	1498494550	2017/06/routeviews-rv2-20170625-1200.pfx2as.gz
3373	1498580169	2017/06/routeviews-rv2-20170626-1200.pfx2as.gz
3374	1498667699	2017/06/routeviews-rv2-20170627-1200.pfx2as.gz
3375	1498753979	2017/06/routeviews-rv2-20170628-1200.pfx2as.gz
3376	1498840316	2017/06/routeviews-rv2-20170629-2200.pfx2as.gz
3377	1498926359	2017/06/routeviews-rv2-20170630-1000.pfx2as.gz
3378	1499013879	2017/07/routeviews-rv2-20170701-1200.pfx2as.gz
3379	1499100250	2017/07/routeviews-rv2-20170702-1200.pfx2as.gz
3380	1499187237	2017/07/routeviews-rv2-20170703-1000.pfx2as.gz
3381	1499273320	2017/07/routeviews-rv2-20170704-1200.pfx2as.gz
3382	1499359329	2017/07/routeviews-rv2-20170705-1200.pfx2as.gz
3383	1499445259	2017/07/routeviews-rv2-20170706-1200.pfx2as.gz
3384	1499531673	2017/07/routeviews-rv2-20170707-2000.pfx2as.gz
3385	1499617983	2017/07/routeviews-rv2-20170708-1400.pfx2as.gz
3386	1499704095	2017/07/routeviews-rv2-20170709-1200.pfx2as.gz
3387	1499790914	2017/07/routeviews-rv2-20170710-1200.pfx2as.gz
3388	1499877213	2017/07/routeviews-rv2-20170711-1200.pfx2as.gz
3389	1499963255	2017/07/routeviews-rv2-20170712-2000.pfx2as.gz
3390	1500049445	2017/07/routeviews-rv2-20170713-1200.pfx2as.gz
3391	1500135872	2017/07/routeviews-rv2-20170714-1400.pfx2as.gz
3392	1500222389	2017/07/routeviews-rv2-20170715-1200.pfx2as.gz`)
	}))
	defer ts.Close()

	tests := []struct {
		suffix         string
		willErr        bool
		lastDownloaded int
		res            []urlAndSeqNum
	}{
		{"", false, 0, []urlAndSeqNum{
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170616-1200.pfx2as.gz", 3363},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170617-1200.pfx2as.gz", 3364},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170618-1000.pfx2as.gz", 3365},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170619-1200.pfx2as.gz", 3366},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170620-1200.pfx2as.gz", 3367},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170621-1000.pfx2as.gz", 3368},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170622-0400.pfx2as.gz", 3369},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170623-1200.pfx2as.gz", 3370},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170624-1200.pfx2as.gz", 3371},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170625-1200.pfx2as.gz", 3372},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170626-1200.pfx2as.gz", 3373},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170627-1200.pfx2as.gz", 3374},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170628-1200.pfx2as.gz", 3375},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170629-2200.pfx2as.gz", 3376},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170630-1000.pfx2as.gz", 3377},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170701-1200.pfx2as.gz", 3378},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170702-1200.pfx2as.gz", 3379},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170703-1000.pfx2as.gz", 3380},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170704-1200.pfx2as.gz", 3381},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170705-1200.pfx2as.gz", 3382},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170706-1200.pfx2as.gz", 3383},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170707-2000.pfx2as.gz", 3384},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170708-1400.pfx2as.gz", 3385},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170709-1200.pfx2as.gz", 3386},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170710-1200.pfx2as.gz", 3387},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170711-1200.pfx2as.gz", 3388},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170712-2000.pfx2as.gz", 3389},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170713-1200.pfx2as.gz", 3390},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170714-1400.pfx2as.gz", 3391},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170715-1200.pfx2as.gz", 3392},
		}},
		{"", false, 3380, []urlAndSeqNum{
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170704-1200.pfx2as.gz", 3381},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170705-1200.pfx2as.gz", 3382},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170706-1200.pfx2as.gz", 3383},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170707-2000.pfx2as.gz", 3384},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170708-1400.pfx2as.gz", 3385},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170709-1200.pfx2as.gz", 3386},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170710-1200.pfx2as.gz", 3387},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170711-1200.pfx2as.gz", 3388},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170712-2000.pfx2as.gz", 3389},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170713-1200.pfx2as.gz", 3390},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170714-1400.pfx2as.gz", 3391},
			urlAndSeqNum{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170715-1200.pfx2as.gz", 3392},
		}},
		{"", false, 4000, nil},
		{"/error", true, 0, nil},
		{"portGarbage", true, 0, nil},
	}

	for _, test := range tests {
		res, err := genRouteViewURLs(ts.URL+test.suffix, test.lastDownloaded)
		if !test.willErr {
			if err != nil {
				t.Errorf("genRouteViewURLs returned %s on %+v, %d.", err, res, test.lastDownloaded)
			}
			if !reflect.DeepEqual(res, test.res) {
				t.Errorf("Expected \n%+v\n, got \n%+v", test.res, res)
			}
		} else {
			if err == nil {
				t.Errorf("Expected error, got nil on %+v", test)
			}

		}
	}

}
