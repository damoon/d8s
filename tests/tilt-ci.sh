#!bash
set -uexo pipefail
export DOCKER_HOST=tcp://127.0.0.1:12376
export DOCKER_BUILDKIT=0
until docker version; do sleep 1; done

cd tilt
tilt down
tilt ci --port 0
tilt down
