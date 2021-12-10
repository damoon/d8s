# D8s

`D8s` is a command tool to work in conjuction with [tilt](https://github.com/tilt-dev/tilt) and (optional) [wedding](https://github.com/damoon/wedding).

D8s is spoken dates.

D8s minimizes the data transfered to upload a docker build context.\
It splits the build context into chunks using a rolling checksum.\
Only new chunks are transfered.\
The context is restored via the server component `Dinner`.\
Dinner can use a `Docker in Docker` (dind) or an other compatible backend to forward the build requests.
