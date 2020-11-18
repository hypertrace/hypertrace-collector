BIN_NAME=xfiles

VERSION ?= dev
GIT_HASH ?=$(shell git rev-parse HEAD)
IMAGE_NAME := "hypertrace/collector"

build:
	go build -ldflags "-w -X main.GitHash=${GIT_HASH} -X main.Version=${VERSION}" ./cmd/collector

run:
	go run cmd/collector/main.go --config ./config.yml

package:
	docker build --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=$(GIT_COMMIT) -t $(IMAGE_NAME):${VERSION} -t $(IMAGE_NAME):latest .
