package download

import (
	"context"
	"regexp"
	"time"

	"github.com/m-lab/downloader/file"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var maxmindFilenameToDedupRegexp = regexp.MustCompile(`(.*/).*/.*`)

var maxmindDownloadInfo = []struct {
	url      string
	filename string
	current  string
}{
	{
		url:      "https://download.maxmind.com/geoip/databases/GeoLite2-ASN/download?suffix=tar.gz",
		filename: "GeoLite2-ASN.tar.gz",
	},
	{
		url:      "https://download.maxmind.com/geoip/databases/GeoLite2-ASN-CSV/download?suffix=zip",
		filename: "GeoLite2-ASN-CSV.zip",
	},
	{
		url:      "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz",
		filename: "GeoLite2-City.tar.gz",
		current:  "Maxmind/current/GeoLite2-City.tar.gz",
	},
	{
		url:      "https://download.maxmind.com/geoip/databases/GeoLite2-City-CSV/download?suffix=zip",
		filename: "GeoLite2-City-CSV.zip",
	},
	{
		url:      "https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz",
		filename: "GeoLite2-Country.tar.gz",
	},
	{
		url:      "https://download.maxmind.com/geoip/databases/GeoLite2-Country-CSV/download?suffix=zip",
		filename: "GeoLite2-Country-CSV.zip",
	},
}

// MaxmindFiles takes a slice of urls pointing to maxmind files, a timestamp
// that the user wants attached to the files, and the instance of the FileStore
// interface where the user wants the files stored. It then downloads the files,
// stores them, and returns and error on failure or nil on success. Guaranteed
// to not introduce duplicates.
func MaxmindFiles(ctx context.Context, timestamp string, store file.Store, maxmindLicenseKey string, maxmindAccountID string) error {
	var lastErr error
	for _, info := range maxmindDownloadInfo {
		dc := config{
			URL:           info.url,
			Store:         store,
			PathPrefix:    "Maxmind/" + timestamp,
			CurrentName:   info.current,
			FilePrefix:    time.Now().UTC().Format("20060102T150405Z-"),
			FixedFilename: info.filename,
			DedupRegexp:   maxmindFilenameToDedupRegexp,
			MaxDuration:   *downloadTimeout,
			BasicAuthUser: maxmindAccountID,
			BasicAuthPass: maxmindLicenseKey,
		}
		if err := runFunctionWithRetry(ctx, download, dc, *waitAfterFirstDownloadFailure, *maximumWaitBetweenDownloadAttempts); err != nil {
			lastErr = err
			metrics.FailedDownloadCount.With(prometheus.Labels{"download_type": "Maxmind"}).Inc()
		}
	}
	return lastErr

}
