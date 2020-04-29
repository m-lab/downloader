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

var routeviewsURLToFilenameRegexp = regexp.MustCompile(`.*(\d{4}/\d{2}/)(.*)`)
var routeviewsFilenameToDedupeRegexp = regexp.MustCompile(`(.*)`)

// urlAndSeqNum is a struct for bundling the Routeview URL and Seqnum
// together into a single struct. This is the return value of the
// genRouteviewsURLs function
type urlAndSeqNum struct {
	URL    string // The URL pointing to the file we need to download
	Seqnum int    // The seqnum of the file, as given in the
	// routeview generation log file. An example of
	// the generation log file can be found at:
	// http://data.caida.org/datasets/routing/routeviews-prefix2as/pfx2as-creation.log
}

// CaidaRouteviewsFiles takes a url pointing to a routeview
// generation log, a directory prefix that the user wants the files
// placed in, a pointer to the SeqNum of the last successful download,
// and the instance of the store interface where the user wants the
// files stored. It will download the files listed in the log file and
// is gaurenteed not to introduce duplicates
func CaidaRouteviewsFiles(logFileURL string, directory string, lastDownloaded *int, canonicalName string, store file.FileStore) error {
	var lastErr error
	routeViewsURLsAndIDs, err := genRouteViewURLs(logFileURL, *lastDownloaded)
	if err != nil {
		return err
	}
	for _, urlAndID := range routeViewsURLsAndIDs {
		dc := config{
			URL:         urlAndID.URL,
			Store:       store,
			PathPrefix:  directory,
			FilePrefix:  "",
			CurrentName: canonicalName,
			URLRegexp:   routeviewsURLToFilenameRegexp,
			DedupRegexp: routeviewsFilenameToDedupeRegexp,
		}
		if err := RunFunctionWithRetry(download, dc, waitAfterFirstDownloadFailure,
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

// genRouteViewURLs takes a URL pointing to a routeview log file, and
// an integer corresponding to the seqnum of the last successful file
// download. It returns a slice of urlAndSeqNum structs which contain
// the files that the user needs to download from the routeview
// webserver.
func genRouteViewURLs(logFileURL string, lastDownloaded int) ([]urlAndSeqNum, error) {
	var urlsAndIDs []urlAndSeqNum

	// Compile parser regex
	re := regexp.MustCompile(`(\d{1,6})\s*(\d{10})\s*(.*)`)

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
				urlAndSeqNum{logFileURL[:strings.LastIndex(logFileURL, "/")+1] + match[3], seqNum})
		}
	}
	return urlsAndIDs, nil
}
