#!/usr/bin/env bash
set -euo pipefail

BINARY_URL="REPLACE_WITH_AGENTCODE_MCP_BINARY_URL"
BINARY_NAME="agentcode-mcp"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/agentcode-mcp}"
CONFIG_PATH="$CONFIG_DIR/config.json"

if [ "$BINARY_URL" = "REPLACE_WITH_AGENTCODE_MCP_BINARY_URL" ]; then
  echo "Please set BINARY_URL in install-agentcode-mcp.sh before running."
  exit 1
fi

mkdir -p "$INSTALL_DIR"

tmp_bin="$(mktemp)"
trap 'rm -f "$tmp_bin"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$BINARY_URL" -o "$tmp_bin"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp_bin" "$BINARY_URL"
else
  echo "Neither curl nor wget is available to download the binary."
  exit 1
fi

chmod +x "$tmp_bin"
mv "$tmp_bin" "$INSTALL_DIR/$BINARY_NAME"

mkdir -p "$CONFIG_DIR"

if [ ! -f "$CONFIG_PATH" ]; then
  cat > "$CONFIG_PATH" <<EOF
{
  "log_level": "info",
  "max_search_results": 50,
  "max_file_bytes": 1048576,
  "build_timeout_seconds": 60,
  "allowed_build_commands": ["go build", "go test", "go vet", "go run"],
  "allowed_paths": [],
  "blocked_extensions": [".env", ".key", ".pem", ".crt", ".cer", ".p12", ".pfx", ".jks", ".keystore"],
  "low_resource_mode": false
}
EOF
fi

echo "Installed $BINARY_NAME to $INSTALL_DIR"
echo "Config file: $CONFIG_PATH"
echo "You can now configure agentcode-mcp in your MCP-enabled client."

