#!/bin/bash

echo "promoting the new version ${VERSION} to downstream repositories"

jx step create pr go --name github.com/jenkins-x-labs/helmboot --version ${VERSION} --build "make build" --repo https://github.com/jenkins-x-labs/jxl.git

jx step create pr regex --regex "^(?m)\s+name: helmboot\s+version: \"(.*)\"$"  --version ${VERSION} --files alpha/plugins.yml --repo https://github.com/jenkins-x-labs/jxl.git
