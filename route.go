package downloader

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
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

func init() {
	http.HandleFunc("/", documentation)
	http.HandleFunc("/grab", grab)
	http.HandleFunc("/cron_push", cronPush)
}

func cronPush(w http.ResponseWriter, r *http.Request) {

}

func grab(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	httpClient := urlfetch.Client(ctx)
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "COULDN'T CONNECT TO GCS")
		return
	}

	bkt := storageClient.Bucket(r.URL.Query().Get("bucket"))
	_, err = bkt.Attrs(ctx)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

	}
	date := time.Now().Format("2006-01-02")
	for _, url := range urls {
		err := download(url, date, bkt, 3, httpClient, w)
		if err != nil {
			handleError(err, url, bkt, w)
		}
	}
	fmt.Fprint(w, "OK "+date)

}
func handleError(err error, url string, bkt *storage.BucketHandle, w http.ResponseWriter) {
	fmt.Fprint(w, err)
}

func download(url string, date string, bkt *storage.BucketHandle, triesLeft int, client *http.Client, w http.ResponseWriter) error {
	resp, err := client.Get(url)
	if err != nil {
		if triesLeft > 0 {
			fmt.Fprint(w, "Retrying!\n")
			return download(url, date, bkt, triesLeft-1, client, w)
		}
		return err
	}
	lastSlash := strings.LastIndex(url, "/")
	if lastSlash < 0 {
		return errors.New("INVALID FILENAME")
	}
	obj := bkt.Object(date + "/" + url[lastSlash:])
	gcsw := obj.NewWriter(context.Background())
	_, err = io.Copy(gcsw, resp.Body)
	return err
}

func documentation(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Fire a get request to /grab with parameters bucket and group to trigger the downloading.")
}
