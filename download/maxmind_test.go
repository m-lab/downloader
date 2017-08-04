package download_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	d "github.com/m-lab/downloader/download"
	"github.com/m-lab/downloader/file"
)

func TestDownloadMaxmindFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.String())
	}))
	tests := []struct {
		urls      []string
		timestamp string
		fsto      file.FileStore
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
		res := d.DownloadMaxmindFiles(test.urls, test.timestamp, test.fsto)
		if (res == nil && test.res != nil) || (res != nil && test.res == nil) {
			t.Errorf("Expected %t, got %t for %+v\n\n, file sto: %+v, fstoaddr: ", test.res, res, test, test.fsto, &test.fsto)
		}
	}

}