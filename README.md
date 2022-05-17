[downloader](https://github.com/m-lab/downloader) [![Version](https://img.shields.io/github/tag/m-lab/downloader.svg)](https://github.com/m-lab/downloader/releases) [![Build Status](https://travis-ci.org/m-lab/downloader.svg?branch=master)](https://travis-ci.org/m-lab/downloader) [![Coverage Status](https://coveralls.io/repos/m-lab/downloader/badge.svg?branch=master)](https://coveralls.io/github/m-lab/downloader?branch=master) [![GoDoc](https://godoc.org/github.com/m-lab/downloader?status.svg)](https://godoc.org/github.com/m-lab/downloader) [![Go Report Card](https://goreportcard.com/badge/github.com/m-lab/downloader)](https://goreportcard.com/report/github.com/m-lab/downloader)

# downloader
The Downloader tool runs on Google Container Engine and will scrape the Maxmind
and Routeviews for new data. It will download them from the websites and place
them in Google Cloud Storage in the specefied bucket.

It takes one mandatory arguement: `--bucket=GCS-BUCKET-NAME`

## Travis Deployment
Downloader is designed to be deployed exclusively from Travis-CI. If you need to
configure Travis to automatically deploy to GKE, then there are a couple things
you need to be sure to configure.

### Service Account
The deployment scripts expect the service accounts needed for uploading the
docker image to GCS and deploying the image to GKE to be stored in the protected
environment variable `GCLOUD_SERVICE_KEY_XXX`, where XXX is the environment
you're deploying to (BOX for sandbox, STG for staging, or PRD for production).

The service account key should be the base64 encoded version of the JSON keyfile
for a service account with the appropriate permissions to create/read/write
objects in GCS and administer clusters in GKE. You can base64 encode the json
key by `cat key.json | base64 -w 0` and putting the output from that command
into the protected environment variable, either through the Travis website or
through encrypting it in the .travis.yml file.

The service account also needs the pubsub publisher and pubsub viewer roles.

### Deployment Configuration
In addition to the service account, when setting up a new travis deployment, you
need to configure the project name you're deploying to, the cluster name within
that project, and the bucket name you want the data saved to. Those are all
parameters passed into the deploy.sh command, along with the service account
JSON key, encoded in base64 form. The deploy.sh command takes the form:
``` shell
deploy.sh <project name> <cluster name> <bucket name> \
    <base64 service account key text>
```

## Kubernetes Secrets
In order for downloader to be able to connect to GCS, it needs to have a service
account with access to GCS. You can use the same service account used for travis
deployment, if you wish. But you need to store the key file in a kubernetes
secret, named downloader-app-key, so that the deployment config can find and
mount it for use by the app. You can set it with: 

``` shell
kubectl create secret generic \
    downloader-app-key --from-file=key.json=/path/to/key.json

```

## Cluster Creation
The default cluster size is enough for the downloader, but you need to be sure
to give it read/write permissions for GCS when you create the cluster.

So, we create a dedicated node pool with storage-rw permissions. Ultimately, a
limited permission service account would be preferable. Initially, three nodes
will be allocated, but the autoscaler will shut down two after the downloader
is deployed.

```sh
gcloud --project=mlab-sandbox container node-pools create downloader-pool \
  --cluster=data-processing   --num-nodes=1   --region=us-east1 \
  --scopes storage-rw \
  --node-labels=downloader-node=true --enable-autorepair --enable-autoupgrade \
  --machine-type=n1-standard-2
```

For prometheus monitoring, you must make an extra node pool, as
described in the readme of the prometheus-support repository.

## Pub/Sub Topic
The downloader also expects a pub/sub topic named "downloader-new-files" to
exist. The topic must be created in the project that the downloader is running
in, otherwise the downloader will not start.

## Prometheus Monitoring
Most of the work for prometheus monitoring is done in the prometheus-support
repository. The only things you need to be aware of is that downloader exports
some prometheus metrics on /metrics, the containers will have the label so that
prometheus scrapes them, and that if you are creating a new cluster for
downloader, you must follow the setup instructions in the prometheus-support
repo's readme.

