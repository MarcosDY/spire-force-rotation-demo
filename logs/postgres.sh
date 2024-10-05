#!/bin/bash

set -eu

stern . -n postgres-ns --template '{{color .ContainerColor .ContainerName}} {{.Message}} {{"\n"}}'
