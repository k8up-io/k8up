#!/usr/bin/env bash

export KUBECONFIG=""
export RESTIC_REPOSITORY=s3:$(minikube service list | grep 9001 | cut -d"|" -f 5 | tr -d "[:space:]")/backups/
export RESTIC_PASSWORD=p@ssw0rd
export AWS_ACCESS_KEY_ID=minio
export AWS_SECRET_ACCESS_KEY=minio123
