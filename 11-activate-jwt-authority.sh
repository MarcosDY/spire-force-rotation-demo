#!/bin/bash

set -eu

. common.sh

podName=$(${KUBECTL_PATH} get pod -n spire-system -l app=spire-server -o jsonpath="{.items[0].metadata.name}")

prepared_authority=$(${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority jwt show -output json | jq -r .prepared.authority_id)
log-info "Prepared authority: $prepared_authority"

log-info "Activating authority"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority jwt activate -authorityID $prepared_authority

