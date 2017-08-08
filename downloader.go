package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
	"github.com/m-lab/downloader/download"
	"github.com/m-lab/downloader/file"
	"github.com/m-lab/downloader/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// The average time (in hours) to wait in between attempts to download
// files
const averageHoursBetweenUpdateChecks = 8 * time.Hour

// The window of time (in hours) to allow a random time to be chosen
// from.
const windowForRandomTimeBetweenUpdateChecks = 4 * time.Hour

// The main function seeds the random number generator, starts
// prometheus in the background, takes the bucket flag from the
// command line, and kicks off the actual downloader loop
func main() {
	bucketName := flag.String("bucket", "", "Specify the bucket name to store the results in.")
	flag.Parse()
	if *bucketName == "" {
		log.Fatal("NO BUCKET SPECIFIED!!!")
	}
	rand.Seed(time.Now().UTC().UnixNano())
	metrics.SetupPrometheus()
	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	loopOverURLsForever(*bucketName)
}

// loopOverURLsForever takes a bucketName, pointing to a GCS bucket,
// and then tries to download the files over and over again until the
// end of time (waiting an average of 8 hours in between attempts)
func loopOverURLsForever(bucketName string) {
	lastDownloadedV4 := 0
	lastDownloadedV6 := 0
	for {
		timestamp := time.Now().Format("2006/01/02/")
		bkt, err := constructBucketHandle(bucketName)
		if err != nil {
			continue
		}
		fileStore := &file.StoreGCS{Bkt: bkt}

		maxmindErr := download.DownloadMaxmindFiles(download.MaxmindURLs, timestamp, fileStore)
		if maxmindErr != nil {
			log.Println(maxmindErr)
		}

		routeviewIPv4Err := download.DownloadCaidaRouteviewsFiles(
			"http://data.caida.org/datasets/routing/routeviews-prefix2as/pfx2as-creation.log",
			"RouteViewIPv4/",
			&lastDownloadedV4,
			fileStore)
		if routeviewIPv4Err != nil {
			log.Println(routeviewIPv4Err)
		}

		routeviewIPv6Err := download.DownloadCaidaRouteviewsFiles(
			"http://data.caida.org/datasets/routing/routeviews6-prefix2as/pfx2as-creation.log",
			"RouteViewIPv6/",
			&lastDownloadedV6,
			fileStore)
		if routeviewIPv6Err != nil {
			log.Println(routeviewIPv6Err)
		}

		if maxmindErr == nil && routeviewIPv4Err == nil && routeviewIPv6Err == nil {
			metrics.LastSuccessTime.SetToCurrentTime()
		}
		time.Sleep(download.GenUniformSleepTime(averageHoursBetweenUpdateChecks, windowForRandomTimeBetweenUpdateChecks))
	}
}

// constructBucketHandle takes a bucket name and safely loads it,
// returning either the handle to the bucket or an error
func constructBucketHandle(bucketName string) (*storage.BucketHandle, error) {
	ctx, _ := context.WithTimeout(context.Background(), 2*time.Minute)
	client, err := storage.NewClient(ctx)
	if err != nil {
		metrics.DownloaderErrorCount.With(prometheus.Labels{"source": "Client Setup"}).Inc()
		return nil, err
	}
	return client.Bucket(bucketName), nil
}
