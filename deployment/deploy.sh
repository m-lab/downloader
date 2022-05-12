#!/bin/bash
set -e
set -x

PROJECT_NAME=${PROJECT_NAME:?Please provide a \$PROJECT_NAME}
CLUSTER_NAME=${CLUSTER_NAME:?Please specify the \$CLUSTER_NAME}
BUCKET_NAME=${BUCKET_NAME:?Please specify the \$BUCKET_NAME where you want files saved}
MAXMIND_LICENSE_KEY=${MAXMIND_LICENSE_KEY:?Please specify the \$MAXMIND_LICENSE_KEY}

./travis/kudo.sh $PROJECT_NAME $CLUSTER_NAME kubectl create \
  secret generic downloader-secret \
    --from-literal=license_key=$MAXMIND_LICENSE_KEY \
    --dry-run -o json | ./travis/kudo.sh $PROJECT_NAME $CLUSTER_NAME kubectl apply -f -

./travis/build_and_push_container.sh \
    gcr.io/${PROJECT_NAME}/downloader:$GIT_COMMIT $PROJECT_NAME

./travis/substitute_values.sh ./deployment/templates/ GITHUB_COMMIT \
    ${GIT_COMMIT} PROJECT_NAME ${PROJECT_NAME} BUCKET_NAME ${BUCKET_NAME}

./travis/kudo.sh $PROJECT_NAME $CLUSTER_NAME kubectl apply \
    -f ./deployment/templates/deploy-downloader.yaml
