package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/iterator"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const retryTimeSeed = 1 // The time (in minutes) to wait before the first retry of a failed download
const sleepInterval = 8 // The average time (in hours) to wait in between attempts to download files

type objIter interface {
	Next() (*storage.ObjectAttrs, error)
}

// URLAndID is a struct for bundling the Routeview URL and Seqnum together into a single struct. This is the return value of the genRouteviewsURLs function
type URLAndID struct {
	URL string // The URL pointing to the file we need to download
	ID  int    // The seqnum of the file, as given in the routeview generation log file
}

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
	//    downloader_Last_Successful_Time
	// Example usage:
	//    LastSuccessTime.Inc()
	LastSuccessTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_Last_Successful_Time",
		Help: "The time that ALL the downloads last completed successfully.",
	})

	// Measures the number of downloads that have failed completely
	// Provides metrics:
	//    downloader_Download_Failed
	// Example usage:
	//    FailedDownloadCount.Inc()
	FailedDownloadCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_Download_Failed",
		Help: "Increments every time a download maxes out our number of retries.",
	}, []string{"DownloadType"})

	// Measures the number of downloader errors
	// Provides metrics:
	//    downloader_Error_Count
	// Example usage:
	//    DownloaderErrorCount.Inc()
	DownloaderErrorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_Error_Count",
		Help: "The current number of unresolved errors encountered while attemting to download the latest maxmind and routeviews data.",
	}, []string{"source"})

	// Measures the number of errors involved with getting the list of routeview files
	// Provides metrics:
	//    downloader_Downloader_Routeviews_URL_Error_Count
	// Example usage:
	//    RouteviewsURLErrorCount.Inc()
	RouteviewsURLErrorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_Downloader_Routeviews_URL_Error_Count",
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

// loopOverURLsForever takes a bucketName and then tries to download the files over and over again until the end of time (waiting an average of 8 hours in between attempts)
func loopOverURLsForever(bucketName string) {
	lastDownloadedV4 := 0
	lastDownloadedV6 := 0
	timestamp := time.Now().Format("2006/01/02/15:04:05-")
	for {
		bkt, err := loadBucket(bucketName)
		if err != nil {
			continue
		}
		maxmindFailure := downloadMaxmindFiles(maxmindURLs, timestamp, bkt)
		routeviewIPv4Failure := downloadRouteviewsFiles("http://data.caida.org/datasets/routing/routeviews-prefix2as/pfx2as-creation.log", "RouteViewIPv4/", &lastDownloadedV4, bkt)
		routeviewIPv6Failure := downloadRouteviewsFiles("http://data.caida.org/datasets/routing/routeviews6-prefix2as/pfx2as-creation.log", "RouteViewIPv6/", &lastDownloadedV6, bkt)
		if !maxmindFailure && !routeviewIPv4Failure && !routeviewIPv6Failure {
			LastSuccessTime.SetToCurrentTime()
		}
		time.Sleep(time.Duration(genSleepTime(sleepInterval)) * time.Hour)
	}
}

// downloadMaxmindFiles takes a slice of urls pointing to maxmind files, a timestamp that the user wants attached to the files, and the handle of the bucket they want the files stored in. It then downloads the files, stores them, and returns true on failure. Gaurenteed to to introduce duplicates.
func downloadMaxmindFiles(urls []string, timestamp string, bkt *storage.BucketHandle) bool {
	failure := false
	for _, url := range urls {
		if err := download(url, retryTimeSeed, bkt, "Maxmind/"+timestamp, 0); err != nil {
			failure = true
			log.Println(err)
			FailedDownloadCount.With(prometheus.Labels{"DownloadType": "Maxmind"}).Inc()
		}
	}
	return failure

}

// downloadRouteviewsFiles takes a url pointing to a routeview generation log, a directory prefix that the user wants the files placed in, a pointer to the ID of the last successful download, and a handle to the bucket it wants the files stored in. It will download the files listed in the log file and is gaurenteed not to introduce duplicates
func downloadRouteviewsFiles(logFileURL string, directory string, lastDownloaded *int, bkt *storage.BucketHandle) bool {
	routeViewsURLsAndIDs, err := genRouteViewURLs(logFileURL, *lastDownloaded)
	if err != nil {
		log.Println(err)
		return true
	}
	routeViewsDownloadFailure := false
	for _, urlAndID := range routeViewsURLsAndIDs {
		if err := download(urlAndID.URL, retryTimeSeed, bkt, directory, 8); err != nil {
			routeViewsDownloadFailure = true
			log.Println(err)
			FailedDownloadCount.With(prometheus.Labels{"DownloadType": directory}).Inc()
		}
		if !routeViewsDownloadFailure {
			*lastDownloaded = urlAndID.ID
		}
	}
	return routeViewsDownloadFailure

}

// genSleepTime generates a random time to sleep (in hours) that is on average, the time given by sleepInterval. It will also max out and cap the return value at 20 hours.
func genSleepTime(sleepInterval float64) float64 {
	sleepTime := rand.ExpFloat64() * sleepInterval
	if sleepTime > 23 {
		sleepTime = 20
	}
	return sleepTime
}

