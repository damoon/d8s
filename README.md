# Wedding

Wedding accepts container image builds mocking the http interface of a docker daemon.\
It schedules builds via jobs to Kubernetes.\
Images are build using buildkit.

This enables Tilt setups using gitlab ci without running a docker in docker daemon or exposing a host docker socket.\
This avoid to maintain a tilt configuration with (ci) and without (local dev) custom_build.

## Use case 1

Using docker cli to build and push an image.

``` bash
export DOCKER_HOST=tcp://wedding:2375
docker build -t registry/user/image:tag .
```

## Use case 2

Using tilt to set up and test an environment.

``` bash
export DOCKER_HOST=tcp://wedding:2375
tilt ci
```

## Use case 3

Using tilt to set up a development environment without a running local docker.

_Terminal 1_
``` bash
kubectl -n wedding port-forward svc/wedding 2375:2375
```

_Terminal 2_
``` bash
export DOCKER_HOST=tcp://127.0.07:2375
tilt up
```
