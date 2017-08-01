package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const waitAfterFirstDownloadFailure = time.Minute * time.Duration(1)      // The time (in minutes) to wait before the first retry of a failed download
const maximumWaitBetweenDownloadAttempts = time.Minute * time.Duration(8) // The maximum time (in minutes) to wait in between download attempts

// downloadConfig is a struct for bundling parameters to be passed through runFunctionWithRetry to the download function.
type downloadConfig struct {
	url       string // The URL of the file to download
	fileStore store  // The store in which to place the file
	prefix    string // The prefix to append to the file name after it's downloaded
	backChars int    // The number of extra characters from the URL to include in the file name
}

// The list of URLs to download from Maxmind
var maxmindURLs []string = []string{
	"http://geolite.maxmind.com/download/geoip/database/GeoLiteCity.dat.gz",
	"http://geolite.maxmind.com/download/geoip/database/GeoLiteCityv6-beta/GeoLiteCityv6.dat.gz",
	"http://download.maxmind.com/download/geoip/database/asnum/GeoIPASNum.dat.gz",
	"http://download.maxmind.com/download/geoip/database/asnum/GeoIPASNumv6.dat.gz",
	"http://download.maxmind.com/download/geoip/database/asnum/GeoIPASNum2v6.zip",
	"http://download.maxmind.com/download/geoip/database/asnum/GeoIPASNum2.zip",
	"http://geolite.maxmind.com/download/geoip/database/GeoLiteCity_CSV/GeoLiteCity-latest.zip",
	"http://geolite.maxmind.com/download/geoip/database/GeoLiteCityv6-beta/GeoLiteCityv6.csv.gz",
	"http://geolite.maxmind.com/download/geoip/database/GeoIPCountryCSV.zip",
	"http://geolite.maxmind.com/download/geoip/database/GeoIPv6.csv.gz",
}

//These vars are the prometheus metrics
var (
	// Always set to the last time we had a successful download of ALL files
	// Provides metrics:
	//    downloader_last_success_time_seconds
	// Example usage:
	//    LastSuccessTime.Inc()
	LastSuccessTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_last_success_time_seconds",
		Help: "The time that ALL the downloads last completed successfully.",
	})

	// Measures the number of downloads that have failed completely
	// Provides metrics:
	//    downloader_download_failed
	// Example usage:
	//    FailedDownloadCount.Inc()
	FailedDownloadCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_download_failed",
		Help: "Increments every time a download maxes out our number of retries.",
	}, []string{"DownloadType"})

	// Measures the number of downloader errors
	// Provides metrics:
	//    downloader_error_count
	// Example usage:
	//    DownloaderErrorCount.Inc()
	DownloaderErrorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_error_count",
		Help: "The current number of unresolved errors encountered while attemting to download the latest maxmind and routeviews data.",
	}, []string{"source"})

	// Measures the number of errors involved with getting the list of routeview files
	// Provides metrics:
	//    downloader_downloader_routeviews_url_error_count
	// Example usage:
	//    RouteviewsURLErrorCount.Inc()
	RouteviewsURLErrorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_downloader_routeviews_url_error_count",
		Help: "The number of erros that occured with retrieving the Routeviews URL list.",
	}, []string{"source"})
)

func main() {}

// downloadMaxmindFiles takes a slice of urls pointing to maxmind files, a timestamp that the user wants attached to the files, and the instance of the store interface where the user wants the files stored. It then downloads the files, stores them, and returns and error on failure or nil on success. Gaurenteed to not introduce duplicates.
func downloadMaxmindFiles(urls []string, timestamp string, fileStore store) error {
	var lastErr error = nil
	for _, url := range urls {
		dc := downloadConfig{url: url, fileStore: fileStore, prefix: "Maxmind/" + timestamp, backChars: 0}
		if err := runFunctionWithRetry(download, dc, waitAfterFirstDownloadFailure, maximumWaitBetweenDownloadAttempts); err != nil {
			lastErr = err
			FailedDownloadCount.With(prometheus.Labels{"DownloadType": "Maxmind"}).Inc()
		}
	}
	return lastErr

}

