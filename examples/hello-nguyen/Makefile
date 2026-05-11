# hello-nguyen

APP_NAME = hello-nguyen
BUILD_DIR = bin

.PHONY: dev build clean

## Development server with HMR
dev:
	go run ./cmd/dev

## Build production binary
build:
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME).exe ./cmd/server

## Clean build artifacts
clean:
	rm -rf $(BUILD_DIR) .nguyen/cache
