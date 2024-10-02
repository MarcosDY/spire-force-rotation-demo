#!/bin/bash

. common.sh

podName=$(${KUBECTL_PATH} get pod -n spire-system -l app=spire-server -o jsonpath="{.items[0].metadata.name}")

old_authority=$(${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority jwt show -output json | jq -r .old.authority_id)

log-info "revoke authority: $old_authority"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server localauthority jwt revoke -authorityID $old_authority

sleep 5

log-info "Bundle in pem format"
${KUBECTL_PATH} exec -n spire-system $podName -- ./bin/spire-server bundle show -format spiffe 
