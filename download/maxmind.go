package download

import (
	"regexp"
	"time"

	"github.com/m-lab/downloader/file"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var maxmindFilenameToDedupRegexp = regexp.MustCompile(`(.*/).*/.*`)

var MaxmindDownloadInfo = []struct {
	url      string
	filename string
}{
	{
		url:      "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-ASN&suffix=tar.gz&license_key=",
		filename: "GeoLite2-ASN.tar.gz",
	},
	{
		url:      "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-ASN-CSV&suffix=zip&license_key=",
		filename: "GeoLite2-ASN-CSV.zip",
	},
	{
		url:      "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&suffix=tar.gz&license_key=",
		filename: "GeoLite2-City.tar.gz",
	},
	{
		url:      "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City-CSV&suffix=zip&license_key=",
		filename: "GeoLite2-City-CSV.zip",
	},
	{
		url:      "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&suffix=tar.gz&license_key=",
		filename: "GeoLite2-Country.tar.gz",
	},
	{
		url:      "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country-CSV&suffix=zip&license_key=",
		filename: "GeoLite2-Country-CSV.zip",
	},
	
}

// DownloadMaxmindFiles takes a slice of urls pointing to maxmind
// files, a timestamp that the user wants attached to the files, and
// the instance of the FileStore interface where the user wants the
// files stored. It then downloads the files, stores them, and returns
// and error on failure or nil on success. Gaurenteed to not introduce
// duplicates.
func DownloadMaxmindFiles(timestamp string, store file.FileStore, maxmindLicenseKey string) error {
	var lastErr error = nil
	for index := range MaxmindDownloadInfo {
		dc := DownloadConfig{
			URL:           MaxmindDownloadInfo[index].url + maxmindLicenseKey,
			Store:         store,
			PathPrefix:    "Maxmind/" + timestamp,
			FilePrefix:    time.Now().UTC().Format("20060102T150405Z-"),
			FixedFilename: MaxmindDownloadInfo[index].filename,
			DedupRegexp:  maxmindFilenameToDedupRegexp}
		if err := RunFunctionWithRetry(Download, dc, WaitAfterFirstDownloadFailure,
			MaximumWaitBetweenDownloadAttempts); err != nil {
			lastErr = err
			metrics.FailedDownloadCount.With(prometheus.Labels{"download_type": "Maxmind"}).Inc()
		}
	}
	return lastErr

}
