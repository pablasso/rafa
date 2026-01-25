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
