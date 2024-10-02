#!/bin/bash

set -eu

. common.sh

podName=$(${KUBECTL_PATH} get pod -n spire-system -l app=spire-server -o jsonpath="{.items[0].metadata.name}")

old_authority=$(${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority x509 show -output json | jq -r .old.authority_id)

log-info "Taiting authority: $old_authority"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority x509 taint -authorityID $old_authority
