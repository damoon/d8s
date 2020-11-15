#!bash
set -uexo pipefail
export DOCKER_HOST=tcp://127.0.0.1:12376
until docker version; do sleep 1; done

docker pull mirror.gcr.io/library/alpine
if docker pull mirror.gcr.io/library/missing; then echo "this should fail"; false; else echo "exit code propagated"; fi

docker tag mirror.gcr.io/library/alpine wedding-registry:5000/test-push:alpine
if docker tag missing b; then echo "this should fail"; false; else echo "exit code propagated"; fi

docker push wedding-registry:5000/test-push:alpine
if docker push missing; then echo "this should fail"; false; else echo "exit code propagated"; fi

echo "done"
