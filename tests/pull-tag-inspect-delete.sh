#!bash
set -uexo pipefail

docker pull alpine
docker tag alpine some-other-name
docker inspect some-other-name
docker rmi some-other-name

echo "done"
