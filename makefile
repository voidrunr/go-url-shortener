APP_NAME:= go-url-shortener
APP_MAIN:= ./cmd/shortener/main.go
BUILD_DIR:= ./build
BUF_VERSION:= v1.75.0

.PHONY: all build run clean test lint vet staticlint cover proto

all: run

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) $(APP_MAIN)

run:
	@test -f .env && set -a && . ./.env && set +a; go run $(APP_MAIN)

test:
	@test -f .env && set -a && . ./.env && set +a; go test -v ./...

lint:
	go vet ./...

staticlint:
	go run ./cmd/staticlint ./...

cover:
	@test -f .env && set -a && . ./.env && set +a; go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1
	rm -f coverage.out

proto:
	go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION) lint
	go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION) generate

format:
	gofmt -w .

clean:
	rm -rf $(BUILD_DIR)

MOCKERY := $(shell go env GOPATH)/bin/mockery

mock:
	$(MOCKERY) --config .mockery.yaml

help:
	@echo "Usage: make <target>"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
