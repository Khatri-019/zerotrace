#!/usr/bin/env bash
set -e

PROJ=/mnt/d/zerotrace
BIN=/tmp/zt-collector
DATADIR=/tmp/badger-smoke

echo "=== Building collector ==="
cd "$PROJ/collector" && go build -o "$BIN" . && echo "OK"

mkdir -p "$DATADIR"

# Override storage path via env var (Viper reads ZEROTRACE_STORAGE_PATH)
echo "=== Starting collector (data=$DATADIR) ==="
ZEROTRACE_STORAGE_PATH="$DATADIR" "$BIN" &
CPID=$!
sleep 3

echo "=== /api/stats ==="
curl -sf http://localhost:8080/api/stats && echo ""

echo "=== /api/services ==="
curl -sf http://localhost:8080/api/services && echo ""

echo "=== /api/graph ==="
curl -sf http://localhost:8080/api/graph && echo ""

echo "=== /api/traces ==="
curl -sf "http://localhost:8080/api/traces?limit=5" && echo ""

kill $CPID 2>/dev/null
echo "SMOKE TEST OK"
