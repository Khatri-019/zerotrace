# ZeroTrace

ZeroTrace is a distributed tracing system that instruments any running application at the Linux kernel level using eBPF probes — with zero code changes, zero library imports, and zero restarts of the target application.

## Architecture
```
┌─────────────────────────────────────────────────────────┐
│                    Target Host (Linux)                  │
│                                                         │
│  ┌────────────┐    ┌──────────────────────────────────┐ │
│  │ Any Binary │◄───│        eBPF Programs             │ │
│  └────────────┘    └──────────┬───────────────────────┘ │
│                               │ perf ring buffer        │
│                    ┌──────────▼───────────────────────┐ │
│                    │     ZeroTrace Agent (Go 1.22)    │ │
│                    └──────────┬───────────────────────┘ │
└───────────────────────────────┼─────────────────────────┘
                                │ gRPC (plaintext)
                    ┌───────────▼───────────────────────┐
                    │  ZeroTrace Collector (Go 1.22)    │
                    └───────────┬───────────────────────┘
                                │ HTTP / WebSocket
                    ┌───────────▼───────────────────────┐
                    │   ZeroTrace UI (React 18 + TS 5)  │
                    └───────────────────────────────────┘
```

## Prerequisites
- Linux kernel ≥ 5.15 with BTF enabled (`/sys/kernel/btf/vmlinux` must exist)
- `clang` ≥ 14, `libbpf-dev`, `bpftool`
- Go 1.22+
- Node.js 20+ (via [nvm](https://github.com/nvm-sh/nvm))
- Root privileges (for agent only)

> **WSL2 users:** WSL2 kernel 6.x ships with BTF enabled. All commands below work as-is.

---

## Quick Start (WSL2 / Linux)

### 1. Install NVM + Node.js (first time only)
```bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
source ~/.nvm/nvm.sh
nvm install 20
```

### 2. Start the Collector
```bash
# From project root — use the provided script (handles env vars correctly)
bash deploy/run-collector.sh

# Or manually with a custom data directory:
cd collector
ZEROTRACE_STORAGE_PATH=/tmp/badger go run .
```

### 3. Start the UI
```bash
# From project root — script handles NVM + dep install automatically
bash deploy/run-ui.sh

# Or manually:
source ~/.nvm/nvm.sh          # ← required in WSL if node is not on PATH
cd ui
npm install                   # first time only
npm run dev
```
Open **http://localhost:5173** in your browser.

### 4. Start the Agent (Linux root required)
```bash
# Requires: clang, libbpf-dev, bpftool, kernel ≥ 5.15 with BTF
sudo bash -c "cd agent && go run . --config deploy/configs/agent.yaml"
```

---

## Running Tests (no kernel required)
```bash
# Agent unit tests (correlator + event parser)
cd test/unit && go test ./... -v

# Collector tests (BadgerDB + assembler + graph)
cd collector && go test ./store/... ./ingest/... ./graph/... -v

# TypeScript type check
cd ui && source ~/.nvm/nvm.sh && npx tsc --noEmit

# All tests via Makefile (from project root)
make test
```

---

## Configuration

### Collector (`deploy/configs/collector.yaml`)
```yaml
grpc:
  address: "0.0.0.0:4317"
http:
  address: "0.0.0.0:8080"
storage:
  path: "/data/badger"       # override with ZEROTRACE_STORAGE_PATH env var
retention:
  hours: 24
```

All keys can be overridden with environment variables using the pattern `ZEROTRACE_<SECTION>_<KEY>`:
```bash
ZEROTRACE_STORAGE_PATH=/tmp/badger
ZEROTRACE_GRPC_ADDRESS=0.0.0.0:9317
ZEROTRACE_HTTP_ADDRESS=0.0.0.0:9080
```

### Agent (`deploy/configs/agent.yaml`)
```yaml
collector:
  address: "localhost:4317"
probes:
  tcp: true
  ssl: true
  process: true
```

---

## How It Works
ZeroTrace uses eBPF kprobes and uprobes to hook into network and process execution events. When an application writes to a TCP socket or calls `SSL_write`, the eBPF program captures the data and sends it via a ring buffer to the Go agent. The agent correlates these events causally to generate OpenTelemetry-like spans, which are then shipped to the Collector for storage (BadgerDB) and visualization.

## Known Limitations
- IPv6 unsupported (IPv4 only)
- macOS/Windows not supported natively (WSL2 works)
- New processes miss first ~10s of SSL traces (uprobe scanner interval)
- Non-HTTP TCP is correlated at connection granularity

## FAQ
- **Does this require a kernel module?** No.
- **Does it work with HTTPS?** Yes, via SSL uprobes on `libssl.so`.
- **What is the performance overhead?** < 1% CPU.
- **Why does `npm run dev` fail in WSL?** Node isn't on PATH by default — run `source ~/.nvm/nvm.sh` first, or use `bash deploy/run-ui.sh` which handles this automatically.
