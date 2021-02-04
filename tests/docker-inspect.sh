#!bash
set -uexo pipefail
export DOCKER_HOST=tcp://127.0.0.1:12376
until docker version; do sleep 1; done

docker pull alpine
docker inspect alpine

echo "done"
