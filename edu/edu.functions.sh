#!/bin/bash

restic() {
	kubectl run "wrestic-$(date +%s)" \
		--rm \
		--attach \
		--restart Never \
		--wait \
		--namespace "default" \
		--image "quay.io/vshn/wrestic:latest" \
		--env "AWS_ACCESS_KEY_ID=myaccesskey" \
		--env "AWS_SECRET_KEY=mysecretkey" \
		--env "RESTIC_PASSWORD=myreposecret" \
		--pod-running-timeout 60s \
		--quiet=true \
		--command -- \
		restic \
		--no-cache \
		-r "s3:http://minio.minio.svc.cluster.local:9000/backup" \
		"${@}"
}

kapply() {
  kustomize build ${1} | k apply -f -
}
kdelete() {
  kustomize build ${1} | k delete -f - --ignore-not-found
}
busybox() {
  name=volume-$(date +%s)
	kubectl run "$name" \
		--rm \
		--attach \
		--restart Never \
		--wait \
		--namespace "default" \
		--image "quay.io/prometheus/busybox:latest" \
		--pod-running-timeout 60s \
		--quiet=true \
    --overrides='{"apiVersion":"v1","spec":{"containers":[{"name":"'$name'","image":"quay.io/prometheus/busybox:latest","volumeMounts":[{"mountPath":"/home/store","name":"pvc"}]}],"volumes":[{"name":"pvc","persistentVolumeClaim":{"claimName":"backup-pvc"}}]}}'
		"${@}"
}
