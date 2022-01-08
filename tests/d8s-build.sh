#!bash
set -uexo pipefail

go run ../cmd/d8s run \
    docker build -t d8s-build-test ./d8s-build -f ./d8s-build/Dockerfile
