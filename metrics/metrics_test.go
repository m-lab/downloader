package metrics_test

import (
	"testing"

	"github.com/m-lab/go/prometheusx"

	"github.com/m-lab/downloader/metrics"
)

func TestMetrics(t *testing.T) {
	// Give the labeled metrics some labels to make them appear in the output.
	metrics.FailedDownloadCount.WithLabelValues("x").Inc()
	metrics.DownloaderErrorCount.WithLabelValues("x").Inc()
	metrics.RouteviewsURLErrorCount.WithLabelValues("x").Inc()
	prometheusx.LintMetrics(t)
}
