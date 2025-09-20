.PHONY: build test lint clean run install uninstall

BINARY_NAME=conflux
BIN_DIR=bin
INSTALL_DIR=/usr/local/bin

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/conflux

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf $(BIN_DIR)

install: build
	sudo cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"

uninstall:
	sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstalled $(BINARY_NAME) from $(INSTALL_DIR)/$(BINARY_NAME)"

run:
	go run ./cmd/conflux -config examples/config.yaml

dev:
	go run ./cmd/conflux -config examples/config.yaml -verbose -dry-run

install-deps:
	go mod tidy
	go mod download