package main

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var urls []string = []string{
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
	metrics_downloadsRetrying = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_Downloads_Retrying",
		Help: "The number of downloads that are currently queued to later retry.",
	})
	metrics_lastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "annotator_Last_Successful_Time",
		Help: "The time that the downloads last completed successfully.",
	})
)

func setupPrometheus() {
	http.Handle("/metrics", promhttp.Handler())
	prometheus.MustRegister(metrics_downloadsRetrying)
	prometheus.MustRegister(metrics_lastSuccess)
}

func main() {
	setupPrometheus()
	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	for {
		time.Sleep(1 * time.Minute)
	}
}

func download(url string) {
	resp, err := http.Get(url)
	if err != nil {
		handleError(url, err)
	}
}

func handleError(url string, err error) {

}
