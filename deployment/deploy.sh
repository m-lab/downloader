#!/bin/bash
set -e
set -x

GIT_COMMIT=${GIT_COMMIT:?Please provide a \$GIT_COMMIT}
PROJECT_NAME=${PROJECT_NAME:?Please provide a \$PROJECT_NAME}
BUCKET_NAME=${BUCKET_NAME:?Please specify the \$BUCKET_NAME where you want files saved}
MAXMIND_LICENSE_KEY=${MAXMIND_LICENSE_KEY:?Please specify the \$MAXMIND_LICENSE_KEY}

kubectl create \
  secret generic downloader-secret \
    --from-literal=license_key=${MAXMIND_LICENSE_KEY} \
    --dry-run -o json | kubectl apply -f -

./travis/substitute_values.sh ./deployment/templates/ GITHUB_COMMIT \
    ${GIT_COMMIT} PROJECT_NAME ${PROJECT_NAME} BUCKET_NAME ${BUCKET_NAME}

kubectl apply -f ./deployment/templates/deploy-downloader.yaml
