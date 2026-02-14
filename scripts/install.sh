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
        print_success "Configuration already exists at: $CONFIG_FILE"
        echo ""
        read -p "Reconfigure? (y/N) " -n 1 -r </dev/tty
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Keeping existing configuration"
            return
        fi
        echo ""
    fi

    # Only ask for Claude data directory - everything else is handled by defaults
    echo ""
    print_info "Where are your Claude conversation files stored?"
    default_claude_dir="${HOME}/.claude/projects"
    echo ""
    read -p "  Claude data directory [$default_claude_dir]: " claude_data_dir </dev/tty
    claude_data_dir=${claude_data_dir:-$default_claude_dir}

    # Validate directory exists
    if [ ! -d "$claude_data_dir" ]; then
        echo ""
        print_warning "Directory does not exist: $claude_data_dir"
        read -p "Create it? (Y/n) " -n 1 -r </dev/tty
        echo
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            mkdir -p "$claude_data_dir"
            print_success "Created directory: $claude_data_dir"
        fi
    fi

    # Get machine ID (automatic)
    machine_id=$(hostname)

    # Write minimal configuration
    # API endpoint and Cognito settings are hardcoded in the CLI defaults
    cat > "$CONFIG_FILE" << EOF
# Claude History Sync Configuration
#
# The CLI connects to: https://claude-history-mcp.devrel.hny.wtf
# Authentication is handled automatically via OAuth

machine_id: "$machine_id"
claude_data_dir: "$claude_data_dir"
exclude_patterns: []
sync_interval_minutes: 5
EOF

    chmod 600 "$CONFIG_FILE"
    echo ""
    print_success "Configuration saved!"
    echo ""
    print_info "Your conversations from: $claude_data_dir"
    print_info "Will be synced as machine: $machine_id"
}

# Check if already authenticated
check_auth() {
    local full_path="${INSTALL_DIR}/${BINARY_NAME}"

    if [ ! -x "$full_path" ]; then
        return 1
    fi

    # Check status, looking for "authenticated" in the output
    if "$full_path" status 2>/dev/null | grep -q "Status: authenticated"; then
        return 0
    else
        return 1
    fi
}

# Authenticate with Cognito
authenticate() {
    print_header "Authentication"

    # Use full path to ensure binary is found
    FULL_PATH="${INSTALL_DIR}/${BINARY_NAME}"

    if [ ! -x "$FULL_PATH" ]; then
        print_error "Binary not found or not executable at: $FULL_PATH"
        return 1
    fi

    # Check if already authenticated
    if check_auth; then
        print_success "Already authenticated!"
        echo ""
        print_info "Current authentication status:"
        "$FULL_PATH" status | grep -A 1 "Auth:"
        echo ""

        read -p "Re-authenticate anyway? (y/N) " -n 1 -r </dev/tty
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Keeping existing authentication"
            return 0
        fi
        echo ""
        print_info "Logging out current session..."
        "$FULL_PATH" logout 2>/dev/null || true
        echo ""
    fi

    print_info "Starting OAuth authentication flow..."
    echo ""
    print_warning "A browser window will open for you to sign in with your credentials."
    print_warning "If the browser doesn't open automatically, copy and paste the URL shown."
    echo ""
    print_info "The CLI will:"
    print_info "  1. Generate an OAuth authorization URL"
    print_info "  2. Start a local callback server on port 3000"
    print_info "  3. Wait for you to complete login in the browser"
    print_info "  4. Exchange the authorization code for access tokens"
    echo ""

    read -p "Press ENTER to continue..." </dev/tty
    echo ""

    print_info "Running: $FULL_PATH login"
    echo ""

    if ! "$FULL_PATH" login; then
        print_error "Authentication failed"
        echo ""
        print_info "Troubleshooting:"
        print_info "  1. Complete the login in the browser within 5 minutes"
        print_info "  2. Make sure port 3000 is not already in use"
        print_info "  3. If your browser didn't open, copy and paste the URL shown above"
        echo ""
        print_warning "Don't worry! The CLI is installed and configured."
        print_info "You can authenticate later by running:"
        print_info "  ${BINARY_NAME} login"
        echo ""
        print_info "After authenticating, you can manually sync with:"
        print_info "  ${BINARY_NAME} sync"
        return 1
    fi

    echo ""
    print_success "Successfully authenticated!"
    echo ""
    print_info "You can now run '${BINARY_NAME} sync' to sync your conversations."
}

# Setup cron job
setup_cron() {
    print_header "Automatic Sync Setup"

    echo ""
    print_success "Authentication successful! Ready for automatic syncing."
    echo ""
    print_info "You can set up automatic syncing to run every 5 minutes."
    print_info "The cron job will call '${BINARY_NAME} sync' which:"
    print_info "  - Scans your Claude conversation directory"
    print_info "  - Uploads new messages to the MCP server"
    print_info "  - Uses your stored authentication tokens"
    echo ""
    print_warning "This will add a cron job to your crontab."
    echo ""

    read -p "Setup automatic sync? (Y/n) " -n 1 -r </dev/tty
    echo

    if [[ $REPLY =~ ^[Nn]$ ]]; then
        print_info "Skipping automatic sync setup"
        echo ""
        print_info "You can manually sync anytime with:"
        print_info "  ${BINARY_NAME} sync"
        return
    fi

    # Add cron job
    CRON_LINE="*/5 * * * * ${INSTALL_DIR}/${BINARY_NAME} sync >> ${CONFIG_DIR}/sync.log 2>&1"

    # Check if cron entry already exists
    if crontab -l 2>/dev/null | grep -q "${BINARY_NAME} sync"; then
        print_warning "Cron job already exists"
    else
        (crontab -l 2>/dev/null; echo "$CRON_LINE") | crontab -
        echo ""
        print_success "Automatic sync configured (every 5 minutes)"
        print_info "Logs will be written to: ${CONFIG_DIR}/sync.log"
        echo ""
        print_info "To view sync logs:"
        print_info "  tail -f ${CONFIG_DIR}/sync.log"
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

    # Check authentication status and authenticate if needed
    # This ensures users are ready to sync immediately
    if authenticate; then
        # Authentication successful or already authenticated
        setup_cron
        show_completion
    else
        # Authentication failed
        echo ""
        print_header "Installation Complete (Auth Pending)"
        echo ""
        print_warning "Installation finished but authentication was not completed."
        print_info "The CLI is installed and configured, but you need to authenticate"
        print_info "before you can sync conversations."
        echo ""
        print_info "To authenticate, run:"
        print_info "  ${BINARY_NAME} login"
        echo ""
        print_info "After authenticating, you can:"
        print_info "  - Manually sync: ${BINARY_NAME} sync"
        print_info "  - Setup cron: Run this installer again or manually add cron job"
        echo ""

        # Still show partial completion info
        echo ""
        print_info "Installation Summary:"
        print_info "  Binary: ${INSTALL_DIR}/${BINARY_NAME}"
        print_info "  Config: $CONFIG_FILE"
        echo ""
    fi
}

# Run main installation
main
