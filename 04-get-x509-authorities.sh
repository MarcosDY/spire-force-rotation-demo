#!/bin/bash

set -eu

. common.sh

podName=$(${KUBECTL_PATH} get pod -n spire-system -l app=spire-server -o jsonpath="{.items[0].metadata.name}")

log-info "X509 Authorities"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority x509 show

log-info "Bundle in pem format"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server bundle show
