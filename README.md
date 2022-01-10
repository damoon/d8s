# D8s

`D8s` is a command tool to work in conjuction with [tilt](https://github.com/tilt-dev/tilt).

D8s is spoken dates.

## Example usage

`d8s up tilt up`

`d8s -h`

## How it works

1. It deploys a `Docker in Docker` pod into kubernetes
2. It wait for `dind` to start and creates a port-forward
3. It executes the given command and sets `DOCKER_HOST=tcp://127.0.0.1:[random_port]` and `DOCKER_BUILDKIT=1` as environment variables

### Pod

To keep `dind` healthy the pod runs docker and [Nurse](https://github.com/turbine-kreuzberg/dind-nurse).

Nurse does the following things:
- restart docker in case it starts take to much memory away from the actuall builds
- limit the number of parallel requests
- run `docker system prune` when `/var/lib/docker` is used to more then 90%
