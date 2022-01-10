#!bash
set -uexo pipefail

go run ../cmd/d8s run \
    docker pull alpine

go run ../cmd/d8s run \
    docker inspect alpine

echo "done"
