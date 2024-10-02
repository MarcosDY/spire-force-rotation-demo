#!/bin/bash

set -eu

stern . -n spire-system -c spire-agent
