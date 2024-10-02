#!/bin/bash

set -eu

. common.sh

podName=$(${KUBECTL_PATH} get pod -n spire-system -l app=spire-server -o jsonpath="{.items[0].metadata.name}")

log-info "JWT Authorities"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority jwt prepare 

sleep 5

log-info "Bundle in spiffe format"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server bundle show -format spiffe
