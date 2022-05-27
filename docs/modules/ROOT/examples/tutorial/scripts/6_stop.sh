#!/usr/bin/env bash

kill $MINIO_PORT
echo $MINIO_PORT
minikube stop
minikube delete
