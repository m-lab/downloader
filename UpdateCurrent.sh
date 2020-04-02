#!/bin/bash

YEAR=$( date +%Y )

for PROJECT in sandbox staging oti
do
  gsutil ls  gs://downloader-mlab-${PROJECT}/Maxmind/${YEAR}/*/*/*GeoLite2-City.tar.gz | tail -n 1 | while read LATEST; do echo $LATEST; gsutil cp $LATEST gs://downloader-mlab-${PROJECT}/Maxmind/current/GeoLite2-City.tar.gz; done
  gsutil ls gs://downloader-mlab-${PROJECT}/RouteViewIPv4/${YEAR}/*/*.pfx2as.gz | tail -n 1 | while read LATEST; do echo $LATEST; gsutil cp $LATEST gs://downloader-mlab-${PROJECT}/RouteViewIPv4/current/routeview.pfx2as.gz; done
  gsutil ls gs://downloader-mlab-${PROJECT}/RouteViewIPv6/${YEAR}/*/*.pfx2as.gz | tail -n 1 | while read LATEST; do echo $LATEST; gsutil cp $LATEST gs://downloader-mlab-${PROJECT}/RouteViewIPv6/current/routeview.pfx2as.gz; done
done
