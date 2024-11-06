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

pull-images() {
    local container_images=("$@")

    for image in "${container_images[@]}"; do
	docker pull "${image}"
    done
}

# Load builded images
container_images=("spiffe-helper:latest-local" "client-service:latest-local" "api-service:latest-local")
load-images ${CLUSTER_NAME} "${container_images[@]}"

# Load SPIRE images, to avoid downloading them from the internet
spire_images=("ghcr.io/spiffe/spire-agent:1.11.0" "ghcr.io/spiffe/spiffe-csi-driver:0.2.3" "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.12.0" "ghcr.io/spiffe/spire-server:1.11.0" "ghcr.io/spiffe/spire-controller-manager:0.6.0")
pull-images "${spire_images[@]}"
load-images ${CLUSTER_NAME} "${spire_images[@]}"

# Deploy SPIRE
log-info "Deploying SPIRE Server"
${KUBECTL_PATH} apply -k ./k8s/core

# Sleeping for now until the CRD validation is created
sleep 30

log-info "Deploy pods"
${KUBECTL_PATH} apply -k ./k8s/demo

