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

// The list of URLs to download from Maxmind
var MaxmindURLs []string = []string{
	"http://geolite.maxmind.com/download/geoip/database/GeoLiteCity.dat.gz",
	"http://geolite.maxmind.com/download/geoip/database/GeoLiteCityv6-beta/GeoLiteCityv6.dat.gz",
	"http://download.maxmind.com/download/geoip/database/asnum/GeoIPASNumv6.dat.gz",
	"http://download.maxmind.com/download/geoip/database/asnum/GeoIPASNum2v6.zip",
	"http://geolite.maxmind.com/download/geoip/database/GeoLiteCityv6-beta/GeoLiteCityv6.csv.gz",
	"http://geolite.maxmind.com/download/geoip/database/GeoIPv6.csv.gz",
	"http://geolite.maxmind.com/download/geoip/database/GeoLite2-City-CSV.zip",
	"http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country-CSV.zip",
	"http://geolite.maxmind.com/download/geoip/database/GeoLite2-ASN-CSV.zip",
	"http://geolite.maxmind.com/download/geoip/database/GeoLite2-City.tar.gz",
	"http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country.tar.gz",
	"http://geolite.maxmind.com/download/geoip/database/GeoLite2-ASN.tar.gz",
}

// DownloadMaxmindFiles takes a slice of urls pointing to maxmind
// files, a timestamp that the user wants attached to the files, and
// the instance of the FileStore interface where the user wants the
// files stored. It then downloads the files, stores them, and returns
// and error on failure or nil on success. Gaurenteed to not introduce
// duplicates.
func DownloadMaxmindFiles(urls []string, timestamp string, store file.FileStore) error {
	var lastErr error = nil
	for _, url := range urls {
		dc := DownloadConfig{URL: url, Store: store, PathPrefix: "Maxmind/" + timestamp,
			FilePrefix: time.Now().UTC().Format("20060102T150405Z-"), URLRegexp: maxmindURLToFilenameRegexp,
			DedupeRegexp: maxmindFilenameToDedupeRegexp}
		if err := RunFunctionWithRetry(Download, dc, WaitAfterFirstDownloadFailure,
			MaximumWaitBetweenDownloadAttempts); err != nil {
			lastErr = err
			metrics.FailedDownloadCount.With(prometheus.Labels{"download_type": "Maxmind"}).Inc()
		}
	}
	return lastErr

}
