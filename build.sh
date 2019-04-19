#!/bin/bash

GIT_COMMIT=$(git rev-list -1 HEAD)
VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///')

operator-sdk build algohub/endpoint-operator:$GIT_COMMIT

sed -i "s|algohub/endpoint-operator.*|algohub/endpoint-operator:$GIT_COMMIT|g" deploy/operator.yaml
