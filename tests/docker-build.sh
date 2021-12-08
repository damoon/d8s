#!bash
set -uexo pipefail
export DOCKER_HOST=tcp://127.0.0.1:12376
export DOCKER_BUILDKIT=0
until docker version; do sleep 1; done

docker build -t wedding-build-test-a -t wedding-build-test-b ./docker -f ./docker/dir/Dockerfile

if docker build ./docker-broken; then echo "this should fail"; false; else echo "exit code propagated"; fi

echo "done"
