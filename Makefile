.PHONY: build test lint clean run

BINARY_NAME=conflux
BIN_DIR=bin

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/conflux

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf $(BIN_DIR)

run:
	go run ./cmd/conflux -config examples/config.yaml

dev:
	go run ./cmd/conflux -config examples/config.yaml -verbose -dry-run

install-deps:
	go mod tidy
	go mod download