#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

REPO="martinjt/claude-history-cli"
BINARY_NAME="claude-history-sync"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="${HOME}/.claude-history-sync"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"

# Print colored message
print_info() {
    echo -e "${BLUE}ℹ${NC}  $1"
}

print_success() {
    echo -e "${GREEN}✓${NC}  $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC}  $1"
}

print_error() {
    echo -e "${RED}✗${NC}  $1"
}

print_header() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
    echo ""
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    print_info "Detected platform: ${OS}/${ARCH}"
}

# Download and install binary
install_binary() {
    print_header "Installing Claude History Sync CLI"

    # Get latest release version
    print_info "Fetching latest release..."
    LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$LATEST" ]; then
        print_error "Could not determine latest release"
        print_warning "Please check https://github.com/${REPO}/releases"
        exit 1
    fi

    VERSION="${LATEST#v}"
    print_success "Latest version: ${LATEST}"

    # Download
    FILENAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${LATEST}/${FILENAME}"

    print_info "Downloading from: ${URL}"
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    if ! curl -fsSL "$URL" -o "${TMP_DIR}/${FILENAME}"; then
        print_error "Download failed"
        print_warning "URL: $URL"
        exit 1
    fi

    # Verify checksum
    print_info "Verifying checksum..."
    CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${LATEST}/checksums.txt"
    curl -fsSL "$CHECKSUMS_URL" -o "${TMP_DIR}/checksums.txt"

    cd "$TMP_DIR"
    if command -v sha256sum &> /dev/null; then
        if ! grep "$FILENAME" checksums.txt | sha256sum -c - > /dev/null 2>&1; then
            print_error "Checksum verification failed!"
            exit 1
        fi
    elif command -v shasum &> /dev/null; then
        if ! grep "$FILENAME" checksums.txt | shasum -a 256 -c - > /dev/null 2>&1; then
            print_error "Checksum verification failed!"
            exit 1
        fi
    else
        print_warning "No checksum tool found (sha256sum/shasum), skipping verification"
    fi
    print_success "Checksum verified"

    # Extract and install
    print_info "Extracting binary..."
    tar xzf "$FILENAME"
    chmod +x "$BINARY_NAME"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY_NAME" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        print_info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "$BINARY_NAME" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    print_success "Installed ${BINARY_NAME} ${LATEST} to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Interactive configuration
configure_cli() {
    print_header "Configuration Setup"

    # Create config directory
    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"

    if [ -f "$CONFIG_FILE" ]; then
        print_warning "Configuration file already exists at: $CONFIG_FILE"
        read -p "Overwrite existing configuration? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Keeping existing configuration"
            return
        fi
    fi

    # Get configuration values
    echo ""
    print_info "Please provide the following configuration:"
    echo ""

    # API Endpoint
    read -p "  API Endpoint [https://claude-history-mcp.devrel.hny.wtf]: " api_endpoint
    api_endpoint=${api_endpoint:-https://claude-history-mcp.devrel.hny.wtf}

    # Machine ID (default to hostname)
    machine_id=$(hostname)
    read -p "  Machine ID [$machine_id]: " custom_machine_id
    machine_id=${custom_machine_id:-$machine_id}

    # Claude data directory
    default_claude_dir="${HOME}/.claude/projects"
    read -p "  Claude data directory [$default_claude_dir]: " claude_data_dir
    claude_data_dir=${claude_data_dir:-$default_claude_dir}

    # Cognito settings
    echo ""
    print_info "Cognito authentication settings:"
    read -p "  Cognito Region [eu-west-1]: " cognito_region
    cognito_region=${cognito_region:-eu-west-1}

    read -p "  Cognito User Pool ID [eu-west-1_CmpHruSh7]: " cognito_pool_id
    cognito_pool_id=${cognito_pool_id:-eu-west-1_CmpHruSh7}

    read -p "  Cognito Client ID [79c7ftkao9ae7drb9qrij9q7tc]: " cognito_client_id
    cognito_client_id=${cognito_client_id:-79c7ftkao9ae7drb9qrij9q7tc}

    read -p "  Cognito Domain [claude-history-prod.auth.eu-west-1.amazoncognito.com]: " cognito_domain
    cognito_domain=${cognito_domain:-claude-history-prod.auth.eu-west-1.amazoncognito.com}

    # Write configuration
    cat > "$CONFIG_FILE" << EOF
# Claude History Sync Configuration
api_endpoint: "$api_endpoint"
machine_id: "$machine_id"
claude_data_dir: "$claude_data_dir"
exclude_patterns: []
sync_interval_minutes: 5

# Cognito authentication settings
cognito_region: "$cognito_region"
cognito_pool_id: "$cognito_pool_id"
cognito_client_id: "$cognito_client_id"
cognito_domain: "$cognito_domain"
EOF

    chmod 600 "$CONFIG_FILE"
    print_success "Configuration saved to: $CONFIG_FILE"
}

# Authenticate with Cognito
authenticate() {
    print_header "Authentication"

    print_info "Starting OAuth authentication flow..."
    echo ""
    print_warning "A browser window will open for you to sign in with your credentials."
    print_warning "If the browser doesn't open automatically, copy and paste the URL shown."
    echo ""

    read -p "Press ENTER to continue..."
    echo ""

    if ! ${BINARY_NAME} login; then
        print_error "Authentication failed"
        echo ""
        print_info "Troubleshooting:"
        print_info "  1. Check your Cognito credentials"
        print_info "  2. Verify the Cognito configuration in: $CONFIG_FILE"
        print_info "  3. Ensure you have an account in the Cognito User Pool"
        echo ""
        print_info "You can retry authentication later with:"
        print_info "  ${BINARY_NAME} login"
        return 1
    fi

    print_success "Successfully authenticated!"
}

# Setup cron job
setup_cron() {
    print_header "Automatic Sync Setup"

    echo ""
    print_info "You can set up automatic syncing to run every 5 minutes."
    print_warning "This will add a cron job to your crontab."
    echo ""

    read -p "Setup automatic sync? (Y/n) " -n 1 -r
    echo

    if [[ $REPLY =~ ^[Nn]$ ]]; then
        print_info "Skipping automatic sync setup"
        print_info "You can manually sync with: ${BINARY_NAME} sync"
        return
    fi

    # Add cron job
    CRON_LINE="*/5 * * * * ${INSTALL_DIR}/${BINARY_NAME} sync >> ${CONFIG_DIR}/sync.log 2>&1"

    # Check if cron entry already exists
    if crontab -l 2>/dev/null | grep -q "${BINARY_NAME} sync"; then
        print_warning "Cron job already exists"
    else
        (crontab -l 2>/dev/null; echo "$CRON_LINE") | crontab -
        print_success "Automatic sync configured (every 5 minutes)"
        print_info "Logs will be written to: ${CONFIG_DIR}/sync.log"
    fi
}

# Show completion message
show_completion() {
    print_header "Installation Complete!"

    echo ""
    print_success "Claude History Sync is ready to use!"
    echo ""
    print_info "Quick reference:"
    echo "  ${BINARY_NAME} status     - Check configuration and auth status"
    echo "  ${BINARY_NAME} sync       - Manually sync conversations"
    echo "  ${BINARY_NAME} login      - Re-authenticate if needed"
    echo "  ${BINARY_NAME} logout     - Clear stored credentials"
    echo "  ${BINARY_NAME} help       - Show all commands"
    echo ""
    print_info "Configuration: $CONFIG_FILE"
    print_info "Logs: ${CONFIG_DIR}/sync.log"
    echo ""

    if crontab -l 2>/dev/null | grep -q "${BINARY_NAME} sync"; then
        print_success "Automatic sync is enabled (every 5 minutes)"
    else
        print_info "Manual sync: Run '${BINARY_NAME} sync' when needed"
    fi
    echo ""
}

# Main installation flow
main() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                                                   ║${NC}"
    echo -e "${GREEN}║       Claude History Sync - Installation         ║${NC}"
    echo -e "${GREEN}║                                                   ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════╝${NC}"
    echo ""

    detect_platform
    install_binary
    configure_cli

    if authenticate; then
        setup_cron
    else
        print_warning "Skipping cron setup due to authentication failure"
        print_info "Run '${BINARY_NAME} login' after fixing authentication issues"
    fi

    show_completion
}

# Run main installation
main
