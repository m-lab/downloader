package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const waitAfterFirstDownloadFailure = time.Minute * time.Duration(1)      // The time (in minutes) to wait before the first retry of a failed download
const maximumWaitBetweenDownloadAttempts = time.Minute * time.Duration(8) // The maximum time (in minutes) to wait in between download attempts
const averageHoursBetweenUpdateChecks = 8                                 // The average time (in hours) to wait in between attempts to download files
const windowForRandomTimeBetweenUpdateChecks = 8                          // The window of time (in hours) to allow a random time to be chosen from.

// urlAndSeqNum is a struct for bundling the Routeview URL and Seqnum together into a single struct. This is the return value of the genRouteviewsURLs function
type urlAndSeqNum struct {
	url    string // The URL pointing to the file we need to download
	seqnum int    // The seqnum of the file, as given in the routeview generation log file
}

// downloadConfig is a struct for bundling parameters to be passed through runFunctionWithRetry to the download function.
type downloadConfig struct {
	url               string // The URL of the file to download
	fileStore         store  // The store in which to place the file
	prefix            string // The prefix to append to the file name after it's downloaded
	backChars         int    // The number of extra characters from the URL to include in the file name
	dedupePrefixDepth int    // The number of characters into the prefix we should search when looking for duplicate files
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
	//    downloader_download_failed_count
	// Example usage:
	//    FailedDownloadCount.Inc()
	FailedDownloadCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_download_failed_count",
		Help: "Increments every time a download maxes out our number of retries.",
	}, []string{"download_type"})

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

// setupPrometheus takes no arguments and sets up prometheus metrics for the package
func setupPrometheus() {
	http.Handle("/metrics", promhttp.Handler())
	prometheus.MustRegister(LastSuccessTime)
	prometheus.MustRegister(FailedDownloadCount)
	prometheus.MustRegister(DownloaderErrorCount)
	prometheus.MustRegister(RouteviewsURLErrorCount)
}

// The main function seeds the random number generator, starts prometheus in the background, takes the bucket flag from the command line, and kicks off the actual downloader loop
func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	setupPrometheus()
	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	bucketName := flag.String("bucket", "", "Specify the bucket name to store the results in.")
	flag.Parse()
	if *bucketName == "" {
		log.Fatal("NO BUCKET SPECIFIED!!!")
	}
	loopOverURLsForever(*bucketName)
}

// loopOverURLsForever takes a bucketName, pointing to a GCS bucket, and then tries to download the files over and over again until the end of time (waiting an average of 8 hours in between attempts)
func loopOverURLsForever(bucketName string) {
	lastDownloadedV4 := 0
	lastDownloadedV6 := 0
	for {
		timestamp := time.Now().Format("2006/01/02/15:04:05-")
		bkt, err := constructBucketHandle(bucketName)
		if err != nil {
			continue
		}
		fileStore := &storeGCS{bkt: bkt}

		maxmindErr := downloadMaxmindFiles(maxmindURLs, timestamp, fileStore)
		if maxmindErr != nil {
			log.Println(maxmindErr)
		}

		routeviewIPv4Err := downloadRouteviewsFiles("http://data.caida.org/datasets/routing/routeviews-prefix2as/pfx2as-creation.log", "RouteViewIPv4/", &lastDownloadedV4, fileStore)
		if routeviewIPv4Err != nil {
			log.Println(routeviewIPv4Err)
		}

		routeviewIPv6Err := downloadRouteviewsFiles("http://data.caida.org/datasets/routing/routeviews6-prefix2as/pfx2as-creation.log", "RouteViewIPv6/", &lastDownloadedV6, fileStore)
		if routeviewIPv6Err != nil {
			log.Println(routeviewIPv6Err)
		}

		if maxmindErr == nil && routeviewIPv4Err == nil && routeviewIPv6Err == nil {
			LastSuccessTime.SetToCurrentTime()
		}
		time.Sleep(time.Duration(genUniformSleepTime(averageHoursBetweenUpdateChecks, windowForRandomTimeBetweenUpdateChecks)) * time.Hour)
	}
}

// downloadMaxmindFiles takes a slice of urls pointing to maxmind files, a timestamp that the user wants attached to the files, and the instance of the store interface where the user wants the files stored. It then downloads the files, stores them, and returns and error on failure or nil on success. Gaurenteed to not introduce duplicates.
func downloadMaxmindFiles(urls []string, timestamp string, fileStore store) error {
	var lastErr error = nil
	for _, url := range urls {
		dc := downloadConfig{url: url, fileStore: fileStore, prefix: "Maxmind/" + timestamp, backChars: 0, dedupePrefixDepth: 16}
		if err := runFunctionWithRetry(download, dc, waitAfterFirstDownloadFailure, maximumWaitBetweenDownloadAttempts); err != nil {
			lastErr = err
			FailedDownloadCount.With(prometheus.Labels{"download_type": "Maxmind"}).Inc()
		}
	}
	return lastErr

}

// downloadRouteviewsFiles takes a url pointing to a routeview generation log, a directory prefix that the user wants the files placed in, a pointer to the SeqNum of the last successful download, and the instance of the store interface where the user wants the files stored. It will download the files listed in the log file and is gaurenteed not to introduce duplicates
func downloadRouteviewsFiles(logFileURL string, directory string, lastDownloaded *int, fileStore store) error {
	var lastErr error = nil
	routeViewsURLsAndIDs, err := genRouteViewURLs(logFileURL, *lastDownloaded)
	if err != nil {
		return err
	}
	for _, urlAndID := range routeViewsURLsAndIDs {
		dc := downloadConfig{url: urlAndID.url, fileStore: fileStore, prefix: directory, backChars: 8, dedupePrefixDepth: len(directory)}
		if err := runFunctionWithRetry(download, dc, waitAfterFirstDownloadFailure, maximumWaitBetweenDownloadAttempts); err != nil {
			lastErr = err
			FailedDownloadCount.With(prometheus.Labels{"download_type": directory}).Inc()
		}
		if lastErr == nil {
			*lastDownloaded = urlAndID.seqnum
		}
	}
	return lastErr

}

