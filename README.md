# Wedding

Wedding accepts container image builds mocking the http interface of a docker daemon.
It schedules builds via jobs to Kubernetes.
Images are build using buildkit.

## Example

Using docker cli to build and push an image.

``` bash
export DOCKER_HOST=tcp://wedding:2375
docker build -t registry/user/image:tag .
```
