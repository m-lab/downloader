#!/bin/bash
set -e
set -x

GIT_COMMIT=${GIT_COMMIT:?Please provide a \$GIT_COMMIT}
PROJECT_NAME=${PROJECT_NAME:?Please provide a \$PROJECT_NAME}
BUCKET_NAME=${BUCKET_NAME:?Please specify the \$BUCKET_NAME where you want files saved}
MAXMIND_LICENSE_KEY=${MAXMIND_LICENSE_KEY:?Please specify the \$MAXMIND_LICENSE_KEY}
MAXMIND_ACCOUNT_ID=${MAXMIND_ACCOUNT_ID:?Please specify the \$MAXMIND_ACCOUNT_ID}

kubectl create \
  secret generic downloader-secret \
    --from-literal=license_key=${MAXMIND_LICENSE_KEY} \
    --from-literal=account_id=${MAXMIND_ACCOUNT_ID} \
    --dry-run -o json | kubectl apply -f -

find ./deployment/templates/ -type f -a -print -a \
   -exec sed \
       --expression="s|{{GITHUB_COMMIT}}|${GIT_COMMIT}|" \
       --expression="s|{{PROJECT_NAME}}|${PROJECT_NAME}|" \
       --expression="s|{{BUCKET_NAME}}|${BUCKET_NAME}|" \
       --in-place {} \;

kubectl apply -f ./deployment/templates/deploy-downloader.yaml
