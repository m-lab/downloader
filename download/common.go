package download

import (
	"bytes"
	"context"
	"errors"
	"flag"
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

var (
	waitAfterFirstDownloadFailure      = flag.Duration("download.waitafterfirstfailure", time.Minute, "How long to wait after seeing the first download failure")
	maximumWaitBetweenDownloadAttempts = flag.Duration("download.maxwaitbetweenattempts", 8*time.Minute, "The maximum amount of time to wait between attempts to download data before we consider the download to have failed")
	downloadTimeout                    = flag.Duration("download.timeout", 30*time.Minute, "The maxmimum amount of time a single download+save sequence should take")
)

// config is a struct for bundling parameters to be passed through
// runFunctionWithRetry to the download function.
type config struct {
	URL         string         // The URL of the file to download
	Store       file.Store     // The file.Store in which to place the file
	PathPrefix  string         // The prefix to attach to the file's path after it's downloaded
	CurrentName string         // The name to give the most recent version of the file.
	FilePrefix  string         // The prefix to attach to the filename after it's downloaded
	URLRegexp   *regexp.Regexp // The regular expression to apply to the URL to create the filename.
	// The first matching group will go before the timestamp, the second after.
	DedupRegexp   *regexp.Regexp // The regexp to apply to the filename to determine the directory to dedupe in.
	FixedFilename string         // The saved file could have fixed filename.
	MaxDuration   time.Duration  // The longest we allow the download process to go on before we consider it failed.
	BasicAuthUser string         // The HTTP Basic Auth user string
	BasicAuthPass string         // The HTTP Basic Auth password string
}

// GenUniformSleepTime generates a random time to sleep (in hours)
// that is on average, the time given by sleepInterval. It will give a
// random time in the interval specefied by sleepDeviation (centered
// around sleepInterval).
func GenUniformSleepTime(sleepInterval time.Duration, sleepDeviation time.Duration) time.Duration {
	return time.Duration((rand.Float64()-0.5)*float64(sleepDeviation)) + sleepInterval
}

// download takes a fully populated download.config and downloads the
// file specified by the URL, storing it in the store implementation
// that is passed in, in the directory specefied by the prefix, given
// the number of extra characters from the URL specified by
// backChars. The error value indicates the error, if any occurred. If
// the error value is not nil, then the boolean will also be set. If
// the boolean is true, that means that the error cannot be fixed by
// retrying the download. If the boolean is false, that means that the
// download might work if you attempt it again. If the error value is
// nil, then the value of the boolean is meaningless.
func download(ctx context.Context, dc config) errWithPermanence {
	ctx, cancel := context.WithTimeout(ctx, dc.MaxDuration)
	defer cancel()

	// Grab the file from the website.
	req, err := http.NewRequest(http.MethodGet, dc.URL, nil)
	if err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Web Get"}).Inc()
		return errWithPermanence{err, false}
	}

	req.Close = true

	// If an HTTP Basic Auth user is defined, then add Basic Auth headers to the request.
	if dc.BasicAuthUser != "" {
		req.SetBasicAuth(dc.BasicAuthUser, dc.BasicAuthPass)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Web Get"}).Inc()
		return errWithPermanence{err, false}
	}

	// Ensure that the webserver thinks our file request was okay
	if resp.StatusCode != http.StatusOK {
		metrics.DownloaderErrorCount.
			With(prometheus.Labels{"source": "Webserver gave non-ok response"}).Inc()
		resp.Body.Close()
		return errWithPermanence{errors.New("URL:" + dc.URL + " gave response code " + resp.Status), false}
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
	w := obj.GetWriter(ctx)

	// Move the file into GCS
	if _, err = io.Copy(w, resp.Body); err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Copy Error"}).Inc()
		return errWithPermanence{err, false}
	}
	w.Close()
	resp.Body.Close()

	// If we downloaded a new file, save it to current.  If it wasn't new, delete it.
	if IsFileNew(ctx, dc.Store, filename, dc.DedupRegexp.FindAllStringSubmatch(filename, -1)[0][1]) {
		if dc.CurrentName != "" {
			err = obj.CopyTo(ctx, dc.CurrentName)
			if err != nil {
				metrics.DownloaderErrorCount.
					With(prometheus.Labels{"source": "Copy to Current Error"}).Inc()
				return errWithPermanence{err, true}
			}
		}
	} else {
		err = obj.DeleteFile(ctx)
		if err != nil {
			metrics.DownloaderErrorCount.
				With(prometheus.Labels{"source": "Duplication Deletion Error"}).Inc()
			return errWithPermanence{err, true}
		}
	}
	return errWithPermanence{}
}

type errWithPermanence struct {
	error
	permanent bool
}

// runFunctionWithRetry takes a struct and a function to pass it to
// and will run that function, giving it that argument. If the
// function returns a non-nil error, the function will be retried
// unless it also returned a boolean flag specifying that it
// encountered an unrecoverable error. It also takes a retryTimeMin to
// wait after the first failure before retrying. After each failure,
// it will wait twice as long until it reaches the retryTimeMax, which
// makes it return the last error it encountered.
func runFunctionWithRetry(ctx context.Context, function func(context.Context, config) errWithPermanence, config config,
	retryTimeMin time.Duration, retryTimeMax time.Duration) error {

	retryTime := retryTimeMin
	for err := function(ctx, config); err.error != nil && ctx.Err() == nil; err = function(ctx, config) {
		log.Printf("Download failed: %+v\n", err)
		if err.permanent || retryTime > retryTimeMax {
			return err.error
		}
		time.Sleep(retryTime)
		retryTime = retryTime * 2
	}
	return nil
}

// IsFileNew takes an implementation of the FileStore
// interface, a filename, and a search dir and determines if any of
// the files in the search dir are duplicates of the file given by
// filename. If there is a duplicate then the file is not new and it
// returns false. If there is not duplicate (or if we are unsure, just
// to be safe) we return true, indicating that the file might be new
// and should be kept.
func IsFileNew(ctx context.Context, store file.Store, fileName string, searchDir string) bool {
	md5Hash, ok := store.NamesToMD5(ctx, fileName)[fileName]
	if !ok {
		log.Println("Couldn't find file for hash generation!!!")
		return true
	}
	md5Map := store.NamesToMD5(ctx, searchDir)
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
