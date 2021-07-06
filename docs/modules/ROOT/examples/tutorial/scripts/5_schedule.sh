#!/usr/bin/env bash

source scripts/environment.sh

# Set Minikube context
kubectl config use-context minikube

# Set the schedule
kubectl apply -f ../schedule.yaml

# Watch how the number of snapshots grow
watch restic snapshots
