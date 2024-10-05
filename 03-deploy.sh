#!/bin/bash

set -eu

. common.sh

load-images() {
    local kind_name=$1; shift
    local container_images=("$@")

    for image in "${container_images[@]}"; do
        ${KIND_PATH} load docker-image --name $kind_name "${image}"
    done
}

# Load builded images
container_images=("spiffe-helper:latest-local" "client-service:latest-local" "api-service:latest-local")
load-images ${CLUSTER_NAME} "${container_images[@]}"

# Deploy SPIRE
log-info "Deploying SPIRE Server"
${KUBECTL_PATH} apply -k ./k8s/core

# ${KUBECTL_PATH} wait --for=condition=established --timeout=60s crd/clusterspiffeids.spire.spiffe.io 
# ${KUBECTL_PATH} apply -f k8s/demo/cluster-spiffe-id.yaml

# Sleeping for now until the CRD validation is created
# sleep 60
# TODO: Wait for CRD to be created
timeout=40
# start_time=$(date +%s)

# while true; do
    # if ${KUBECTL_PATH} get crd clusterspiffeids.spire.spiffe.io > /dev/null 2>&1; then
	# log-info "CRD exists, proceeding..."
	# ${KUBECTL_PATH} wait --for=condition=established --timeout=60s crd/clusterspiffeids.spire.spiffe.io
	# break
    # else
	# echo "Waiting for CRD to be created..."
	# sleep 5
    # fi

    # current_time=$(date +%s)
    # elapsed_time=$((current_time - start_time))

    # if [ "$elapsed_time" -ge "$timeout" ]; then
	# echo "Timed out waiting for CRD to be created."
	# exit 1
    # fi
# done

log-info "Deploy pods"
${KUBECTL_PATH} apply -k ./k8s/demo

