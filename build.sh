#!/bin/bash

GIT_COMMIT=$(git rev-list -1 HEAD)
VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///')

operator-sdk build algohub/pipeline-operator:$GIT_COMMIT

docker tag algohub/pipeline-operator:$GIT_COMMIT algohub/pipeline-operator:latest

sed -i "s|algohub/pipeline-operator.*|algohub/pipeline-operator:$GIT_COMMIT|g" deploy/operator.yaml
