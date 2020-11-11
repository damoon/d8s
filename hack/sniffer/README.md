# Sniffer

Sniffer simplifies the implementation of the [Docker API](https://docs.docker.com/engine/api/v1.40/#operation/ImageBuild).

It is sitting as a HTTP proxy (127.0.07:23765 -> 127.0.0.1:2376) in between dockerd and a docker client.

The endpoint 127.0.0.1:2376 needs to be enabled for dockerd.

It prints requests and responses to stdout.

## Example

_Terminal 1_

``` bash
go run ./hack/sniffer.go
```

_Terminal 2_
``` bash
DOCKER_HOST=tcp://127.0.07:23765 docker build .
```
