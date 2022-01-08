#!bash
set -uexo pipefail
export DOCKER_HOST=tcp://127.0.0.1:12375
export DOCKER_BUILDKIT=0
until docker version; do sleep 1; done

docker pull alpine
if docker pull missing; then echo "this should fail"; false; else echo "exit code propagated"; fi

docker tag alpine d8s-registry:5000/test-push:alpine
if docker tag missing b; then echo "this should fail"; false; else echo "exit code propagated"; fi

docker push d8s-registry:5000/test-push:alpine
if docker push missing; then echo "this should fail"; false; else echo "exit code propagated"; fi

echo "done"
