BINARY_NAME=clouddesktop
BUILD_DIR=bin
MODULE=github.com/nbugash-viafoura/clouddesktop

.PHONY: build test test-coverage install uninstall clean lint

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/clouddesktop

test:
	go test ./...

# Packages with meaningful unit-testable logic (excludes CLI glue, test infra, and main)
COVER_PKGS=$(MODULE)/internal/config,$(MODULE)/internal/terraform,$(MODULE)/internal/aws

test-coverage:
	go test -coverprofile=coverage.out -coverpkg=$(COVER_PKGS) ./...
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=coverage.out | grep ^total:
	@echo ""
	@echo "Per-function detail: go tool cover -func=coverage.out"
	@echo "HTML report:         go tool cover -html=coverage.out"

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)

uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)

lint:
	golangci-lint run ./...
