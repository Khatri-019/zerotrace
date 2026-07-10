#!/usr/bin/env bash
set -e

echo "=== Killing old collector ==="
pkill -f zt-collector 2>/dev/null || true
pkill -f "go run" 2>/dev/null || true
sleep 1

echo "=== Starting new collector with warm-up ==="
mkdir -p /tmp/zt-data
ZEROTRACE_STORAGE_PATH=/tmp/zt-data /tmp/zt-collector-v2 2>&1 &
CPID=$!
sleep 4

echo "=== Collector stats ==="
curl -sf http://localhost:8080/api/stats && echo ""

echo "=== Injecting multi-service traces ==="
/tmp/zt-inject --target localhost:4317 --traces 100 --loops 1
sleep 1

echo "=== Graph after inject ==="
curl -sf http://localhost:8080/api/graph | python3 -c "
import sys, json
g = json.load(sys.stdin)
print(f'nodes={len(g[\"nodes\"])} edges={len(g[\"edges\"])}')
for n in g['nodes'][:5]:
    print(f'  node: {n[\"id\"]} p50={n[\"p50_ms\"]:.1f}ms')
for e in g['edges'][:5]:
    print(f'  edge: {e[\"source\"]} -> {e[\"target\"]} calls={e[\"call_count\"]}')
" 2>/dev/null

echo ""
echo "=== Now restarting to test warm-up persistence ==="
kill $CPID 2>/dev/null
sleep 2

ZEROTRACE_STORAGE_PATH=/tmp/zt-data /tmp/zt-collector-v2 2>&1 &
CPID2=$!
sleep 4

echo "=== Graph after restart (should still have data from warm-up) ==="
curl -sf http://localhost:8080/api/graph | python3 -c "
import sys, json
g = json.load(sys.stdin)
print(f'nodes={len(g[\"nodes\"])} edges={len(g[\"edges\"])}')
" 2>/dev/null || echo "python3 not available, raw:" && curl -sf http://localhost:8080/api/graph | head -3

kill $CPID2 2>/dev/null
echo "ALL DONE"
