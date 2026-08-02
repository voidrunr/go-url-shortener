APP_NAME:= go-url-shortener
APP_MAIN:= ./cmd/shortener/main.go
BUILD_DIR:= ./build

.PHONY: all build run clean test lint vet

all: run

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) $(APP_MAIN)

run:
	@test -f .env && set -a && . ./.env && set +a; go run $(APP_MAIN)

test:
	@test -f .env && set -a && . ./.env && set +a; go test -v ./...

lint:
	go vet ./...

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
