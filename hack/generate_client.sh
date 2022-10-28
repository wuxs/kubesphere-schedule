#!/bin/bash

set -e

GV="$1"

./hack/generate_group.sh all kubesphere.io/schedule/pkg/client kubesphere.io/schedule/api "${GV}" --output-base=./  -h "$PWD/hack/boilerplate.go.txt"
