package download

import (
	"bytes"
	"errors"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	"github.com/m-lab/downloader/file"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// WaitAfterFirstDownloadFailure is the time (in minutes) to wait before the
// first retry of a failed download
var WaitAfterFirstDownloadFailure = time.Minute * time.Duration(1)

// MaximumWaitBetweenDownloadAttempts is the maximum time (in minutes) to wait
// in between download attempts
var MaximumWaitBetweenDownloadAttempts = time.Minute * time.Duration(8)

// Config is a struct for bundling parameters to be passed through
// runFunctionWithRetry to the download function.
//
// TODO: Find a better method than using backChars. Possibly regex?
type Config struct {
	URL         string         // The URL of the file to download
	Store       file.FileStore // The FileStore in which to place the file
	PathPrefix  string         // The prefix to attach to the file's path after it's downloaded
	CurrentName string         // The name to give the most recent version of the file.
	FilePrefix  string         // The prefix to attach to the filename after it's downloaded
	URLRegexp   *regexp.Regexp // The regular expression to apply to the URL to create the filename.
	// The first matching group will go before the timestamp, the second after.
	DedupRegexp   *regexp.Regexp // The regexp to apply to the filename to determine the directory to dedupe in.
	FixedFilename string         // The saved file could have fixed filename.
}

// GenUniformSleepTime generatres a random time to sleep (in hours)
// that is on average, the time given by sleepInterval. It will give a
// random time in the interval specefied by sleepDeviation (centered
// around sleepInterval).
func GenUniformSleepTime(sleepInterval time.Duration, sleepDeviation time.Duration) time.Duration {
	return time.Duration((rand.Float64()-0.5)*float64(sleepDeviation)) + sleepInterval
}

// Download takes a fully populated downloadConfig and downloads the
// file specefied by the URL, storing it in the store implementation
// that is passed in, in the directory specefied by the prefix, given
// the number of extra characters from the URL specified by
// backChars. The error value indicates the error, if any occurred. If
// the error value is not nil, then the boolean will also be set. If
// the boolean is true, that means that the error cannot be fixed by
// retrying the download. If the boolean is false, that means that the
// download might work if you attempt it again. If the error value is
// nil, then the value of the boolean is meaningless.
func Download(config interface{}) (error, bool) {
	dc, ok := config.(Config)
	if !ok {
		return errors.New("wrong configuration type passed to Download()"), true
	}

	// Grab the file from the website
	resp, err := http.Get(dc.URL)
	if err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Web Get"}).Inc()
		return err, false
	}
	// Ensure that the webserver thinks our file request was okay
	if resp.StatusCode != http.StatusOK {
		metrics.DownloaderErrorCount.
			With(prometheus.Labels{"source": "Webserver gave non-ok response"}).Inc()
		resp.Body.Close()
		return errors.New("URL:" + dc.URL + " gave response code " + resp.Status), false
	}

	// Get a handle on our object in GCS where we will store the file
	var filename string
	if dc.FixedFilename != "" {
		filename = dc.PathPrefix + dc.FilePrefix + dc.FixedFilename
	} else {
		urlMatches := dc.URLRegexp.FindAllStringSubmatch(dc.URL, -1)
		filename = dc.PathPrefix + urlMatches[0][1] + dc.FilePrefix + urlMatches[0][2]
	}
	obj := dc.Store.GetFile(filename)
	w := obj.GetWriter()

	// Move the file into GCS
	if _, err = io.Copy(w, resp.Body); err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Copy Error"}).Inc()
		return err, false
	}
	w.Close()
	resp.Body.Close()

	// If we downloaded a new file, save it to current.  If it wasn't new, delete it.
	if IsFileNew(dc.Store, filename, dc.DedupRegexp.FindAllStringSubmatch(filename, -1)[0][1]) {
		if dc.CurrentName != "" {
			err = obj.CopyTo(dc.CurrentName)
			if err != nil {
				metrics.DownloaderErrorCount.
					With(prometheus.Labels{"source": "Copy to Current Error"}).Inc()
				return err, true
			}
		}
	} else {
		err = obj.DeleteFile()
		if err != nil {
			metrics.DownloaderErrorCount.
				With(prometheus.Labels{"source": "Duplication Deletion Error"}).Inc()
			return err, true
		}
	}
	return nil, false
}

// RunFunctionWithRetry takes a struct and a function to pass it to
// and will run that function, giving it that argument. If the
// function returns a non-nil error, the function will be retried
// unless it also returned a boolean flag specifying that it
// encountered an unrecoverable error. It also takes a retryTimeMin to
// wait after the first failure before retrying. After each failure,
// it will wait twice as long until it reaches the retryTimeMax, which
// makes it return the last error it encountered.
func RunFunctionWithRetry(function func(interface{}) (error, bool), config interface{},
	retryTimeMin time.Duration, retryTimeMax time.Duration) error {

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

// CopyFile copies one file in GCS to another.

// IsFileNew takes an implementation of the FileStore
// interface, a filename, and a search dir and determines if any of
// the files in the search dir are duplicates of the file given by
// filename. If there is a duplicate then the file is not new and it
// returns false. If there is not duplicate (or if we are unsure, just
// to be safe) we return true, indicating that the file might be new
// and should be kept.
func IsFileNew(store file.FileStore, fileName string, searchDir string) bool {
	md5Hash, ok := store.NamesToMD5(fileName)[fileName]
	if !ok {
		log.Println("Couldn't find file for hash generation!!!")
		return true
	}
	md5Map := store.NamesToMD5(searchDir)
	return CheckIfHashIsUniqueInList(md5Hash, md5Map, fileName)
}

// CheckIfHashIsUniqueInList takes an MD5 hash, a map of names to MD5
// hashes, and a filename corresponding to the MD5 hash. It will
// return false if it finds another file in the slice with a matching
// MD5 and a different filename. Otherwise, it will return true.
func CheckIfHashIsUniqueInList(md5Hash []byte, md5Map map[string][]byte, fileName string) bool {
	for otherName, otherMD5 := range md5Map {
		if bytes.Equal(otherMD5, md5Hash) && otherName != fileName {
			return false
		}
	}
	return true
}
