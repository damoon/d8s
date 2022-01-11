# D8s

`D8s` is a command tool to work in conjuction with [tilt](https://github.com/tilt-dev/tilt).

D8s is spoken dates.

## Example

`d8s up tilt up`

This will
1. deploys a `Docker in Docker` pod into kubernetes
2. creates a port-forward to `dind`
3. execute the given command (`tilt up`) and sets `DOCKER_HOST=tcp://127.0.0.1:[random_port]` and `DOCKER_BUILDKIT=1` as environment variables

### Docker in docker Pod

To keep `dind` healthy the pod runs docker and [Nurse](https://github.com/turbine-kreuzberg/dind-nurse).

Nurse does the following things:
- restart docker in case it starts take to much memory away from the actuall builds
- limit the number of parallel requests
- run `docker system prune` when `/var/lib/docker` is used to more then 90%
