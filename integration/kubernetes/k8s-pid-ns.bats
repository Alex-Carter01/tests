#!/usr/bin/env bats
#
# Copyright (c) 2018 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
#

load "${BATS_TEST_DIRNAME}/../../.ci/lib.sh"

setup() {
	skip "This is not working (https://github.com/kata-containers/agent/issues/261)"
	export KUBECONFIG="$HOME/.kube/config"
	pod_name="busybox"
	first_container_name="first-test-container"
	second_container_name="second-test-container"

	if kubectl get runtimeclass | grep kata; then
		pod_config_dir="${BATS_TEST_DIRNAME}/runtimeclass_workloads"
	else
		pod_config_dir="${BATS_TEST_DIRNAME}/untrusted_workloads"
	fi
}

@test "Check PID namespaces" {
	skip "This is not working (https://github.com/kata-containers/agent/issues/261)"
	wait_time=60
	sleep_time=5

	# Create the pod
	kubectl create -f "${pod_config_dir}/busybox-pod.yaml"

	# Check pod creation
	pod_status_cmd="kubectl get pods -a | grep $pod_name | grep Running"
	waitForProcess "$wait_time" "$sleep_time" "$pod_status_cmd"

	# Check PID from first container
	first_pid_container=$(kubectl exec $pod_name -c $first_container_name ps | grep "/pause")

	# Check PID from second container
	second_pid_container=$(kubectl exec $pod_name -c $second_container_name ps | grep "/pause")

	[ "$first_pid_container" == "$second_pid_container" ]
}

teardown() {
	skip "This is not working (https://github.com/kata-containers/agent/issues/261)"
	kubectl delete deployment "$pod_name"
	# Wait for the pods to be deleted
	cmd="kubectl get pods | grep found."
	waitForProcess "$wait_time" "$sleep_time" "$cmd"
	kubectl get pods
}
