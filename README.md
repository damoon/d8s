# Wedding

Wedding accepts container image builds mocking the http interface of a docker daemon.\
It schedules tasks as jobs to Kubernetes.\
Images are build using buildkit.\
Images are taged using skopeo.

This enables running Tilt setups in gitlab pipelines without running a docker in docker daemon or exposing a host docker socket.\
Building images remotely allows to work from locations with slow internet upstream (home office).

## Use case 1

Using docker cli to build and push an image.

``` bash
export DOCKER_HOST=tcp://wedding:2376
docker build -t registry/user/image:tag .
```

## Use case 2

Using tilt to set up and test an environment.

``` bash
export DOCKER_HOST=tcp://wedding:2376
tilt ci
```

## Use case 3

Using tilt to set up a development environment without a running local docker.

_Terminal 1_
``` bash
kubectl -n wedding port-forward svc/wedding 2376:2376
```

_Terminal 2_
``` bash
export DOCKER_HOST=tcp://127.0.07:2376
tilt up
```
