#!/usr/bin/env bash
# run-ui.sh — Start the ZeroTrace UI dev server
# Usage: bash deploy/run-ui.sh [--api-url http://localhost:8080]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

API_URL="${VITE_API_URL:-http://localhost:8080}"
WS_URL="${VITE_WS_URL:-ws://localhost:8080/ws/traces}"

# Parse args
while [[ $# -gt 0 ]]; do
  case $1 in
    --api-url) API_URL="$2"; shift 2 ;;
    --ws-url)  WS_URL="$2";  shift 2 ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ── Node.js / NVM bootstrap ───────────────────────────────────────────────────
# Load NVM if present (needed in WSL where node isn't on PATH by default)
if [ -f "$HOME/.nvm/nvm.sh" ]; then
  # shellcheck source=/dev/null
  source "$HOME/.nvm/nvm.sh"
elif [ -f "/usr/share/nvm/init-nvm.sh" ]; then
  source "/usr/share/nvm/init-nvm.sh"
fi

# Verify node is available
if ! command -v node &>/dev/null; then
  echo "ERROR: Node.js not found. Install via nvm:"
  echo "  curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash"
  echo "  source ~/.nvm/nvm.sh && nvm install 20"
  exit 1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ZeroTrace UI (dev server)"
echo "  Node   : $(node --version)"
echo "  API    : $API_URL"
echo "  WS     : $WS_URL"
echo "  UI     : http://localhost:5173"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

cd "$PROJECT_ROOT/ui"

# Install deps if node_modules is missing or package.json is newer
if [ ! -d node_modules ] || [ package.json -nt node_modules ]; then
  echo "Installing dependencies..."
  npm install
fi

VITE_API_URL="$API_URL" VITE_WS_URL="$WS_URL" npm run dev
