#!/usr/bin/env bash

# This script triggers a backup job

# Set Minikube context
kubectl config use-context minikube

# Trigger backup
kubectl apply -f backup.yaml
