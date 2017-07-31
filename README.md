# downloader
The Downloader tool runs on Google Container Engine and will scrape the Maxmind and Routeviews for new data. It will download them from the websites and place them in Google Cloud Storage in the specefied bucket. 

It takes one mandatory arguement: `--bucket=GCS-BUCKET-NAME`

## Travis Deployment
Downloader is designed to be deployed exclusively from Travis-CI. If you need to configure Travis to automatically deploy to GKE, then there are a couple things you need to be sure to configure.

### Service Account
The deployment scripts expect the service accounts needed for uploading the docker image to GCS and deploying the image to GKE to be stored in the protected environment variable `GCLOUD_SERVICE_KEY_XXX`, where XXX is the environment you're deploying to (BOX for sandbox, STG for staging, or PRD for production).

The service account key should be the base64 encoded version of the JSON keyfile for a service account with the appropriate permissions to create/read/write objects in GCS and administer clusters in GKE. You can base64 encode the json key by `cat key.json | base64 -w 0` and putting the output from that command into the protected environment variable, either through the Travis website or through encrypting it in the .travis.yml file.

### Deployment Configuration
The deployment scripts also expect a number of other environment variables to be set:

- PROJECT_NAME_XXX: Determines the name of the project to deploy to.
- CLUSTER_NAME_XXX: Determines the GKE cluster to deploy the image to within the project.
- BUCKET_NAME_XXX:  Determines the bucket within the project for the downloaded files to be placed in.
- CLOUDSDK_COMPUTE_ZONE: Determines the zone that we expect the cluster to be located within.
- DOCKER_IMAGE_NAME: Determines what we want to call the docker image in our image repository.

## Kubernetes Secrets
In order for downloader to be able to connect to GCS, it needs to have a service account with access to GCS. You can use the same service account used for travis deployment, if you wish. But you need to store the key file in a kubernetes secret, named downloader-app-key, so that the deployment config can find and mount it for use by the app. You can set it with: `kubectl create secret generic downloader-app-key --from-file=key.json=/path/to/key.json`

## Cluster Creation
The default cluster size is enough for the downloader, but you need to be sure to give it read/write permissions for GCS when you create the cluster.

However, for prometheus monitoring, you must make an extra node pool, as described in the readme of the prometheus-support repository. 

## Prometheus Monitoring
Most of the work for prometheus monitoring is done in the prometheus-support repository. The only things you need to be aware of is that downloader exports some prometheus metrics on /metrics, the containers will have the label so that prometheus scrapes them, and that if you are creating a new cluster for downloader, you must follow the setup instructions in the prometheus-support repo's readme.
