#!/usr/bin/env bash

# This script rebuilds the complete minikube cluster in one shot,
# creating a ready-to-use WordPress + MariaDB + Minio environment.

echo ""
echo "••• Launching Minikube •••"
minikube start --memory 4096 --disk-size 60g --cpus 4
kubectl config use-context minikube

echo ""
echo "••• Installing Secrets •••"
kubectl apply -k secrets

echo ""
echo "••• Installing Minio •••"
kubectl apply -k minio

echo ""
echo "••• Installing MariaDB •••"
kubectl apply -k mariadb

echo ""
echo "••• Installing WordPress •••"
kubectl apply -k wordpress

echo ""
echo "••• Installing K8up •••"
helm repo add appuio https://charts.appuio.ch
helm repo update
helm install appuio/k8up --generate-name --set k8up.backupImage.tag=v0.1.8-root

echo ""
echo "••• Watch pods •••"
k9s
