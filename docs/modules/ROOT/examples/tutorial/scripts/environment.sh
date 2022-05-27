#!/usr/bin/env bash

kubectl port-forward svc/minio 9000:9000 &
export MINIO_PORT=$!
export KUBECONFIG=""
export RESTIC_REPOSITORY=s3:http://localhost:9000/backups/
export RESTIC_PASSWORD=p@ssw0rd
export AWS_ACCESS_KEY_ID=minio
export AWS_SECRET_ACCESS_KEY=minio123
