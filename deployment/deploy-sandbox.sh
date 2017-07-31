#!/bin/bash
echo "Starting Script"
set -e
set -x
echo "Building Image"
docker build -t gcr.io/${PROJECT_NAME_BOX}/${DOCKER_IMAGE_NAME}:$TRAVIS_COMMIT .
echo "Grabbing Keys"
echo $GCLOUD_SERVICE_KEY_BOX | base64 --decode -i > ${HOME}/gcloud-service-key.json
cat ${HOME}/gcloud-service-key.json | grep -v private_key
gcloud auth activate-service-account --key-file ${HOME}/gcloud-service-key.json

echo "Setting Project Name"
gcloud --quiet config set project $PROJECT_NAME_BOX
echo "Setting Cluster Name"
gcloud --quiet config set container/cluster $CLUSTER_NAME_BOX
echo "Setting Zone"
gcloud --quiet config set compute/zone ${CLOUDSDK_COMPUTE_ZONE}
echo "Getting Credentials"
gcloud --quiet container clusters get-credentials $CLUSTER_NAME_BOX

echo "Pushing Image"
gcloud docker -- push gcr.io/${PROJECT_NAME_BOX}/${DOCKER_IMAGE_NAME}

echo "Tagging Image"
yes | gcloud beta container images add-tag gcr.io/${PROJECT_NAME_BOX}/${DOCKER_IMAGE_NAME}:$TRAVIS_COMMIT gcr.io/${PROJECT_NAME_BOX}/${DOCKER_IMAGE_NAME}:latest

echo "Viewing kubectl config"
kubectl config view
kubectl config current-context

echo "Generating Deployment Config"
./travis/substitute_values.sh ./deployment/templates/ GITHUB_COMMIT $TRAVIS_COMMIT PROJECT_NAME ${PROJECT_NAME_BOX} BUCKET_NAME ${BUCKET_NAME_BOX}

ls ./deployment/templates/
cat ./deployment/templates/deploy-downloader.yaml

echo "Applying Deployment"

kubectl apply -f ./deployment/templates/deploy-downloader.yaml

#echo "Setting Image"
#kubectl set image deployment/${KUBE_DEPLOYMENT_NAME} ${KUBE_DEPLOYMENT_CONTAINER_NAME}=gcr.io/${PROJECT_NAME_BOX}/${DOCKER_IMAGE_NAME}:$TRAVIS_COMMIT

