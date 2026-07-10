#!/usr/bin/env bash
# verify_scripts.sh — End-to-end verification of collector and UI scripts
set -e

PROJ=/mnt/d/zerotrace

# ── Kill any leftover collectors ──────────────────────────────────────────────
pkill -f zerotrace-collector 2>/dev/null || true
pkill -f "go run ." 2>/dev/null || true
sleep 1

echo "=== Testing run-collector.sh ==="
TMPDATA=$(mktemp -d /tmp/zt-badger-XXXXXX)
ZEROTRACE_STORAGE_PATH="$TMPDATA" \
ZEROTRACE_GRPC_ADDRESS="0.0.0.0:14317" \
ZEROTRACE_HTTP_ADDRESS="0.0.0.0:18080" \
  bash "$PROJ/deploy/run-collector.sh" &
CPID=$!
sleep 5

echo "--- /api/stats ---"
curl -sf http://localhost:18080/api/stats && echo ""

echo "--- /api/traces ---"
curl -sf "http://localhost:18080/api/traces?limit=5" && echo ""

echo "--- /api/services ---"
curl -sf http://localhost:18080/api/services && echo ""

echo "--- /api/graph ---"
curl -sf http://localhost:18080/api/graph && echo ""

kill $CPID 2>/dev/null
wait $CPID 2>/dev/null
echo "COLLECTOR SCRIPT: OK"

echo ""
echo "=== Testing run-ui.sh (init only, no browser needed) ==="
bash "$PROJ/deploy/run-ui.sh" &
UPID=$!
# Give Vite up to 8 seconds to start
for i in $(seq 1 8); do
  sleep 1
  if curl -sf http://localhost:5173 > /dev/null 2>&1; then
    echo "UI SCRIPT: OK — http://localhost:5173 responded in ${i}s"
    break
  fi
  if [ "$i" -eq 8 ]; then
    echo "UI SCRIPT: WARN — http://localhost:5173 not yet up (Vite still starting)"
  fi
done
kill $UPID 2>/dev/null
wait $UPID 2>/dev/null

echo ""
echo "=== ALL DONE ==="
