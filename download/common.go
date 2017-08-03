package download

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/m-lab/downloader/file"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const waitAfterFirstDownloadFailure = time.Minute * time.Duration(1)      // The time (in minutes) to wait before the first retry of a failed download
const maximumWaitBetweenDownloadAttempts = time.Minute * time.Duration(8) // The maximum time (in minutes) to wait in between download attempts

// TODO(JosephMarques): Find a better method than using backChars. Possibly regex?
// downloadConfig is a struct for bundling parameters to be passed through runFunctionWithRetry to the download function.
type DownloadConfig struct {
	URL       string         // The URL of the file to download
	Store     file.FileStore // The FileStore in which to place the file
	Prefix    string         // The prefix to append to the file name after it's downloaded
	BackChars int            // The number of extra characters from the URL to include in the file name
}

// Download takes a fully populated downloadConfig and downloads the file specefied by the URL,
// storing it in the store implementation that is passed in, in the directory specefied by the prefix,
// given the number of extra characters from the URL specified by backChars.
// The error value indicates the error, if any occurred.
// If the error value is not nil, then the boolean will also be set.
// If the boolean is true, that means that the error cannot be fixed by retrying the download.
// If the boolean is false, that means that the download might work if you attempt it again.
// If the error value is nil, then the value of the boolean is meaningless.
func Download(config interface{}) (error, bool) {
	dc, ok := config.(DownloadConfig)
	if !ok {
		return errors.New("WRONG TYPE!!"), true
	}

	// Grab the file from the website
	resp, err := http.Get(dc.URL)
	if err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Web Get"}).Inc()
		return err, false
	}
	// Ensure that the webserver thinks our file request was okay
	if resp.StatusCode != http.StatusOK {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Webserver gave non-ok response"}).Inc()
		resp.Body.Close()
		return errors.New("URL:" + dc.URL + " gave response code " + resp.Status), false
	}

	// Get a handle on our object in GCS where we will store the file
	filename := dc.URL[strings.LastIndex(dc.URL, "/")+1-dc.BackChars:]
	obj := dc.Store.GetFile(dc.Prefix + filename)
	w := obj.GetWriter()

	// Move the file into GCS
	if _, err = io.Copy(w, resp.Body); err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Copy Error"}).Inc()
		return err, false
	}
	w.Close()
	resp.Body.Close()

	// Check to make sure we didn't just download a duplicate, and delete it if we did.
	fileNew := DetermineIfFileIsNew(dc.Store, dc.Prefix+filename, dc.Prefix+filename[:dc.BackChars])
	if !fileNew {
		err = obj.DeleteFile()
		if err != nil {
			metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Duplication Deletion Error"}).Inc()
			return err, true
		}
	}
	return nil, false
}

// RunFunctionWithRetry takes a struct and a function to pass it to and will run that function,
// giving it that argument. If the function returns a non-nil error,
// the function will be retried unless it also returned a boolean flag specifying
// that it encountered an unrecoverable error.
// It also takes a retryTimeMin to wait after the first failure before retrying.
// After each failure, it will wait twice as long until it reaches the retryTimeMax,
// which makes it return the last error it encountered.
func RunFunctionWithRetry(function func(interface{}) (error, bool), config interface{}, retryTimeMin time.Duration, retryTimeMax time.Duration) error {
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

// DetermineIfFileIsNew takes an implementation of the FileStore interface,
// a filename, and a search dir and determines if any of the files in the
// search dir are duplicates of the file given by filename.
// If there is a duplicate then the file is not new and it returns false.
// If there is not duplicate (or if we are unsure, just to be safe) we return true,
// indicating that the file might be new and should be kept.
func DetermineIfFileIsNew(store file.FileStore, fileName string, searchDir string) bool {
	md5Hash, ok := store.NamesToMD5(fileName)[fileName]
	if !ok {
		log.Println("Couldn't find file for hash generation!!!")
		return true
	}
	md5Map := store.NamesToMD5(searchDir)
	return CheckIfHashIsUniqueInList(md5Hash, md5Map, fileName)
}

// CheckIfHashIsUniqueInList takes an MD5 hash, a map of names to MD5 hashes,
// and a filename corresponding to the MD5 hash.
// It will return false if it finds another file in the slice
// with a matching MD5 and a different filename.
// Otherwise, it will return true.
func CheckIfHashIsUniqueInList(md5Hash []byte, md5Map map[string][]byte, fileName string) bool {
	for otherName, otherMD5 := range md5Map {
		if bytes.Equal(otherMD5, md5Hash) && otherName != fileName {
			return false
		}
	}
	return true
}