// download takes a fully populated downloadConfig and downloads the file specefied by the URL, storing it in the store implementation that is passed in, in the directory specefied by the prefix, given the number of extra characters from the URL specified by backChars.
func download(config interface{}) (error, bool) {
	dc, ok := config.(downloadConfig)
	if !ok {
		return errors.New("WRONG TYPE!!"), true
	}
	// Get a handle on our object in GCS where we will store the file
	filename := dc.url[strings.LastIndex(dc.url, "/")+1-dc.backChars:]
	obj := dc.fileStore.getFile(dc.prefix + filename)
	w := obj.getWriter()

	// Grab the file from the website
	resp, err := http.Get(dc.url)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Web Get"}).Inc()
		return err, false
	}

	if resp.StatusCode != http.StatusOK {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Webserver gave non-ok response"}).Inc()
		resp.Body.Close()
		return errors.New("URL:" + dc.url + " gave response code " + resp.Status), false
	}

	// Move the file into GCS
	if _, err = io.Copy(w, resp.Body); err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Copy Error"}).Inc()
		return err, false
	}
	w.Close()
	resp.Body.Close()

	// Check to make sure we didn't just download a duplicate, and delete it if we did.
	fileNew := determineIfFileIsNew(dc.fileStore, dc.prefix+filename, dc.prefix+filename[:dc.backChars])
	if !fileNew {
		err = obj.deleteFile()
		if err != nil {
			DownloaderErrorCount.With(prometheus.Labels{"source": "Duplication Deletion Error"}).Inc()
			return err, true
		}
	}
	return nil, false
}

// runFunctionWithRetry takes a struct and a function to pass it to and will run that function, giving it that argument. If the function returns a non-nil error, the function will be retried unless it also returned a boolean flag specefying that it encountered an unrecoverable error. It also takes a retryTimeMin to wait after the first failure before retrying. After each failure, it will wait twice as long until it reaches the retryTimeMax, which makes it return the last error it encountered.
func runFunctionWithRetry(function func(interface{}) (error, bool), config interface{}, retryTimeMin time.Duration, retryTimeMax time.Duration) error {
	retryTime := retryTimeMin
	for err, forceIgnore := function(config); err != nil; err, forceIgnore = function(config) {
		if forceIgnore || retryTime > retryTimeMax {
			return err
		}
		log.Println(err)
		time.Sleep(retryTime)
		retryTime = retryTime * 2
	}
	return nil
}

// determineIfFileIsNew takes an implementation of the store interface, a filename, and a search dir and determines if any of the files in the search dir are duplicates of the file given by filename. If there is a duplicate then the file is not new and it returns false. If there is not duplicate (or if we are unsure, just to be safe) we return true, indicating that the file is new and should be kept.
func determineIfFileIsNew(fileStore store, fileName string, searchDir string) bool {
	md5Hash, err := getHashOfFile(fileStore.getFile(fileName))
	if err != nil {
		log.Println(err)
		return true
	}
	objects := fileStore.getFiles(searchDir)
	return checkIfHashIsUniqueInList(md5Hash, objects, fileName)
}

// getHashOfGCSFile takes an implementation of the fileObject interface and returns the MD5 hash of that fileObject, or an error if we cannot get the hash
func getHashOfFile(obj fileObject) ([]byte, error) {
	attrs, err := obj.getAttrs()
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Couldn't get GCS File Attributes for hash generation"}).Inc()
		return nil, err
	}
	return attrs.getMD5(), nil
}

// checkIfHashIsUniqueInList takes an MD5 hash, a slice of fileAttributes, and a filename corresponding to the MD5 hash. It will return false if it finds another file in the slice with a matching MD5 and a different filename. Otherwise, it will return true.
func checkIfHashIsUniqueInList(md5Hash []byte, fileAttrsList []fileAttributes, fileName string) bool {
	if fileAttrsList == nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Couldn't get list of other files in directory"}).Inc()
		return true
	}
	for _, otherFile := range fileAttrsList {
		if bytes.Equal(otherFile.getMD5(), md5Hash) && otherFile.getName() != fileName {
			return false
		}
	}
	return true
}
