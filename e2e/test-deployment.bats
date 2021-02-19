#!/usr/bin/env bats

load "lib/utils"
load "lib/detik"
load "lib/k8up"

DETIK_CLIENT_NAME="kubectl"
DETIK_CLIENT_NAMESPACE="k8up-system"
DEBUG_DETIK="true"

@test "verify the deployment" {
	apply definitions/deployment
	echo "$output"

	try "at most 10 times every 2s to find 1 pod named 'k8up-operator' with '.spec.containers[*].image' being '${E2E_IMAGE}'"
	try "at most 20 times every 2s to find 1 pod named 'k8up-operator' with 'status' being 'running'"
}
