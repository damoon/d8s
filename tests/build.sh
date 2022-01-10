#!bash
set -uexo pipefail

docker build \
    -t build-test-a \
    -t build-test-b \
    -f ./docker/dir/Dockerfile \
    ./docker

if docker build ./docker-broken; then
    echo "this should fail";
    false;
else
    echo "exit code propagated";
fi

echo "done"
