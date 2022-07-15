#!/usr/bin/env bats

load "lib/utils"
load "lib/detik"
load "lib/k8up"

DETIK_CLIENT_NAME="kubectl"
DETIK_CLIENT_NAMESPACE="k8up-system"
DEBUG_DETIK="true"

@test "Given Operator config, When applying manifests, Then expect running pod" {
	# Remove traces of operator deployments from other tests
	kubectl delete namespace "$DETIK_CLIENT_NAMESPACE" --ignore-not-found
	kubectl create namespace "$DETIK_CLIENT_NAMESPACE" || true

	given_a_running_operator

	try "at most 10 times every 2s to find 1 pod named 'k8up' with '.spec.containers[*].image' being '${E2E_IMAGE}'"
	try "at most 20 times every 2s to find 1 pod named 'k8up' with 'status' being 'running'"
}
