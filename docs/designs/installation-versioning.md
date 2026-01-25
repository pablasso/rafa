# Technical Design: Installation, Setup & Versioning

## Overview

Add installation/uninstallation support and proper versioning for Rafa, enabling the first public release (v0.1.1). Users will install via a curl one-liner that downloads pre-built binaries from GitHub releases.

**PRD Reference**: [docs/prds/rafa-core.md](../prds/rafa-core.md) - Installation & Setup section

## Goals

- Users can install Rafa with a single curl command
- Users can uninstall Rafa cleanly
- Proper semantic versioning starting at v0.1.1
- Automated release process via GitHub Actions + goreleaser

## Non-Goals

- `go install` support (users must use the curl script)
- Homebrew tap (future enhancement)
- Auto-upgrade functionality (future enhancement)
- Windows support (future enhancement - Linux/macOS only for v0.1.1)

## Architecture

```
┌─────────────────┐     git tag      ┌──────────────────┐
│   Developer     │ ───────────────► │  GitHub Actions  │
│   pushes tag    │                  │  (release.yml)   │
└─────────────────┘                  └────────┬─────────┘
                                              │
                                              ▼
                                     ┌──────────────────┐
                                     │   goreleaser     │
                                     │  - builds bins   │
                                     │  - creates release│
                                     │  - uploads assets │
                                     └────────┬─────────┘
                                              │
                                              ▼
                                     ┌──────────────────┐
                                     │  GitHub Release  │
                                     │  - rafa_darwin_arm64.tar.gz
                                     │  - rafa_darwin_amd64.tar.gz
                                     │  - rafa_linux_amd64.tar.gz
                                     │  - rafa_linux_arm64.tar.gz
                                     │  - install.sh
                                     └────────┬─────────┘
                                              │
         curl -fsSL .../install.sh | sh      │
┌─────────────────┐                          │
│      User       │ ◄────────────────────────┘
└─────────────────┘
```

## Technical Details

### Version Injection

Version is injected at build time using Go's `-ldflags`:

```go
// internal/version/version.go
package version

// Set via ldflags at build time
var (
    Version   = "dev"     // e.g., "v0.1.1"
    CommitSHA = "unknown" // e.g., "abc1234"
    BuildDate = "unknown" // e.g., "2025-01-25"
)
```

Build command:
```bash
go build -ldflags "-X github.com/pablasso/rafa/internal/version.Version=v0.1.1 \
                   -X github.com/pablasso/rafa/internal/version.CommitSHA=$(git rev-parse --short HEAD) \
                   -X github.com/pablasso/rafa/internal/version.BuildDate=$(date -u +%Y-%m-%d)"
```

### CLI Version Command

Update root command to use injected version:

```go
// internal/cli/root.go
import "github.com/pablasso/rafa/internal/version"

var rootCmd = &cobra.Command{
    Use:     "rafa",
    Short:   "Task loop runner for AI coding agents",
    Version: version.Version,
}

func init() {
    // Custom version template to show more info
    rootCmd.SetVersionTemplate(`Rafa {{.Version}}
Commit: ` + version.CommitSHA + `
Built:  ` + version.BuildDate + "\n")
}
```

### goreleaser Configuration

```yaml
# .goreleaser.yaml
version: 2

before:
  hooks:
    - go mod tidy
    - go fmt ./...

builds:
  - id: rafa
    main: ./cmd/rafa
    binary: rafa
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/pablasso/rafa/internal/version.Version={{.Version}}
      - -X github.com/pablasso/rafa/internal/version.CommitSHA={{.ShortCommit}}
      - -X github.com/pablasso/rafa/internal/version.BuildDate={{.Date}}

archives:
  - id: default
    formats:
      - tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: 'checksums.txt'

release:
  github:
    owner: pablasso
    name: rafa
  extra_files:
    - glob: ./scripts/install.sh

changelog:
  use: github
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
```

### Install Script

