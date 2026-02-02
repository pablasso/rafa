#!/bin/sh
# scripts/install.sh
# Install Rafa - Task loop runner for AI coding agents
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh -s -- -v v0.1.1

set -e

REPO="pablasso/rafa"
VERSION=""

# Colors for output (disabled if not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    NC=''
fi

info() {
    printf "${GREEN}[info]${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}[warn]${NC} %s\n" "$1"
}

error() {
    printf "${RED}[error]${NC} %s\n" "$1" >&2
    exit 1
}

# Parse arguments
while [ $# -gt 0 ]; do
    case "$1" in
        -v|--version)
            if [ -z "$2" ]; then
                error "Missing version argument for $1"
            fi
            VERSION="$2"
            shift 2
            ;;
        -h|--help)
            echo "Install Rafa - Task loop runner for AI coding agents"
            echo ""
            echo "Usage:"
            echo "  curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh"
            echo "  curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh -s -- -v v0.1.1"
            echo ""
            echo "Options:"
            echo "  -v, --version VERSION  Install a specific version (default: latest)"
            echo "  -h, --help            Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  INSTALL_DIR           Installation directory (default: ~/.local/bin)"
            exit 0
            ;;
        *)
            error "Unknown option: $1. Use --help for usage."
            ;;
    esac
done

# Detect OS
detect_os() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$OS" in
        darwin)
            OS="darwin"
            ;;
        linux)
            OS="linux"
            ;;
        *)
            error "Unsupported operating system: $OS. Rafa supports darwin (macOS) and linux."
            ;;
    esac
    echo "$OS"
}

# Detect architecture
detect_arch() {
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64|amd64)
            ARCH="x64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH. Rafa supports x64 (x86_64) and arm64."
            ;;
    esac
    echo "$ARCH"
}

# Get latest version from GitHub API
get_latest_version() {
    LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$LATEST" ]; then
        error "Could not determine latest version. Please specify a version with -v or check your network connection."
    fi
    echo "$LATEST"
}

# Detect the user's shell and return the appropriate config file
detect_shell_config() {
    SHELL_NAME="$(basename "$SHELL")"
    case "$SHELL_NAME" in
        bash)
            if [ -f "$HOME/.bash_profile" ]; then
                echo "$HOME/.bash_profile"
            elif [ -f "$HOME/.bashrc" ]; then
                echo "$HOME/.bashrc"
            else
                echo "$HOME/.profile"
            fi
            ;;
        zsh)
            echo "$HOME/.zshrc"
            ;;
        fish)
            FISH_CONFIG_DIR="$HOME/.config/fish"
            if [ ! -d "$FISH_CONFIG_DIR" ]; then
                mkdir -p "$FISH_CONFIG_DIR"
            fi
            echo "$FISH_CONFIG_DIR/config.fish"
            ;;
        *)
            echo "$HOME/.profile"
            ;;
    esac
}

# Check if a directory is in PATH
is_in_path() {
    case ":$PATH:" in
        *":$1:"*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Add directory to PATH in shell config
add_to_path() {
    DIR="$1"
    CONFIG_FILE=$(detect_shell_config)
    SHELL_NAME="$(basename "$SHELL")"

    info "Adding $DIR to PATH in $CONFIG_FILE"

    # Create config file if it doesn't exist
    if [ ! -f "$CONFIG_FILE" ]; then
        touch "$CONFIG_FILE"
    fi

    # Add appropriate export based on shell
    case "$SHELL_NAME" in
        fish)
            echo "" >> "$CONFIG_FILE"
            echo "# Added by Rafa installer" >> "$CONFIG_FILE"
            echo "set -gx PATH \"$DIR\" \$PATH" >> "$CONFIG_FILE"
            ;;
        *)
            echo "" >> "$CONFIG_FILE"
            echo "# Added by Rafa installer" >> "$CONFIG_FILE"
            echo "export PATH=\"$DIR:\$PATH\"" >> "$CONFIG_FILE"
            ;;
    esac

    warn "PATH updated. Run 'source $CONFIG_FILE' or start a new terminal session."
}

