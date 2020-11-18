#!bash
set -uexo pipefail
export DOCKER_HOST=tcp://127.0.0.1:12376
until docker version; do sleep 1; done

docker inspect alpine
if docker pull missing; then echo "this should fail"; false; else echo "exit code propagated"; fi

echo "done"
