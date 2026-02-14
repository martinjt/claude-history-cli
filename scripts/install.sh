#!/bin/bash
set -euo pipefail

REPO="martinjt/claude-history-cli"
BINARY_NAME="claude-history-sync"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Detected: ${OS}/${ARCH}"

# Get latest release version
LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "Error: Could not determine latest release"
  exit 1
fi

echo "Latest version: ${LATEST}"
VERSION="${LATEST#v}"

# Download
FILENAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${FILENAME}"

echo "Downloading ${URL}..."
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -sL "$URL" -o "${TMP_DIR}/${FILENAME}"

# Verify checksum
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${LATEST}/checksums.txt"
curl -sL "$CHECKSUMS_URL" -o "${TMP_DIR}/checksums.txt"

cd "$TMP_DIR"
if command -v sha256sum &> /dev/null; then
  grep "$FILENAME" checksums.txt | sha256sum -c - || { echo "Checksum verification failed!"; exit 1; }
elif command -v shasum &> /dev/null; then
  grep "$FILENAME" checksums.txt | shasum -a 256 -c - || { echo "Checksum verification failed!"; exit 1; }
fi

# Extract and install
tar xzf "$FILENAME"
chmod +x "$BINARY_NAME"

if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY_NAME" "${INSTALL_DIR}/${BINARY_NAME}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "$BINARY_NAME" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo "Installed ${BINARY_NAME} ${LATEST} to ${INSTALL_DIR}/${BINARY_NAME}"

# Create config directory
CONFIG_DIR="${HOME}/.claude-history-sync"
mkdir -p "$CONFIG_DIR"
chmod 700 "$CONFIG_DIR"

# Create default config if not exists
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
  MACHINE_ID=$(hostname)
  cat > "$CONFIG_FILE" << EOF
# Claude History Sync Configuration
api_endpoint: "https://api.claude-history.example.com"
machine_id: "${MACHINE_ID}"
claude_data_dir: "${HOME}/.claude/projects"
exclude_patterns: []
sync_interval_minutes: 5

# Cognito authentication settings
cognito_region: "eu-west-1"
cognito_pool_id: ""
cognito_client_id: ""
cognito_domain: ""
EOF
  chmod 600 "$CONFIG_FILE"
  echo "Created config at ${CONFIG_FILE}"
  echo "Please edit the config with your Cognito settings before authenticating."
fi

# Authenticate
echo ""
echo "To authenticate, run:"
echo "  ${BINARY_NAME} login"
echo ""
echo "To setup automatic sync (every 5 minutes), add to crontab:"
echo "  (crontab -l 2>/dev/null; echo '*/5 * * * * ${INSTALL_DIR}/${BINARY_NAME} sync >> ${CONFIG_DIR}/sync.log 2>&1') | crontab -"
