.PHONY: fmt check-fmt test build clean release-dry-run release watch watch-mac dev dev-demo

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

# Test goreleaser configuration (requires goreleaser installed)
release-dry-run:
	goreleaser release --snapshot --clean

# Release target - runs tests, creates and pushes a tag
# Usage: make release VERSION=v0.2.0
release:
ifndef VERSION
	$(error VERSION is required. Usage: make release VERSION=v0.2.0)
endif
	@echo "Creating release $(VERSION)..."
	@$(MAKE) test
	git tag $(VERSION)
	git push origin $(VERSION)
	@echo "Release $(VERSION) pushed. GitHub Actions will handle the rest."

# Hot reload for TUI development
# Terminal 1: make dev (or make dev-demo)
# Terminal 2: make watch (Linux) or make watch-mac (macOS)
# Edit .go files and save - TUI restarts automatically

# Watch for changes, rebuild, and kill running TUI (triggers restart in dev loop)
# Linux version (requires inotify-tools: sudo apt install inotify-tools)
watch:
	@while true; do \
		inotifywait -q -e modify -e attrib $$(find . -name '*.go' -not -path './vendor/*') && \
		$(MAKE) build && killall rafa 2>/dev/null || true; \
	done

# macOS version (requires fswatch: brew install fswatch)
watch-mac:
	@fswatch -o $$(find . -name '*.go' -not -path './vendor/*') | xargs -n1 sh -c '$(MAKE) build && killall rafa 2>/dev/null || true'

# Run TUI in a loop (restarts when killed by watch)
# Close the terminal tab/pane to exit
dev:
	@$(MAKE) build
	@while true; do ./bin/rafa || true; done

# Run demo mode in a loop (restarts when killed by watch)
# Usage: make dev-demo DEMO_ARGS="--speed=marathon"
dev-demo:
	@$(MAKE) build
	@while true; do ./bin/rafa demo $(DEMO_ARGS) || true; done
