package download_test

import (
	"log"
	"net/http"
	"os"
	"testing"

	d "github.com/m-lab/downloader/download"
)

func TestAllMaxmindURLs(t *testing.T) {
	log.Println(os.Getenv("MAXMIND_LICENSE_KEY"))
	for _, info := range d.MaxmindDownloadInfo {
		resp, err := http.Head(info.url + os.Getenv("MAXMIND_LICENSE_KEY"))
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("Bad URL (%q), err: %v (%v)", url, err, resp.StatusCode)
		}
	}
}
