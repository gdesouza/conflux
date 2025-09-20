.PHONY: build test lint clean run

BINARY_NAME=conflux
BUILD_DIR=build

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/conflux

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf $(BUILD_DIR)

run:
	go run ./cmd/conflux -config examples/config.yaml

dev:
	go run ./cmd/conflux -config examples/config.yaml -verbose -dry-run

install-deps:
	go mod tidy
	go mod download