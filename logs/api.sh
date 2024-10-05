#!/bin/bash

set -eu

stern . -n api-ns --template '{{color .ContainerColor .ContainerName}} {{.Message}} {{"\n"}}'
