#!/usr/bin/env bash
# run-collector.sh — Start the ZeroTrace collector in development mode
# Usage: bash deploy/run-collector.sh [--data-dir /path/to/data]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

DATA_DIR="${ZEROTRACE_STORAGE_PATH:-/tmp/zerotrace-badger}"
GRPC_ADDR="${ZEROTRACE_GRPC_ADDRESS:-0.0.0.0:4317}"
HTTP_ADDR="${ZEROTRACE_HTTP_ADDRESS:-0.0.0.0:8080}"

# Parse args
while [[ $# -gt 0 ]]; do
  case $1 in
    --data-dir) DATA_DIR="$2"; shift 2 ;;
    --grpc)     GRPC_ADDR="$2"; shift 2 ;;
    --http)     HTTP_ADDR="$2"; shift 2 ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

mkdir -p "$DATA_DIR"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ZeroTrace Collector (dev mode)"
echo "  gRPC   : $GRPC_ADDR"
echo "  HTTP   : $HTTP_ADDR"
echo "  Data   : $DATA_DIR"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

cd "$PROJECT_ROOT/collector"
ZEROTRACE_STORAGE_PATH="$DATA_DIR" \
ZEROTRACE_GRPC_ADDRESS="$GRPC_ADDR" \
ZEROTRACE_HTTP_ADDRESS="$HTTP_ADDR" \
  go run .
