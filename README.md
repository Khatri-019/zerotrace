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
- Linux kernel ≥ 5.15
- clang ≥ 14, libbpf-dev, bpftool
- Go 1.22+
- Node 20+
- Root privileges (for agent)

## Quick Start
```bash
git clone <repo>
make all
cd collector && ./bin/zerotrace-collector
sudo cd agent && ./bin/zerotrace-agent
```

## How It Works
ZeroTrace uses eBPF kprobes and uprobes to hook into network and process execution events. When an application writes to a TCP socket or calls SSL_write, the eBPF program captures the data and sends it via a ring buffer to the Go agent. The agent correlates these events causally to generate OpenTelemetry-like spans, which are then shipped to the Collector for storage (BadgerDB) and visualization.

## UI Screenshots
See /screenshots directory.

## Benchmark Results
Results populated in `bench/results/latest.json`. CPU overhead is < 1%.

## Known Limitations
- IPv6 unsupported
- macOS/Windows unsupported natively (requires Linux VM)
- New processes miss first 5s of SSL traces
- Non-HTTP TCP correlated at connection granularity

## FAQ
- Does this require a kernel module? No.
- Does it work with HTTPS? Yes, via SSL uprobes.
- What is the performance overhead? < 1% CPU.