# Verify checksum
verify_checksum() {
    BINARY_FILE="$1"
    CHECKSUMS_FILE="$2"
    BINARY_NAME="$3"

    # Use word boundary matching to avoid partial matches (e.g., rafa-linux-arm64 matching rafa-linux-arm64-v2)
    EXPECTED_CHECKSUM=$(grep -w "$BINARY_NAME" "$CHECKSUMS_FILE" | awk '{print $1}')
    if [ -z "$EXPECTED_CHECKSUM" ]; then
        error "Could not find checksum for $BINARY_NAME in checksums.txt"
    fi

    # Use sha256sum on Linux, shasum on macOS
    if command -v sha256sum > /dev/null 2>&1; then
        ACTUAL_CHECKSUM=$(sha256sum "$BINARY_FILE" | awk '{print $1}')
    elif command -v shasum > /dev/null 2>&1; then
        ACTUAL_CHECKSUM=$(shasum -a 256 "$BINARY_FILE" | awk '{print $1}')
    else
        error "Neither sha256sum nor shasum found. Cannot verify checksum."
    fi

    if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
        error "Checksum verification failed!\nExpected: $EXPECTED_CHECKSUM\nActual:   $ACTUAL_CHECKSUM\nThe downloaded file may be corrupted. Please try again."
    fi

    info "Checksum verified successfully"
}

# Main installation logic
main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)

    if [ -z "$VERSION" ]; then
        info "Fetching latest version..."
        VERSION=$(get_latest_version)
    fi

    BINARY_NAME="rafa-${OS}-${ARCH}"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

    # Determine install directory
    # Default to ~/.local/bin, can be overridden via INSTALL_DIR env var
    INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

    info "Installing Rafa ${VERSION} for ${OS}/${ARCH}..."

    # Create temp directory for download
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    # Download binary
    info "Downloading ${BINARY_NAME}..."
    if ! curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${BINARY_NAME}" 2>/dev/null; then
        error "Failed to download ${BINARY_NAME} from ${DOWNLOAD_URL}\nPlease check the version exists and your network connection."
    fi

    # Download checksums
    info "Downloading checksums..."
    if ! curl -fsSL "$CHECKSUM_URL" -o "${TMP_DIR}/checksums.txt" 2>/dev/null; then
        error "Failed to download checksums from ${CHECKSUM_URL}"
    fi

    # Verify checksum
    verify_checksum "${TMP_DIR}/${BINARY_NAME}" "${TMP_DIR}/checksums.txt" "$BINARY_NAME"

    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        info "Creating directory $INSTALL_DIR..."
        mkdir -p "$INSTALL_DIR" || error "Failed to create directory $INSTALL_DIR"
    fi

    # Install binary
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/rafa"
        chmod +x "${INSTALL_DIR}/rafa"
    else
        info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/rafa" || error "Failed to install binary. Do you have sudo access?"
        sudo chmod +x "${INSTALL_DIR}/rafa"
    fi

    # Add to PATH if needed
    if ! is_in_path "$INSTALL_DIR"; then
        add_to_path "$INSTALL_DIR"
    fi

    # Verify installation by running the binary
    if "${INSTALL_DIR}/rafa" --version > /dev/null 2>&1; then
        info "Rafa ${VERSION} installed successfully to ${INSTALL_DIR}/rafa!"
        echo ""

        if is_in_path "$INSTALL_DIR"; then
            echo "Run 'rafa --help' to get started."
        else
            echo "Run '${INSTALL_DIR}/rafa --help' to get started."
            echo "Or start a new terminal session to use 'rafa' directly."
        fi
    else
        error "Installation failed - binary not working at ${INSTALL_DIR}/rafa"
    fi
}

main
