#!/bin/bash

set -eu

stern . -n client-ns --template '{{color .ContainerColor .ContainerName}} {{.Message}} {{"\n"}}'
