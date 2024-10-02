#!/bin/bash

. common.sh

if ! command -v stern &> /dev/null
then
    go install github.com/stern/stern@latest
fi

stern . -A --template '{{color .ContainerColor .ContainerName}} {{.Message}} {{"\n"}} ' \
	-E kindnet-cni -E spiffe-csi-driver -E spire-controller-manager -E etcd -E kube-controller-manager -E node-driver-registrar -E kube-apiserver

