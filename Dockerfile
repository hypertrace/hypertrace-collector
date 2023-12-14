FROM golang:1.20-bookworm as build-stage

RUN mkdir -p /go/src/github.com/hypertrace/collector
WORKDIR /go/src/github.com/hypertrace/collector

ARG BUILD_COMMIT_SHA
ARG BUILD_VERSION

COPY go.mod go.mod
COPY go.sum go.sum
COPY . .

RUN GOOS=linux make build BUILD_VERSION=${BUILD_VERSION} BUILD_COMMIT_SHA=${BUILD_COMMIT_SHA}

FROM gcr.io/distroless/base
# Following folder conventions described in https://unix.stackexchange.com/a/11552
WORKDIR /usr/local/bin/hypertrace

ARG BUILD_COMMIT_SHA
ARG BUILD_VERSION

LABEL org.opencontainers.image.version=${BUILD_VERSION}
LABEL org.opencontainers.image.revision=${BUILD_COMMIT_SHA}

COPY --from=build-stage /go/src/github.com/hypertrace/collector/collector .
COPY default-config.yml /etc/opt/hypertrace/config.yml

EXPOSE 9411

ENTRYPOINT ["/usr/local/bin/hypertrace/collector"]

CMD ["--config", "/etc/opt/hypertrace/config.yml"]
