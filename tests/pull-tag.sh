#!bash
set -uexo pipefail

docker pull alpine
if docker pull missing; then
    echo "this should fail";
    false;
else
    echo "exit code propagated";
fi

docker tag alpine d8s-registry:5000/test-push:alpine
if docker tag missing b; then
    echo "this should fail";
    false;
else
    echo "exit code propagated";
fi

echo "done"
