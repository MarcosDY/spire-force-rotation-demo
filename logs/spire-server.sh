#!/bin/bash

set -eu

stern . -n spire-system -c spire-server --template '{{color .ContainerColor .ContainerName}} {{.Message}} {{"\n"}}'
