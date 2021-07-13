#!/usr/bin/env bash

# This script restores the contents of a backup to its rightful PVCs.
# After the pods that perform the restore operation are "Completed",
# execute the '5_restore_files.sh' script,
# and after that the '6_delete_restore_pods.sh' script.

source scripts/environment.sh

# Set Minikube context
kubectl config use-context minikube

# Restore WordPress PVC
SNAPSHOT_ID=$(restic snapshots --json --last --path /data/wordpress-pvc | jq -r '.[0].id')
scripts/customize.py wordpress "${SNAPSHOT_ID}" | kubectl apply -f -

# Read SQL data from Restic into file
SNAPSHOT_ID=$(restic snapshots --json --last --path /default-mariadb | jq -r '.[0].id')

# Restore MariaDB data
MARIADB_POD=$(kubectl get pods -o custom-columns="NAME:.metadata.name" --no-headers -l "app=wordpress,tier=mariadb")
restic dump "${SNAPSHOT_ID}" /default-mariadb | kubectl exec -i "$MARIADB_POD" -- /bin/bash -c "mysql -uroot --password=\"${MARIADB_ROOT_PASSWORD}\""
