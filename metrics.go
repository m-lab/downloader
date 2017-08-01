import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
