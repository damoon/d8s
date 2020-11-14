set -uexo pipefail

export DOCKER_HOST=tcp://127.0.0.1:12376

cd tilt

timeout 120 tilt ci --port 0
