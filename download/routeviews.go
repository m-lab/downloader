package download

import (
	"bytes"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/m-lab/downloader/file"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// urlAndSeqNum is a struct for bundling the Routeview URL and Seqnum
// together into a single struct. This is the return value of the
// genRouteviewsURLs function
type UrlAndSeqNum struct {
	URL    string // The URL pointing to the file we need to download
	Seqnum int    // The seqnum of the file, as given in the routeview generation log file
}

// DownloadRouteviewsFiles takes a url pointing to a routeview
// generation log, a directory prefix that the user wants the files
// placed in, a pointer to the SeqNum of the last successful download,
// and the instance of the store interface where the user wants the
// files stored. It will download the files listed in the log file and
// is gaurenteed not to introduce duplicates
func DownloadCaidaRouteviewsFiles(logFileURL string, directory string, lastDownloaded *int, store file.FileStore) error {
	var lastErr error = nil
	routeViewsURLsAndIDs, err := GenRouteViewURLs(logFileURL, *lastDownloaded)
	if err != nil {
		return err
	}
	for _, urlAndID := range routeViewsURLsAndIDs {
		dc := DownloadConfig{URL: urlAndID.URL, Store: store, Prefix: directory, BackChars: 8}
		if err := RunFunctionWithRetry(Download, dc, waitAfterFirstDownloadFailure,
			maximumWaitBetweenDownloadAttempts); err != nil {

			lastErr = err
			metrics.FailedDownloadCount.With(prometheus.Labels{"download_type": directory}).Inc()
		}
		if lastErr == nil {
			*lastDownloaded = urlAndID.Seqnum
		}
	}
	return lastErr

}

// GenRouteViewsURLs takes a URL pointing to a routeview log file, and
// an integer corresponding to the seqnum of the last successful file
// download. It returns a slice of urlAndSeqNum structs which contain
// the files that the user needs to download from the routeview
// webserver.
func GenRouteViewURLs(logFileURL string, lastDownloaded int) ([]UrlAndSeqNum, error) {
	var urlsAndIDs []UrlAndSeqNum = nil

	// Compile parser regex
	re, err := regexp.Compile(`(\d{1,6})\s*(\d{10})\s*(.*)`)
	if err != nil {
		metrics.RouteviewsURLErrorCount.With(prometheus.Labels{"source": "Regex Compilation Error"}).Inc()
		return nil, err
	}

	// Get the generation log file from the routeviews website
	resp, err := http.Get(logFileURL)
	if err != nil {
		metrics.RouteviewsURLErrorCount.
			With(prometheus.Labels{"source": "Couldn't grab the log file from the Routeviews server."}).Inc()
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		metrics.RouteviewsURLErrorCount.With(prometheus.Labels{"source": "Webserver gave non-ok response"}).Inc()
		return nil, errors.New("URL:" + logFileURL + " gave response code " + resp.Status)
	}

	// Match parse the data we need from the log file
	responseBodyBuffer := new(bytes.Buffer)
	responseBodyBuffer.ReadFrom(resp.Body)
	matches := re.FindAllStringSubmatch(responseBodyBuffer.String(), -1)

	// Check the file to find files with a higher ID number than
	// our last downloaded and add them to the list of files to
	// grab
	for _, match := range matches {
		seqNum, err := strconv.Atoi(match[1])
		if err != nil {
			metrics.RouteviewsURLErrorCount.
				With(prometheus.Labels{"source": "Regex is matching non-numbers where it should not."}).Inc()
			continue
		}
		if seqNum > lastDownloaded {
			urlsAndIDs = append(urlsAndIDs,
				UrlAndSeqNum{logFileURL[:strings.LastIndex(logFileURL, "/")+1] + match[3], seqNum})
		}
	}
	return urlsAndIDs, nil
}
