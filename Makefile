.PHONY: all build test clean tidy help release version coverage coverage-html

BINARY_NAME=helm-upgrade-check
BIN_DIR=bin
CMD_DIR=cmd/$(BINARY_NAME)
VERSION=1.0.1

all: tidy test build

tidy:
	go mod tidy

build: tidy
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

test: tidy
	go test -v ./...

coverage: tidy
	go test -v -coverprofile=coverage.out ./...
	@echo "\nCoverage report generated: coverage.out"
	@go tool cover -func=coverage.out | tail -1

coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated: coverage.html"

clean:
	rm -rf $(BIN_DIR)
	rm -rf dist/
	rm -f coverage.out coverage.html
	go clean

help:
	@echo "Available targets:"
	@echo "  all           - run tidy, test, and build"
	@echo "  build         - build the plugin binary"
	@echo "  test          - run all tests"
	@echo "  coverage      - run tests with coverage reporting"
	@echo "  coverage-html - generate HTML coverage report (coverage.html)"
	@echo "  tidy          - download and tidy dependencies"
	@echo "  clean         - remove build artifacts"
	@echo "  version       - show current version"
	@echo "  release       - build releases for all platforms using goreleaser"
	@echo "  help          - show this help message"

version:
	@echo "$(VERSION)"

release: tidy test
	@command -v goreleaser >/dev/null 2>&1 || (echo "Error: goreleaser is not installed. Install from https://goreleaser.com"; exit 1)
	@echo "Building release $(VERSION) with goreleaser..."
	goreleaser release --clean --rm-dist
