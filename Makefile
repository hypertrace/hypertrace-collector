VERSION ?= dev
GIT_HASH ?=$(shell git rev-parse HEAD)
IMAGE_NAME ?= "hypertrace/collector"
CONFIG_FILE ?= ./default-config.yml

.PHONY: unit-test
unit-test:
	go test -count=1 -v -race -cover ./...

.PHONY: test
test: unit-test

.PHONY: build
build:
	$(if $(GOOS),GOOS=${GOOS},) go build -ldflags "-w -X main.GitHash=${GIT_HASH} -X main.Version=${VERSION}" ./cmd/collector

.PHONY: run
run:
	go run -ldflags "-w -X main.GitHash=${GIT_HASH} -X main.Version=${VERSION}" cmd/collector/* --config ${CONFIG_FILE}

.PHONY: package
package:
	@docker build --build-arg VERSION=${VERSION} --build-arg GIT_COMMIT=$(GIT_COMMIT) -t $(IMAGE_NAME):${VERSION} .

.PHONY: docker-push
docker-push:
	docker push ${DOCKER_IMAGE}:${VERSION}

.PHONY: lint
lint:
	@echo "Running linters..."
	@golangci-lint run ./... && echo "Done."
