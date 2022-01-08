#!bash
set -uexo pipefail
export DOCKER_HOST=tcp://127.0.0.1:12375
export DOCKER_BUILDKIT=0
until docker version; do sleep 1; done

docker build -t d8s-build-test-a -t d8s-build-test-b ./docker -f ./docker/dir/Dockerfile

if docker build ./docker-broken; then echo "this should fail"; false; else echo "exit code propagated"; fi

echo "done"
