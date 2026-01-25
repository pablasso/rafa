.PHONY: fmt check-fmt test build clean

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build variables
BINARY_NAME = rafa
BUILD_DIR = bin
LDFLAGS = -ldflags "-X github.com/pablasso/rafa/internal/version.Version=$(VERSION) \
                    -X github.com/pablasso/rafa/internal/version.CommitSHA=$(COMMIT_SHA) \
                    -X github.com/pablasso/rafa/internal/version.BuildDate=$(BUILD_DATE)"

# Format all Go files
fmt:
	go fmt ./...

# Check if files are formatted (for CI)
check-fmt:
	@test -z "$$(gofmt -l .)" || (echo "Files not formatted. Run 'make fmt'" && gofmt -l . && exit 1)

# Run all tests
test:
	go test ./...

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/rafa

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
