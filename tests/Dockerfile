FROM tiltdev/tilt:v0.17.11

RUN apt update

RUN apt install bats -y

WORKDIR /tests
COPY tests /tests

