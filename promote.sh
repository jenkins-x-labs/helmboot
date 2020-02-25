#!/bin/bash

echo "promoting the new version ${VERSION} to downstream repositories"

jx step create pr go --name github.com/jenkins-x-labs/helmboot --version ${VERSION} --build "make mod" --repo https://github.com/jenkins-x-labs/jx-labs.git
