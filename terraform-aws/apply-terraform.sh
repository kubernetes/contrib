#!/bin/sh
source aws-variables.sh && \
./master/make-cloud-config.py && \
terraform apply -var "access_key=$AWS_ACCESS_KEY_ID" -var "secret_key=$AWS_SECRET_ACCESS_KEY" \
-var "region=$AWS_REGION"