```bash
#!/bin/sh
# scripts/install.sh
# Install Rafa - Task loop runner for AI coding agents
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh -s -- -v v0.1.1

set -e

REPO="pablasso/rafa"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION=""

# Parse arguments
while [ $# -gt 0 ]; do
    case "$1" in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    *)
        echo "Error: Unsupported operating system: $OS"
        exit 1
        ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Get latest version if not specified
if [ -z "$VERSION" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi
fi

echo "Installing Rafa ${VERSION} for ${OS}/${ARCH}..."

# Download and extract
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/rafa_${OS}_${ARCH}.tar.gz"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/rafa.tar.gz"
curl -fsSL "$CHECKSUM_URL" -o "${TMP_DIR}/checksums.txt"

# Verify checksum
EXPECTED_CHECKSUM=$(grep "rafa_${OS}_${ARCH}.tar.gz" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED_CHECKSUM" ]; then
    echo "Error: Could not find checksum for rafa_${OS}_${ARCH}.tar.gz"
    exit 1
fi

# Use sha256sum on Linux, shasum on macOS
if command -v sha256sum > /dev/null 2>&1; then
    ACTUAL_CHECKSUM=$(sha256sum "${TMP_DIR}/rafa.tar.gz" | awk '{print $1}')
else
    ACTUAL_CHECKSUM=$(shasum -a 256 "${TMP_DIR}/rafa.tar.gz" | awk '{print $1}')
fi
if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    echo "Error: Checksum verification failed"
    echo "Expected: $EXPECTED_CHECKSUM"
    echo "Actual:   $ACTUAL_CHECKSUM"
    exit 1
fi

tar -xzf "${TMP_DIR}/rafa.tar.gz" -C "$TMP_DIR"

# Install binary
if [ -w "$INSTALL_DIR" ]; then
    mv "${TMP_DIR}/rafa" "${INSTALL_DIR}/rafa"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "${TMP_DIR}/rafa" "${INSTALL_DIR}/rafa"
fi

chmod +x "${INSTALL_DIR}/rafa"

# Verify installation
if "${INSTALL_DIR}/rafa" --version > /dev/null 2>&1; then
    echo "Rafa ${VERSION} installed successfully!"
    echo ""
    echo "Run 'rafa --help' to get started."
else
    echo "Error: Installation failed - binary not working"
    exit 1
fi
```

### GitHub Actions Workflow

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'

      - name: Run tests
        run: make test

      - name: Run goreleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### README Documentation

Add the following sections to README.md:

#### Installation Section

```markdown
## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh
```

To install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh -s -- -v v0.1.1
```

If you prefer to review the script before running:

```bash
curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh -o install.sh
cat install.sh  # review the script
sh install.sh
```

### Uninstall

```bash
rm $(which rafa)
```

To also remove Rafa data from a repository:

```bash
rm -rf .rafa/
```
```

#### Contributing/Release Section

```markdown
## Releasing

To create a new release:

1. Ensure all changes are committed and tests pass (`make test`)
2. Tag the release:
   ```bash
   git tag v0.1.2
   git push origin v0.1.2
   ```
3. GitHub Actions will automatically build and publish the release

The changelog is auto-generated from commit messages.
```

### Makefile Updates

Add build and release targets:

```makefile
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%d)
LDFLAGS := -X github.com/pablasso/rafa/internal/version.Version=$(VERSION) \
           -X github.com/pablasso/rafa/internal/version.CommitSHA=$(COMMIT) \
           -X github.com/pablasso/rafa/internal/version.BuildDate=$(DATE)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o bin/rafa ./cmd/rafa

.PHONY: install
install: build
	cp bin/rafa /usr/local/bin/rafa

.PHONY: release-dry-run
release-dry-run:
	goreleaser release --snapshot --clean
```

## File Changes Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/version/version.go` | Create | Version variables for ldflags injection |
| `internal/cli/root.go` | Modify | Use version package, custom version template |
| `.goreleaser.yaml` | Create | goreleaser configuration |
| `scripts/install.sh` | Create | Installation script |
| `.github/workflows/release.yml` | Create | Release automation |
| `Makefile` | Modify | Add build/install/release targets |
| `README.md` | Modify | Add installation instructions |

## Release Process

1. Ensure all changes are committed and pushed to main
2. Create and push a git tag:
   ```bash
   git tag v0.1.1
   git push origin v0.1.1
   ```
3. GitHub Actions automatically:
   - Runs tests
   - Builds binaries for all platforms
   - Creates GitHub release with binaries and install script
   - Generates changelog from commit messages

The changelog is auto-generated by goreleaser from commit history. Use conventional commit messages for better changelogs (e.g., "feat: add plan run command", "fix: handle empty tasks").

## Testing

- **Local build**: `make build` produces working binary with version
- **Install script**: Test on macOS and Linux (both amd64 and arm64 if available)
- **Checksum verification**: Corrupt a downloaded tarball and verify the script rejects it
- **Release dry-run**: `make release-dry-run` validates goreleaser config
- **Full release**: Test with a pre-release tag (e.g., `v0.1.1-rc1`) before final release

## Edge Cases

| Case | How it's handled |
|------|------------------|
| No sudo access | Script prompts for sudo only if needed |
| Unsupported OS | Clear error message |
| Unsupported arch | Clear error message |
| Network failure | curl fails with error |
| GitHub API rate limit | User can specify version manually with `-v` |
| Binary already exists | Overwrites existing binary |
| Binary doesn't work | Verification step catches and reports error |
| Checksum mismatch | Script fails with expected vs actual checksums |
| Checksums.txt missing | Script fails with clear error |

## Trade-offs

**Why curl script over `go install`?**
- Target users may not have Go installed
- Consistent experience for all users
- Control over binary naming and installation location

**Why goreleaser?**
- Industry standard for Go projects
- Handles cross-compilation, checksums, changelogs
- Integrates seamlessly with GitHub Actions
- Alternatives (manual scripts) are error-prone

**Why no Homebrew?**
- Extra maintenance burden
- curl script covers the use case
- Can add later if there's demand

## Open Questions

None - ready for implementation.
