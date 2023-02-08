BUILD_VERSION ?= dev
BUILD_COMMIT_SHA ?= $(shell git rev-parse HEAD)
IMAGE_NAME ?= "hypertrace/collector"
CONFIG_FILE ?= ./default-config.yml

.PHONY: unit-test
unit-test:
	go test -count=1 -v -race -cover ./...
	cd receiver/jaegerreceiver && go test ./...
	cd exporter/kafkaexporter && go test ./...

.PHONY: test
test: unit-test

.PHONY: build
build:
	$(if $(GOOS),GOOS=${GOOS},) go build -ldflags "-w -X main.BuildCommitSHA=${BUILD_COMMIT_SHA} -X main.BuildVersion=${BUILD_VERSION}" ./cmd/collector

.PHONY: run
run:
	go run -ldflags "-w -X main.BuildCommitSHA=${BUILD_COMMIT_SHA} -X main.BuildVersion=${BUILD_VERSION}" cmd/collector/* --config ${CONFIG_FILE}

.PHONY: package
package:
	@docker build \
	--build-arg BUILD_VERSION=${BUILD_VERSION} \
	--build-arg BUILD_COMMIT_SHA=$(BUILD_COMMIT_SHA) \
	--progress=plain \
	-t $(IMAGE_NAME):${BUILD_VERSION} .


.PHONY: docker-push
docker-push:
	docker push ${DOCKER_IMAGE}:${VERSION}

.PHONY: lint
lint:
	@echo "Running linters..."
	@golangci-lint run ./... && echo "Done."
