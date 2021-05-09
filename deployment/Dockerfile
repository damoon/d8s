###############################################################
FROM moby/buildkit:v0.8.3-rootless AS buildkit

# build-env ###################################################
FROM golang:1.15.7-buster AS build-env

# skopeo
RUN echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
RUN curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_10/Release.key | apt-key add -
RUN apt-get -y update
RUN apt-get -y install skopeo

# buildctl
COPY --from=buildkit /usr/bin/buildctl /usr/local/bin/buildctl

WORKDIR /app

RUN apt-get update
RUN apt-get install -y entr

COPY deployment/entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]

COPY go.mod .
COPY go.sum .
RUN go mod download

ARG SOURCE_BRANCH
ARG SOURCE_COMMIT

ENV CGO_ENABLED=0
ENV GOOS=linux

COPY cmd cmd
COPY pkg pkg
RUN go install -ldflags="-X main.gitRef=${SOURCE_BRANCH} -X main.gitHash=${SOURCE_COMMIT}" -installsuffix cgo ./cmd/wedding

###############################################################
FROM alpine:3.13.1 AS prod

# skopeo
RUN apk add --no-cache -X http://dl-cdn.alpinelinux.org/alpine/edge/community skopeo

# buildctl
COPY --from=buildkit /usr/bin/buildctl /usr/local/bin/buildctl

RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=build-env /go/bin/wedding /usr/local/bin/wedding
ENTRYPOINT [ "wedding", "server" ]
