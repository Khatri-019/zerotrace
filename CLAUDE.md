# ZeroTrace

**Goal**: eBPF-Based Zero-Overhead Distributed Tracer (Phase 1 complete).
**Directories**:
- `agent/`: Go userspace agent
- `collector/`: Go ingestion server
- `ui/`: React/Vite dashboard
- `bpf/`: eBPF C programs
- `proto/`: gRPC definitions
- `bench/`: Benchmark suite
- `test/`: Unit/E2E tests
- `deploy/`: Docker compose and configs

**Build**: `make all`

**Status**: Complete
