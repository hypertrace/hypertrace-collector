FROM golang:1.15-buster as build-stage

RUN mkdir -p /go/src/github.com/hypertrace/collector
WORKDIR /go/src/github.com/hypertrace/collector

COPY . /go/src/github.com/hypertrace/collector

ARG GIT_COMMIT
ARG VERSION

RUN GOOS=linux make build

FROM gcr.io/distroless/base
# Following folder conventions described in https://unix.stackexchange.com/a/11552
WORKDIR /usr/local/bin/hypertrace

COPY --from=build-stage /go/src/github.com/hypertrace/collector/collector .
COPY default-config.yml /etc/opt/hypertrace/config.yml

EXPOSE 9411

ENTRYPOINT ["/usr/local/bin/hypertrace/collector", "--config", "/etc/opt/hypertrace/config.yml"]