// genSleepTime generatres a random time to sleep (in hours) that is on average, the time given by sleepInterval. It will give a random time in the interval specefied by sleepDeviation (centered around sleepInterval).
func genUniformSleepTime(sleepInterval float64, sleepDeviation float64) float64 {
	return (rand.Float64()-0.5)*sleepDeviation + sleepInterval
}

// constructBucketHandle takes a bucket name and safely loads it, returning either the handle to the bucket or an error
func constructBucketHandle(bucketName string) (*storage.BucketHandle, error) {
	ctx, _ := context.WithTimeout(context.Background(), 2*time.Minute)
	client, err := storage.NewClient(ctx)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Client Setup"}).Inc()
		return nil, err
	}
	return client.Bucket(bucketName), nil
}

// download takes a fully populated downloadConfig and downloads the file specefied by the URL, storing it in the store implementation that is passed in, in the directory specefied by the prefix, given the number of extra characters from the URL specified by backChars.
func download(config interface{}) (error, bool) {
	dc, ok := config.(downloadConfig)
	if !ok {
		return errors.New("WRONG TYPE!!"), true
	}

	// Grab the file from the website
	resp, err := http.Get(dc.url)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Web Get"}).Inc()
		return err, false
	}
	// Ensure that the webserver thinks our file request was okay
	if resp.StatusCode != http.StatusOK {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Webserver gave non-ok response"}).Inc()
		resp.Body.Close()
		return errors.New("URL:" + dc.url + " gave response code " + resp.Status), false
	}

	// Get a handle on our object in GCS where we will store the file
	filename := dc.url[strings.LastIndex(dc.url, "/")+1-dc.backChars:]
	obj := dc.fileStore.getFile(dc.prefix + filename)
	w := obj.getWriter()

	// Move the file into GCS
	if _, err = io.Copy(w, resp.Body); err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Copy Error"}).Inc()
		return err, false
	}
	w.Close()
	resp.Body.Close()

	// Check to make sure we didn't just download a duplicate, and delete it if we did.
	fileNew := determineIfFileIsNew(dc.fileStore, dc.prefix+filename, dc.prefix[:dc.dedupePrefixDepth]+filename[:dc.backChars])
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
	md5Hash, err := fileStore.getFile(fileName).getMD5()
	if err != nil {
		log.Println(err)
		return true
	}
	objects := fileStore.getFiles(searchDir)
	return checkIfHashIsUniqueInList(md5Hash, objects, fileName)
}

// checkIfHashIsUniqueInList takes an MD5 hash, a slice of fileAttributes, and a filename corresponding to the MD5 hash. It will return false if it finds another file in the slice with a matching MD5 and a different filename. Otherwise, it will return true.
func checkIfHashIsUniqueInList(md5Hash []byte, fileList []fileObject, fileName string) bool {
	if fileList == nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Couldn't get list of other files in directory"}).Inc()
		return true
	}
	for _, otherFile := range fileList {
		otherMD5, err := otherFile.getMD5()
		if err != nil {
			return true
		}
		otherName, err := otherFile.getName()
		if err != nil {
			return true
		}
		if bytes.Equal(otherMD5, md5Hash) && otherName != fileName {
			return false
		}
	}
	return true
}

// genRouteViewsURLs takes a URL pointing to a routeview log file, and an integer corresponding to the seqnum of the last successful file download. It returns a slice of urlAndSeqNum structs which contain the files that the user needs to download from the routeview webserver.
func genRouteViewURLs(logFileURL string, lastDownloaded int) ([]urlAndSeqNum, error) {
	var urlsAndIDs []urlAndSeqNum = nil

	// Compile parser regex
	re, err := regexp.Compile(`(\d{1,6})\s*(\d{10})\s*(.*)`)
	if err != nil {
		RouteviewsURLErrorCount.With(prometheus.Labels{"source": "Regex Compilation Error"}).Inc()
		return nil, err
	}

	// Get the generation log file from the routeviews website
	resp, err := http.Get(logFileURL)
	if err != nil {
		RouteviewsURLErrorCount.With(prometheus.Labels{"source": "Couldn't grab the log file from the Routeviews server."}).Inc()
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		RouteviewsURLErrorCount.With(prometheus.Labels{"source": "Webserver gave non-ok response"}).Inc()
		return nil, errors.New("URL:" + logFileURL + " gave response code " + resp.Status)
	}

	// Match parse the data we need from the log file
	responseBodyBuffer := new(bytes.Buffer)
	responseBodyBuffer.ReadFrom(resp.Body)
	matches := re.FindAllStringSubmatch(responseBodyBuffer.String(), -1)

	// Check the file to find files with a higher ID number than our last downloaded and add them to the list of files to grab
	for _, match := range matches {
		seqNum, err := strconv.Atoi(match[1])
		if err != nil {
			RouteviewsURLErrorCount.With(prometheus.Labels{"source": "Regex is matching non-numbers where it should not."}).Inc()
			continue
		}
		if seqNum > lastDownloaded {
			urlsAndIDs = append(urlsAndIDs, urlAndSeqNum{logFileURL[:strings.LastIndex(logFileURL, "/")+1] + match[3], seqNum})
		}
	}
	return urlsAndIDs, nil
}
