FROM golang:1.15-alpine AS build-stage

RUN apk add --update make
RUN mkdir -p /go/src/github.com/hypertrace/collector
WORKDIR /go/src/github.com/hypertrace/collector

COPY . /go/src/github.com/hypertrace/collector

ARG GIT_COMMIT
ARG VERSION

RUN make build

FROM alpine
# Following folder conventions described in https://unix.stackexchange.com/a/11552
RUN apk --update add ca-certificates
RUN mkdir /usr/local/bin/hypertrace
WORKDIR /usr/local/bin/hypertrace

COPY --from=build-stage /go/src/github.com/hypertrace/collector/collector .
COPY default-config.yml /etc/opt/hypertrace/config.yml

EXPOSE 9411

ENTRYPOINT ./collector --config /etc/opt/hypertrace/config.yml