// loadBucket takes a bucket name and safely loads it, returning either the handle to the bucket or an error
func loadBucket(bucketName string) (*storage.BucketHandle, error) {
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Client Setup"}).Inc()
		return nil, err
	}
	return client.Bucket(bucketName), nil
}

// download takes a URL, a time to wait in between attempted downloads, a bucket handle where the download will be stored, a prefix to add to the downloaded files, and a number of characters to add onto the begining of the filename from the URL (in addition to the actual file name given by the url). It will download the file, retrying upon failure, or returning the error if the maximum number of retries has been reached.
func download(url string, retTime int, bkt *storage.BucketHandle, prefix string, cutChars int) error {
	// Get a handle on our object in GCS where we will store the file
	filename := url[strings.LastIndex(url, "/")+1-cutChars:]
	ctx := context.Background()
	obj := bkt.Object(prefix + filename)
	w := obj.NewWriter(ctx)

	// Grab the file from the website
	resp, err := http.Get(url)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Web Get"}).Inc()
		return retryDownloadAfterError(url, err, retTime, bkt, prefix, cutChars)
	}

	// Move the file into GCS
	if _, err = io.Copy(w, resp.Body); err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Copy Error"}).Inc()
		return retryDownloadAfterError(url, err, retTime, bkt, prefix, cutChars)
	}
	w.Close()

	// Check to make sure we didn't just download a duplicate, and delete it if we did.
	fileNew := determineIfFileIsNew(bkt, prefix+filename, prefix+filename[:8])
	if !fileNew {
		err = obj.Delete(ctx)
		if err != nil {
			DownloaderErrorCount.With(prometheus.Labels{"source": "Duplication Deletion Error"}).Inc()
			return err
		}
	}
	return nil
}

// retryDownloadAfterError works in tandem with download to handle the retry logic of the function. Essentially, it waits the time given by retryTime (in minutes), and then retries the download with double the amount of wait time passed into the download function. If the download wait time is beyond 15 minutes, it will simply give up and return the error.
func retryDownloadAfterError(url string, err error, retryTime int, bkt *storage.BucketHandle, prefix string, cutChars int) error {
	if retryTime > 15 {
		return err
	}
	time.Sleep(time.Duration(retryTime) * time.Minute)
	return download(url, retryTime*2, bkt, prefix, cutChars)
}

// determineIfFileIsNew takes a bucket handle, a filename, and a search dir and determines if any of the files in the search dir are duplicates of the file given by filename. If there is a duplicate then the file is not new and it returns false. If there is not duplicate (or if we are unsure, just to be safe) we return true, indicating that the file is new and should be kept.
func determineIfFileIsNew(bkt *storage.BucketHandle, fileName string, searchDir string) bool {
	ctx := context.Background()
	md5Hash, err := getHashOfGCSFile(bkt, fileName)
	if err != nil {
		log.Println(err)
		return true
	}
	objects := bkt.Objects(ctx, &storage.Query{"", searchDir, false})
	return checkIfHashIsUniqueInList(md5Hash, objects, fileName)
}

// getHashOfGCSFile takes a bucket handle and a filename specefying a file in that bucket and returns the MD5 hash of that file, or an error if we cannot get the hash
func getHashOfGCSFile(bkt *storage.BucketHandle, fileName string) ([]byte, error) {
	ctx := context.Background()
	obj := bkt.Object(fileName)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Couldn't get GCS File Attributes for hash generation"}).Inc()
		return nil, err
	}
	return attrs.MD5, nil
}

// checkIfHashIsUniqueInList takes an MD5 hash, an ObjectIterator of file attributes, and a filename corresponding to the MD5 hash. It will return false if it finds another file in the ObjectIterator with a matching MD5 and a different filename. Otherwise, it will return true.
func checkIfHashIsUniqueInList(md5Hash []byte, fileAttributes objIter, fileName string) bool {
	if fileAttributes == nil {
		DownloaderErrorCount.With(prometheus.Labels{"source": "Couldn't get list of other files in directory"}).Inc()
		return true
	}
	for otherFile, err := fileAttributes.Next(); err != iterator.Done; otherFile, err = fileAttributes.Next() {
		if err != nil {
			DownloaderErrorCount.With(prometheus.Labels{"source": "Unkown Error in iterator in checkIfHashIsUniqueInList"}).Inc()
		}
		if bytes.Equal(otherFile.MD5, md5Hash) && otherFile.Name != fileName {
			return false
		}
	}
	return true
}

// genRouteViewsURLs takes a URL pointing to a routeview log file, and an integer corresponding to the seqnum of the last successful file download. It returns a slice of URLAndID structs which contain the files that the user needs to download from the routeview webserver.
func genRouteViewURLs(logFileURL string, lastDownloaded int) ([]URLAndID, error) {
	var urlsAndIDs []URLAndID = nil

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
			urlsAndIDs = append(urlsAndIDs, URLAndID{logFileURL[:strings.LastIndex(logFileURL, "/")+1] + match[3], seqNum})
			lastDownloaded = seqNum
		}
	}
	return urlsAndIDs, nil
}
