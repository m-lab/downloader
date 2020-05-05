package download

import (
	"net/http"
	"os"
	"testing"
)

// We bundle the config with the code in downloader. As long as we are doing
// that, then we should verify that the configured URLs actually work.
//
// TODO: Distribute the configuration and code separately.
func TestAllMaxmindURLs(t *testing.T) {
	for index := range maxmindDownloadInfo {
		key, found := os.LookupEnv("MAXMIND_LICENSE_KEY")
		if !found {
			t.Error("Could not load Maxmind license key from ${MAXMIND_LICENSE_KEY}")
			return
		}
		u := maxmindDownloadInfo[index].url + key
		resp, err := http.Head(u)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("Bad URL (%q), err: %v (%v)", u, err, resp.StatusCode)
		}
	}
}
