BINARY_NAME=clouddesktop
BUILD_DIR=bin
DIST_DIR=dist
MODULE=github.com/nbugash-viafoura/clouddesktop

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
           -X $(MODULE)/internal/version.Version=$(VERSION) \
           -X $(MODULE)/internal/version.Commit=$(COMMIT) \
           -X $(MODULE)/internal/version.Date=$(DATE)

PLATFORMS = linux/amd64 darwin/amd64 darwin/arm64

.PHONY: build build-all checksums test test-coverage install uninstall clean lint

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/clouddesktop

build-all:
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(BINARY_NAME); \
		echo "Building $$os/$$arch..."; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$$output ./cmd/clouddesktop && \
		tar -czf $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-$$os-$$arch.tar.gz -C $(DIST_DIR) $$output && \
		rm $(DIST_DIR)/$$output; \
	done

checksums:
	@cd $(DIST_DIR) && shasum -a 256 *.tar.gz > checksums.txt

test:
	go test ./...

# Packages with meaningful unit-testable logic (excludes CLI glue, test infra, and main)
COVER_PKGS=$(MODULE)/internal/config,$(MODULE)/internal/aws

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
	rm -rf $(BUILD_DIR) $(DIST_DIR)

lint:
	golangci-lint run ./...
