package download_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	d "github.com/m-lab/downloader/download"
	"github.com/m-lab/downloader/file"
)

func TestDownloadCaidaRouteviewsFiles(t *testing.T) {
	tests := []struct {
		logFile string
		dir     string
		lastD   int
		lastS   int
		fsto    file.FileStore
		res     error
	}{
		{
			logFile: "/logFile1",
			dir:     "test1/",
			lastD:   0,
			lastS:   3365,
			fsto:    &testStore{map[string]*testFileObject{}},
			res:     nil,
		},
		{
			logFile: "/logFile2",
			dir:     "test2/",
			lastD:   0,
			lastS:   3364,
			fsto:    &testStore{map[string]*testFileObject{}},
			res:     errors.New("2"),
		},
		{
			logFile: "portGarbage",
			dir:     "test3/",
			lastD:   0,
			lastS:   0,
			fsto:    &testStore{map[string]*testFileObject{}},
			res:     errors.New("3"),
		},
	}
	d.MaximumWaitBetweenDownloadAttempts = 0
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
3365	1497889838	2017/06/copyFail`)
			return
		}
		fmt.Fprint(w, r.URL.String())
	}))
	for _, test := range tests {
		res := d.DownloadCaidaRouteviewsFiles(ts.URL+test.logFile, test.dir, &test.lastD, "", test.fsto)
		if (res == nil && test.res != nil) || (res != nil && test.res == nil) {
			t.Errorf("Expected %t, got %t!!!", test.res, res)
		}
		if test.lastD != test.lastS {
			t.Errorf("Expected %d, got %d", test.lastS, test.lastD)
		}
	}
}

func TestGenRouteViewURLs(t *testing.T) {
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
		res            []d.URLAndSeqNum
	}{
		{"", false, 0, []d.URLAndSeqNum{
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170616-1200.pfx2as.gz", 3363},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170617-1200.pfx2as.gz", 3364},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170618-1000.pfx2as.gz", 3365},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170619-1200.pfx2as.gz", 3366},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170620-1200.pfx2as.gz", 3367},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170621-1000.pfx2as.gz", 3368},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170622-0400.pfx2as.gz", 3369},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170623-1200.pfx2as.gz", 3370},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170624-1200.pfx2as.gz", 3371},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170625-1200.pfx2as.gz", 3372},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170626-1200.pfx2as.gz", 3373},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170627-1200.pfx2as.gz", 3374},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170628-1200.pfx2as.gz", 3375},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170629-2200.pfx2as.gz", 3376},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/06/routeviews-rv2-20170630-1000.pfx2as.gz", 3377},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170701-1200.pfx2as.gz", 3378},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170702-1200.pfx2as.gz", 3379},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170703-1000.pfx2as.gz", 3380},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170704-1200.pfx2as.gz", 3381},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170705-1200.pfx2as.gz", 3382},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170706-1200.pfx2as.gz", 3383},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170707-2000.pfx2as.gz", 3384},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170708-1400.pfx2as.gz", 3385},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170709-1200.pfx2as.gz", 3386},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170710-1200.pfx2as.gz", 3387},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170711-1200.pfx2as.gz", 3388},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170712-2000.pfx2as.gz", 3389},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170713-1200.pfx2as.gz", 3390},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170714-1400.pfx2as.gz", 3391},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170715-1200.pfx2as.gz", 3392},
		}},
		{"", false, 3380, []d.URLAndSeqNum{
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170704-1200.pfx2as.gz", 3381},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170705-1200.pfx2as.gz", 3382},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170706-1200.pfx2as.gz", 3383},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170707-2000.pfx2as.gz", 3384},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170708-1400.pfx2as.gz", 3385},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170709-1200.pfx2as.gz", 3386},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170710-1200.pfx2as.gz", 3387},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170711-1200.pfx2as.gz", 3388},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170712-2000.pfx2as.gz", 3389},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170713-1200.pfx2as.gz", 3390},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170714-1400.pfx2as.gz", 3391},
			{ts.URL[:strings.LastIndex(ts.URL, "/")+1] + "2017/07/routeviews-rv2-20170715-1200.pfx2as.gz", 3392},
		}},
		{"", false, 4000, nil},
		{"/error", true, 0, nil},
		{"portGarbage", true, 0, nil},
	}

	for _, test := range tests {
		res, err := d.GenRouteViewURLs(ts.URL+test.suffix, test.lastDownloaded)
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
