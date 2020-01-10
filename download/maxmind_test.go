package download

import (
	"net/http"
	"os"
	"testing"
)

func TestAllMaxmindURLs(t *testing.T) {
	for index, _ := range MaxmindDownloadInfo {
		resp, err := http.Head(MaxmindDownloadInfo[index].url + os.Getenv("MAXMIND_LICENSE_KEY"))
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("Bad URL (%q), err: %v (%v)", url, err, resp.StatusCode)
		}
	}
}
