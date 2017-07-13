package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/iterator"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
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
		Help: "The number of downloads that had to be queued to later retry.",
	})
	metrics_lastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_Last_Successful_Time",
		Help: "The time that the downloads last completed successfully.",
	})
	metrics_infrastructureFailure = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_Infrastructure_Failure",
		Help: "Gets set if there was a failure not involved with the external website.",
	})
	metrics_downloadFailed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_Download_Failed",
		Help: "Increments every time a download maxes out our number of retries.",
	})
	metrics_copyError = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "downloader_Copy_Error",
		Help: "Increments every time a download fails to copy to GCS.",
	})
)

func setupPrometheus() {
	http.Handle("/metrics", promhttp.Handler())
	prometheus.MustRegister(metrics_downloadsRetrying)
	prometheus.MustRegister(metrics_lastSuccess)
	prometheus.MustRegister(metrics_infrastructureFailure)
	prometheus.MustRegister(metrics_downloadFailed)
	prometheus.MustRegister(metrics_copyError)
}

func main() {
	setupPrometheus()
	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	for {
		failure := false
		timestamp := time.Now().Format("2006/01/02/15:04:05-")
		for _, url := range urls {
			if err := download(url, 3, timestamp, "maxmind-feed-sandbox"); err != nil {
				failure = true
				fmt.Println(err)
				metrics_downloadFailed.Inc()
			}
		}
		if !failure {
			metrics_lastSuccess.SetToCurrentTime()
		}
		failure = false
		time.Sleep(10 * time.Minute)
	}
}

func download(url string, ret int, timestamp string, bucketName string) error {
	filename := url[strings.LastIndex(url, "/")+1:]
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		metrics_infrastructureFailure.Inc()
		if handleError(url, err, ret, timestamp, bucketName) != nil {
			return err
		}
	}
	bkt := client.Bucket(bucketName)
	obj := bkt.Object(timestamp + filename)
	w := obj.NewWriter(ctx)

	resp, err := http.Get(url)
	if err != nil {
		metrics_downloadsRetrying.Inc()
		if handleError(url, err, ret, timestamp, bucketName) != nil {
			return err
		}
	}

	if _, err = io.Copy(w, resp.Body); err != nil {
		metrics_copyError.Inc()
		return handleError(url, err, ret, timestamp, bucketName)
	}
	w.Close()
	fmt.Println(determineIfFileIsNew(bkt, timestamp+filename, timestamp[:8]))
	return nil
}

func handleError(url string, err error, retries int, timestamp string, bucketName string) error {
	if retries == 0 {
		return err
	}
	time.Sleep(2 * time.Minute)
	return download(url, retries-1, timestamp, bucketName)
}

func determineIfFileIsNew(bkt *storage.BucketHandle, fileName string, searchDir string) bool {
	ctx := context.Background()
	obj := bkt.Object(fileName)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		fmt.Println(err)
		return true
	}
	md5Hash := attrs.MD5
	objects := bkt.Objects(ctx, &storage.Query{"", searchDir, false})
	for otherFile, err := objects.Next(); err != iterator.Done; otherFile, err = objects.Next() {
		if bytes.Equal(otherFile.MD5, md5Hash) && otherFile.Name != fileName {
			return false
		}
	}
	return true
}
