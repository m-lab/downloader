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

	"cloud.google.com/go/storage"
)

//// testStore implements the store interface for testing
type testStore struct {
	files map[string]obj
}

func (fsto testStore) getFile(name string) fileObject {
	if file, ok := fsto.files[name]; ok {
		return file
	}
	return &obj{name: name, md5: nil, data: bytes.NewBuffer(nil), fsto: &fsto}
}

func (fsto testStore) getFiles(prefix string) []fileAttributes {
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

func Test_genSleepTime(t *testing.T) {
	rand.Seed(0)
	testVals := make([]float64, 5)
	testVals[0] = 20
	testVals[1] = 1.281275096938293
	testVals[2] = 20
	testVals[3] = 0.5108671561337503
	testVals[4] = 14.863133989807169

	for i := 0; i < 5; i++ {
		val := testVals[i]
		testRes := genSleepTime(8)
		if val != testRes {
			t.Errorf("Expected %s, got %s.", val, testRes)
		}
	}

}

/*url       string
fileStore store
prefix    string
backChars int*/

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
				fileStore: testStore{map[string]obj{}},
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
				fileStore: testStore{map[string]obj{}},
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
				fileStore: testStore{map[string]obj{}},
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
				fileStore: testStore{map[string]obj{
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
				fileStore: testStore{map[string]obj{}},
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
		fs        testStore
		directory string
		filename  string
		res       bool
	}{
		{
			fs: testStore{map[string]obj{
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
			fs: testStore{map[string]obj{
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
			fs: testStore{map[string]obj{
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
			fs: testStore{map[string]obj{
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
			fs: testStore{map[string]obj{
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

func Test_genRouteViewURLs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		lastDownloaded int
		res            []URLAndID
	}{
		{0, []URLAndID{
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170616-1200.pfx2as.gz", ID: 3363},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170617-1200.pfx2as.gz", ID: 3364},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170618-1000.pfx2as.gz", ID: 3365},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170619-1200.pfx2as.gz", ID: 3366},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170620-1200.pfx2as.gz", ID: 3367},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170621-1000.pfx2as.gz", ID: 3368},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170622-0400.pfx2as.gz", ID: 3369},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170623-1200.pfx2as.gz", ID: 3370},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170624-1200.pfx2as.gz", ID: 3371},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170625-1200.pfx2as.gz", ID: 3372},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170626-1200.pfx2as.gz", ID: 3373},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170627-1200.pfx2as.gz", ID: 3374},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170628-1200.pfx2as.gz", ID: 3375},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170629-2200.pfx2as.gz", ID: 3376},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170630-1000.pfx2as.gz", ID: 3377},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170701-1200.pfx2as.gz", ID: 3378},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170702-1200.pfx2as.gz", ID: 3379},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170703-1000.pfx2as.gz", ID: 3380},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170704-1200.pfx2as.gz", ID: 3381},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170705-1200.pfx2as.gz", ID: 3382},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170706-1200.pfx2as.gz", ID: 3383},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170707-2000.pfx2as.gz", ID: 3384},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170708-1400.pfx2as.gz", ID: 3385},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170709-1200.pfx2as.gz", ID: 3386},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170710-1200.pfx2as.gz", ID: 3387},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170711-1200.pfx2as.gz", ID: 3388},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170712-2000.pfx2as.gz", ID: 3389},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170713-1200.pfx2as.gz", ID: 3390},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170714-1400.pfx2as.gz", ID: 3391},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170715-1200.pfx2as.gz", ID: 3392},
		}},
		{3380, []URLAndID{
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170704-1200.pfx2as.gz", ID: 3381},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170705-1200.pfx2as.gz", ID: 3382},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170706-1200.pfx2as.gz", ID: 3383},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170707-2000.pfx2as.gz", ID: 3384},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170708-1400.pfx2as.gz", ID: 3385},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170709-1200.pfx2as.gz", ID: 3386},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170710-1200.pfx2as.gz", ID: 3387},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170711-1200.pfx2as.gz", ID: 3388},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170712-2000.pfx2as.gz", ID: 3389},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170713-1200.pfx2as.gz", ID: 3390},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170714-1400.pfx2as.gz", ID: 3391},
			URLAndID{URL: ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170715-1200.pfx2as.gz", ID: 3392},
		}},
		{4000, nil},
	}

	for _, test := range tests {
		res, err := genRouteViewURLs(ts.URL, test.lastDownloaded)
		if err != nil {
			t.Errorf("genRouteViewURLs returned %s on %+v, %d.", err, res, test.lastDownloaded)
		}
		if !reflect.DeepEqual(res, test.res) {
			t.Errorf("Expected \n%+v\n, got \n%+v", test.res, res)
		}
	}

}
