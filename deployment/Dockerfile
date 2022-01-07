# build-env ###################################################
FROM golang:1.17.6-buster AS build-env

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
RUN go install -v \
    -ldflags="-X main.gitRef=${SOURCE_BRANCH} -X main.gitHash=${SOURCE_COMMIT}" \
    -installsuffix cgo \
    ./cmd/dinner

###############################################################
FROM alpine:3.13.5 AS prod

RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=build-env /go/bin/dinner /usr/local/bin/dinner
ENTRYPOINT [ "dinner", "server" ]
