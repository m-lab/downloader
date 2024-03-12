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
		user, found := os.LookupEnv("MAXMIND_ACCOUNT_ID")
		if !found {
			t.Error("Could not load Maxmind account ID from ${MAXMIND_ACCOUNT_ID}")
			return
		}
		key, found := os.LookupEnv("MAXMIND_LICENSE_KEY")
		if !found {
			t.Error("Could not load Maxmind license key from ${MAXMIND_LICENSE_KEY}")
			return
		}

		url := maxmindDownloadInfo[index].url
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Errorf("http.NewRequest() failed for %q: %v ", url, err)
		}

		req.Close = true
		req.SetBasicAuth(user, key)

		client := http.Client{}
		resp, err := client.Do(req)

		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("Bad URL (%q), err: %v (%v)", url, err, resp.StatusCode)
		}
	}
}
