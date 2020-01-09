package download

import (
	"regexp"
	"time"

	"github.com/m-lab/downloader/file"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var maxmindURLToFilenameRegexp = regexp.MustCompile(`.*/()(.*)`)
var maxmindFilenameToDedupeRegexp = regexp.MustCompile(`(.*/).*/.*`)

var MaxmindDownloadInfo = []struct {
	url      string
	filename string
}{
	{
		url:      "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-ASN&suffix=tar.gz&license_key=",
		filename: "GeoLite2-ASN_20200107.tar.gz",
	},
}

// The list of URLs to download from Maxmind
var MaxmindURLs []string = []string{
	"https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-ASN&suffix=tar.gz&license_key=",
	//"http://geolite.maxmind.com/download/geoip/database/GeoLite2-City-CSV.zip",
	//"http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country-CSV.zip",
	//"http://geolite.maxmind.com/download/geoip/database/GeoLite2-ASN-CSV.zip",
	//"http://geolite.maxmind.com/download/geoip/database/GeoLite2-City.tar.gz",
	//"http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country.tar.gz",
	//"http://geolite.maxmind.com/download/geoip/database/GeoLite2-ASN.tar.gz",
}

// DownloadMaxmindFiles takes a slice of urls pointing to maxmind
// files, a timestamp that the user wants attached to the files, and
// the instance of the FileStore interface where the user wants the
// files stored. It then downloads the files, stores them, and returns
// and error on failure or nil on success. Gaurenteed to not introduce
// duplicates.
func DownloadMaxmindFiles(timestamp string, store file.FileStore, maxmindLicenseKey string) error {
	var lastErr error = nil
	for index, _ := range MaxmindDownloadInfo {
		dc := DownloadConfig{
			URL:           MaxmindDownloadInfo[index].url + maxmindLicenseKey,
			Store:         store,
			PathPrefix:    "Maxmind/" + timestamp,
			FilePrefix:    time.Now().UTC().Format("20060102T150405Z-"),
			FixedFilename: MaxmindDownloadInfo[index].filename,
			DedupeRegexp:  maxmindFilenameToDedupeRegexp}
		if err := RunFunctionWithRetry(Download, dc, WaitAfterFirstDownloadFailure,
			MaximumWaitBetweenDownloadAttempts); err != nil {
			lastErr = err
			metrics.FailedDownloadCount.With(prometheus.Labels{"download_type": "Maxmind"}).Inc()
		}
	}
	return lastErr

}
