#!/bin/bash
set -e
set -x

USAGE="$0 <project name> <cluster name> <bucket name>"
PROJECT_NAME=${1:?Please provide a project name: $USAGE}
CLUSTER_NAME=${2:?Please specify the name of the cluster: $USAGE}
BUCKET_NAME=${3:?Please specify the name of the bucket where you want files saved: $USAGE}
GCLOUD_SERVICE_KEY=${4:?Please enter the base64 encoded json keyfile: $USAGE}

echo $GCLOUD_SERVICE_KEY | base64 --decode -i > ${HOME}/gcloud-service-key.json
gcloud auth activate-service-account --key-file ${HOME}/gcloud-service-key.json

./travis/build_and_push_container.sh gcr.io/${PROJECT_NAME}/downloader:$TRAVIS_COMMIT PROJECT_NAME

./travis/substitute_values.sh ./deployment/templates/ GITHUB_COMMIT $TRAVIS_COMMIT PROJECT_NAME ${PROJECT_NAME} BUCKET_NAME ${BUCKET_NAME}


./kubectl.sh $PROJECT_NAME $CLUSTER_NAME kubectl apply -f ./deployment/templates/deploy-downloader.yaml
