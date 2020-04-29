// The metrics package defines a set of metrics for
// monitoring the downloader and provides a function
// to initialize those metrics on the /metrics path

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

//These vars are the prometheus metrics
var (
	// Always set to the last time we had a successful download of ALL files
	// Provides metrics:
	//    downloader_last_success_time_seconds
	// Example usage:
	//    LastSuccessTime.Inc()
	LastSuccessTime = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_last_success_time_seconds",
		Help: "The time that ALL the downloads last completed successfully.",
	})

	// Measures the number of downloads that have failed completely
	// Provides metrics:
	//    downloader_download_failed_total
	// Example usage:
	//    FailedDownloadCount.Inc()
	FailedDownloadCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_download_failed_total",
		Help: "Increments every time a download maxes out our number of retries.",
	}, []string{"download_type"})

	// Measures the number of downloader errors
	// Provides metrics:
	//    downloader_error_total
	// Example usage:
	//    DownloaderErrorCount.Inc()
	DownloaderErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_error_total",
		Help: "The current number of unresolved errors encountered while attempting to download the latest maxmind and routeviews data.",
	}, []string{"source"})

	// Measures the number of errors involved with getting the list of routeview files
	// Provides metrics:
	//    downloader_downloader_routeviews_url_error_total
	// Example usage:
	//    RouteviewsURLErrorCount.Inc()
	RouteviewsURLErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "downloader_downloader_routeviews_url_error_total",
		Help: "The number of errors that occured with retrieving the Routeviews URL list.",
	}, []string{"source"})
)
