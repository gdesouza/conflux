.PHONY: build test coverage lint clean run install uninstall version-major version-minor version-patch version-show version-help

BINARY_NAME=conflux
BIN_DIR=bin
INSTALL_DIR=/usr/local/bin

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags to inject version information
LDFLAGS=-ldflags "-X conflux/pkg/version.Version=$(VERSION) -X conflux/pkg/version.GitCommit=$(GIT_COMMIT) -X conflux/pkg/version.BuildDate=$(BUILD_DATE)"

build:
	mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/conflux

test:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@echo ""
	@echo "📊 Coverage Summary:"
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "💡 For detailed coverage report, run: go tool cover -html=coverage.out"

coverage:
	@echo "Generating detailed coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"
	@echo "📈 Overall coverage:"
	@go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run

clean:
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

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

# Version management functions
define get_current_version
$(shell git tag --list 'v*' --sort=-version:refname | head -n1 2>/dev/null | sed 's/^v//' || echo "0.0.0")
endef

define increment_version
$(shell echo "$(1)" | awk -F. -v part="$(2)" '
BEGIN {
	major = $$1; minor = $$2; patch = $$3;
	if (part == "major") { major++; minor=0; patch=0 }
	else if (part == "minor") { minor++; patch=0 }
	else if (part == "patch") { patch++ }
	printf "%d.%d.%d", major, minor, patch
}')
endef

# Version targets
version-show:
	@current_version=$$(git tag --list 'v*' --sort=-version:refname | head -n1 2>/dev/null || echo "v0.0.0"); \
	echo "Current version: $$current_version"

version-major:
	@current_version=$$(git tag --list 'v*' --sort=-version:refname | head -n1 2>/dev/null | sed 's/^v//' || echo "0.0.0"); \
	new_version=$$(echo "$$current_version" | awk -F. '{ printf "%d.%d.%d", $$1+1, 0, 0 }'); \
	echo "Bumping version: v$$current_version → v$$new_version (major)"; \
	git tag -a "v$$new_version" -m "Release v$$new_version: Major version bump"; \
	echo "Tagged v$$new_version successfully!"

version-minor:
	@current_version=$$(git tag --list 'v*' --sort=-version:refname | head -n1 2>/dev/null | sed 's/^v//' || echo "0.0.0"); \
	new_version=$$(echo "$$current_version" | awk -F. '{ printf "%d.%d.%d", $$1, $$2+1, 0 }'); \
	echo "Bumping version: v$$current_version → v$$new_version (minor)"; \
	git tag -a "v$$new_version" -m "Release v$$new_version: Minor version bump"; \
	echo "Tagged v$$new_version successfully!"

version-patch:
	@current_version=$$(git tag --list 'v*' --sort=-version:refname | head -n1 2>/dev/null | sed 's/^v//' || echo "0.0.0"); \
	new_version=$$(echo "$$current_version" | awk -F. '{ printf "%d.%d.%d", $$1, $$2, $$3+1 }'); \
	echo "Bumping version: v$$current_version → v$$new_version (patch)"; \
	git tag -a "v$$new_version" -m "Release v$$new_version: Patch version bump"; \
	echo "Tagged v$$new_version successfully!"

version-help:
	@echo "Version Management Commands:"
	@echo "  make version-show    Show current version"
	@echo "  make version-major   Bump major version (1.0.0 → 2.0.0)"
	@echo "  make version-minor   Bump minor version (1.0.0 → 1.1.0)" 
	@echo "  make version-patch   Bump patch version (1.0.0 → 1.0.1)"
	@echo ""
	@echo "Example workflow:"
	@echo "  git add . && git commit -m 'Add new feature'"
	@echo "  make version-minor"
	@echo "  git push origin main --tags"