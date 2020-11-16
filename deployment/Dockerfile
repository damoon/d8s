# build-env ###################################################
FROM golang:1.15.4 AS build-env
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

COPY cmd cmd
COPY pkg pkg
RUN go install -ldflags="-X main.gitRef=${SOURCE_BRANCH} -X main.gitHash=${SOURCE_COMMIT}" ./cmd/wedding

###############################################################
FROM debian:buster

RUN apt-get update && apt-get install -y \
    ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY --from=build-env /go/bin/wedding /usr/local/bin/wedding
ENTRYPOINT [ "wedding", "server" ]
