# PRD: ZeroTrace — eBPF-Based Zero-Overhead Distributed Tracer
## Document Metadata
- **Project Name:** ZeroTrace
- **Version:** 2.0 (production-ready, AI-agent-complete)
- **Module Path:** `github.com/zerotrace/zerotrace`
- **Go Version:** 1.22
- **Target:** Autonomous AI agent implementation — zero human intervention required
- **Execution Environment:** Ubuntu 22.04 LTS VM, root privileges, kernel ≥ 5.15 with BTF enabled, `bpftool` installed
- **Estimated Build Time:** 8–12 weeks (solo agent)
- **Primary Languages:** C (eBPF programs), Go 1.22 (agent + collector), TypeScript/React (UI)

---

## AGENT META-INSTRUCTIONS (Read First, Always)

You are an autonomous AI software engineer implementing the ZeroTrace PRD exactly as specified. Act as a senior systems engineer: write robust error handling, avoid deprecated eBPF helpers, and ensure memory safety in both C and Go.

**Rules:**
1. Do NOT ask for permission to proceed to the next phase. Execute the Implementation Order (Section 12) sequentially.
2. For every phase: write the code → verify it compiles → fix all verifier/compilation errors autonomously → update `CLAUDE.md` → move to next phase.
3. If a BPF verifier error occurs, fix it before proceeding. Never stub or skip BPF programs.
4. Every `.bpf.c` file MUST include `char LICENSE[] SEC("license") = "Dual BSD/GPL";` — without this the kernel rejects the program unconditionally.
5. Use structured logging (`go.uber.org/zap`) everywhere. No `fmt.Println`, no `log.Printf`.
6. All configuration via `spf13/viper`. Never hardcode IPs, ports, or paths.
7. On any ambiguity not covered by this PRD, choose the simpler, more explicit implementation.

---

## 1. Project Overview

### 1.1 What We Are Building
ZeroTrace is a distributed tracing system that instruments any running application at the Linux kernel level using eBPF probes — with **zero code changes, zero library imports, and zero restarts** of the target application.

It captures per-request latency, syscall patterns, TCP connection flows, and cross-service dependency graphs across a fleet of microservices by attaching eBPF programs to kernel hooks (kprobes, uprobes, tracepoints). Trace data streams via ring buffers to a userspace Go agent, forwards to a central collector, and renders as trace timelines and service dependency maps in a React dashboard.

### 1.2 Core Value Proposition
- Works on **any binary** — Python, Java, Go, Rust, Node.js — without touching source
- **Sub-1% CPU overhead** under normal load (verified by benchmark suite in Section 9)
- **Causal trace correlation** across services without injecting headers
- **Production-safe** — BPF verifier sandboxes all programs; no kernel panic risk

### 1.3 Hard Scope Boundaries (Do NOT implement)
- No IPv6 support
- No Windows/macOS support
- No Kubernetes pod/namespace abstraction
- No Prometheus metrics integration
- No authentication/RBAC on any API
- No gRPC TLS (plaintext only)
- No eBPF CO-RE for kernels without BTF (kernel < 5.8)
- No persistent storage beyond 24h BadgerDB TTL
- No alerting rules

---

## 2. System Architecture

### 2.1 High-Level Components

```
┌─────────────────────────────────────────────────────────┐
│                    Target Host (Linux)                   │
│                                                          │
│  ┌────────────┐    ┌──────────────────────────────────┐ │
│  │ Any Binary │    │        eBPF Programs              │ │
│  │ (Python /  │◄───│  kprobe__tcp_sendmsg              │ │
│  │  Java /    │    │  kprobe__tcp_recvmsg              │ │
│  │  Go / etc) │    │  uprobe__SSL_write (OpenSSL)      │ │
│  └────────────┘    │  tracepoint__sched_process_exec   │ │
│                    └──────────┬───────────────────────┘ │
│                               │ perf ring buffer         │
│                    ┌──────────▼───────────────────────┐ │
│                    │     ZeroTrace Agent (Go 1.22)     │ │
│                    │  - Loads & manages BPF programs   │ │
│                    │  - Reads ring buffers             │ │
│                    │  - Correlates kernel events       │ │
│                    │  - DWARF symbol resolution        │ │
│                    │  - gRPC stream to Collector       │ │
│                    └──────────┬───────────────────────┘ │
└───────────────────────────────┼─────────────────────────┘
                                │ gRPC (plaintext)
                    ┌───────────▼───────────────────────┐
                    │  ZeroTrace Collector (Go 1.22)     │
                    │  - Receives spans from N agents    │
                    │  - Assembles distributed traces    │
                    │  - Stores in BadgerDB              │
                    │  - Serves REST + WebSocket API     │
                    └───────────┬───────────────────────┘
                                │ HTTP / WebSocket
                    ┌───────────▼───────────────────────┐
                    │   ZeroTrace UI (React 18 + TS 5)   │
                    │  - Service dependency map (D3)     │
                    │  - Trace timeline (Gantt)          │
                    │  - Live tail of recent traces      │
                    └───────────────────────────────────┘
```

### 2.2 Data Flow (Per Request)
1. Target app calls `write()` on a TCP socket → kernel executes `tcp_sendmsg`
2. kprobe fires → eBPF program reads `sk_buff`, extracts `{pid, tid, fd, bytes, timestamp_ns}`
3. eBPF program looks up BPF hash map `{pid, fd} → active_request_t`
4. Event is written to per-CPU ring buffer (`BPF_MAP_TYPE_RINGBUF`)
5. Go agent polls ring buffer via epoll, reads raw event structs
6. Agent enriches: resolves pid → process name via `/proc/<pid>/comm`, fd → remote IP:port via `/proc/net/tcp`
7. Agent runs causal correlation engine (Section 5)
8. Agent emits an OpenTelemetry-compatible span via gRPC to Collector
9. Collector assembles spans into traces by trace_id, persists to BadgerDB
10. UI queries REST API, renders timeline and service graph

---

## 3. Repository Structure

Agent must create this exact structure. Do not deviate.

```
zerotrace/
├── README.md
├── CLAUDE.md                         # Agent context — created in Phase 1, updated each phase
├── Makefile                          # Top-level orchestration
├── go.work                           # Go workspace (agent + collector share workspace)
├── .gitmodules                       # libbpf as git submodule
│
├── vendor/
│   └── libbpf/                       # git submodule: github.com/libbpf/libbpf
│
├── bpf/
│   ├── headers/
│   │   ├── vmlinux.h                 # Generated: bpftool btf dump file vmlinux format c
│   │   ├── bpf_helpers.h             # Copied from vendor/libbpf/src/bpf_helpers.h
│   │   └── common.h                  # Shared structs between BPF C and Go userspace
│   ├── tcp_trace.bpf.c
│   ├── ssl_trace.bpf.c
│   ├── sched_trace.bpf.c
│   ├── http_trace.bpf.c
│   └── xdp_classifier.bpf.c
│
├── agent/
│   ├── go.mod                        # module github.com/zerotrace/zerotrace/agent, go 1.22
│   ├── main.go
│   ├── loader/
│   │   ├── loader.go                 # bpf2go generated code consumer + probe attacher
│   │   └── maps.go                   # BPF map read/write helpers
│   ├── reader/
│   │   ├── ringbuf.go                # Ring buffer poller (epoll via cilium/ebpf ringbuf.Reader)
│   │   └── event_parser.go           # Raw bytes → Go event structs
│   ├── enricher/
│   │   ├── proc.go                   # /proc resolution: pid→name, fd→remote addr
│   │   ├── dwarf.go                  # ELF/DWARF symbol resolution for uprobes
│   │   └── ssl_detector.go           # OpenSSL version detection + struct offset table
│   ├── correlator/
│   │   ├── correlator.go             # Causal correlation engine
│   │   └── request_tracker.go        # In-flight request state machine
│   ├── exporter/
│   │   ├── grpc_exporter.go          # Batched gRPC span sender
│   │   └── otel_span.go              # Internal span → OTLP proto span
│   └── config/
│       └── config.go                 # Viper config loader with typed struct + defaults
│
├── collector/
│   ├── go.mod                        # module github.com/zerotrace/zerotrace/collector, go 1.22
│   ├── main.go
│   ├── ingest/
│   │   ├── grpc_server.go            # gRPC server receiving SpanBatch streams
│   │   └── span_assembler.go         # Groups spans into complete traces
│   ├── store/
│   │   ├── badger_store.go           # BadgerDB persistence with 24h TTL
│   │   └── index.go                  # In-memory recent-10k trace index
│   ├── api/
│   │   ├── rest.go                   # gorilla/mux REST endpoints
│   │   └── websocket.go              # Live tail WebSocket endpoint
│   └── graph/
│       └── dependency_graph.go       # Service adjacency map with HdrHistogram stats
│
├── ui/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── styles/
│       │   ├── global.css            # CSS custom properties (design tokens)
│       │   └── reset.css             # Minimal CSS reset
│       ├── api/
│       │   └── client.ts             # REST + WebSocket client (no axios — native fetch)
│       ├── components/
│       │   ├── layout/
│       │   │   ├── AppShell.tsx      # Top nav + left sidebar + main content
│       │   │   ├── TopNav.tsx
│       │   │   └── Sidebar.tsx
│       │   ├── ServiceMap.tsx        # D3 force-directed service graph
│       │   ├── TraceTimeline.tsx     # Gantt-style span timeline
│       │   ├── TraceLiveTable.tsx    # Live tail table (WebSocket)
│       │   ├── TraceListTable.tsx    # Paginated trace list
│       │   ├── SpanDetailDrawer.tsx  # Right-side drawer, span metadata
│       │   └── shared/
│       │       ├── Badge.tsx         # Status badges (success/error/pending)
│       │       ├── DurationBar.tsx   # Inline horizontal bar for duration
│       │       └── EmptyState.tsx    # Empty/loading state placeholder
│       ├── stores/
│       │   └── traceStore.ts         # Zustand global state
│       ├── hooks/
│       │   ├── useTraces.ts          # React Query data fetching hook
│       │   └── useLiveTail.ts        # WebSocket subscription hook
│       └── types/
│           └── trace.ts              # TypeScript types mirroring Go structs exactly
│
├── proto/
│   └── zerotrace.proto
│
├── bench/
│   ├── overhead_test.go
│   ├── throughput_test.go
│   └── target_apps/
│       ├── flask_app.py              # Python Flask target (pip install flask)
│       └── go_http_server.go         # Go HTTP target
│
├── test/
│   ├── integration/
│   │   └── e2e_test.go
│   └── unit/
│       ├── correlator_test.go
│       └── event_parser_test.go
│
└── deploy/
    ├── docker-compose.yml            # Collector + UI only (agent runs on host)
    ├── docker-compose.dev.yml        # Dev: agent in privileged container
    └── configs/
        ├── agent.yaml
        └── collector.yaml
```

---

## 4. Dependency Setup (Phase 1 — Execute Before Any Code)

### 4.1 libbpf Git Submodule
```bash
git init
git submodule add https://github.com/libbpf/libbpf vendor/libbpf
git submodule update --init --recursive
# Copy required headers
cp vendor/libbpf/src/bpf_helpers.h bpf/headers/
cp vendor/libbpf/src/bpf_helper_defs.h bpf/headers/
cp vendor/libbpf/src/bpf_tracing.h bpf/headers/
cp vendor/libbpf/src/bpf_core_read.h bpf/headers/
```

### 4.2 Generate vmlinux.h
```bash
# Requires running kernel with BTF support (/sys/kernel/btf/vmlinux must exist)
bpftool btf dump file /sys/kernel/btf/vmlinux format c > bpf/headers/vmlinux.h
```
If `/sys/kernel/btf/vmlinux` does not exist, abort with error: "Kernel BTF not available. Requires kernel ≥ 5.8 compiled with CONFIG_DEBUG_INFO_BTF=y."

### 4.3 Go Workspace Setup
```bash
mkdir agent collector
cd agent && go mod init github.com/zerotrace/zerotrace/agent
cd ../collector && go mod init github.com/zerotrace/zerotrace/collector
cd ..
go work init ./agent ./collector
```

### 4.4 Protobuf Code Generation
```bash
# Install tools
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# Generate
protoc --go_out=agent/proto --go-grpc_out=agent/proto \
       --go_out=collector/proto --go-grpc_out=collector/proto \
       proto/zerotrace.proto
```

---

## 5. eBPF Programs (`bpf/`)

### 5.1 MANDATORY Header for Every `.bpf.c` File
Every single `.bpf.c` file must begin with exactly this block. Missing this causes kernel rejection:

```c
// SPDX-License-Identifier: Dual BSD/GPL
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>
#include "common.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";
```

### 5.2 Shared Structs (`bpf/headers/common.h`)

```c
#pragma once

#define EVENT_TCP_SEND    1
#define EVENT_TCP_RECV    2
#define EVENT_SSL_WRITE   3
#define EVENT_SSL_READ    4
#define EVENT_PROC_EXEC   5
#define EVENT_PROC_EXIT   6
#define EVENT_HTTP_REQ    7
#define EVENT_HTTP_RESP   8

#define MAX_DATA_SIZE     256
#define TASK_COMM_LEN     16
#define MAX_PATH_LEN      128

struct tcp_event_t {
    __u64 timestamp_ns;
    __u32 pid;
    __u32 tid;
    __u64 sk_ptr;
    __u32 bytes;
    __u16 sport;
    __u16 dport;
    __u8  saddr[4];
    __u8  daddr[4];
    __u8  event_type;
    char  comm[TASK_COMM_LEN];
};

struct ssl_event_t {
    __u64 timestamp_ns;
    __u32 pid;
    __u32 tid;
    __u64 ssl_ptr;
    __u32 bytes;
    __u8  event_type;
    char  comm[TASK_COMM_LEN];
    char  data[MAX_DATA_SIZE];
};

struct proc_event_t {
    __u64 timestamp_ns;
    __u32 pid;
    __u32 ppid;
    __u8  event_type;
    char  comm[TASK_COMM_LEN];
    char  filename[MAX_PATH_LEN];
    __u32 exit_code;
};

struct http_event_t {
    __u64 timestamp_ns;
    __u32 pid;
    __u32 tid;
    __u64 conn_id;
    __u8  event_type;
    __u16 status_code;
    char  method[8];
    char  path[128];
    char  host[64];
    __u64 content_length;
};
```

### 5.3 BPF Maps (define identically in each `.bpf.c` that uses them)

```c
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024 * 1024);
} events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u64);    // (pid << 32 | tid)
    __type(value, __u64);  // active sk_ptr
} active_conns SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u64);    // (pid << 32 | tid)
    __type(value, __u64);  // SSL* pointer stashed at uprobe entry
} ssl_args SEC(".maps");
```

### 5.4 `tcp_trace.bpf.c`

```c
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(kprobe__tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size) {
    struct tcp_event_t *event;
    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;

    event->timestamp_ns = bpf_ktime_get_ns();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->tid = (u32)bpf_get_current_pid_tgid();
    event->sk_ptr = (u64)sk;
    event->bytes = size;
    event->event_type = EVENT_TCP_SEND;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    // CO-RE: access kernel struct fields safely
    BPF_CORE_READ_INTO(&event->dport, sk, __sk_common.skc_dport);
    BPF_CORE_READ_INTO(&event->sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&event->daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&event->saddr, sk, __sk_common.skc_rcv_saddr);

    bpf_ringbuf_submit(event, 0);
    return 0;
}

SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(kprobe__tcp_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int flags, int *addr_len) {
    // Mirror of tcp_sendmsg with EVENT_TCP_RECV
    struct tcp_event_t *event;
    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;

    event->timestamp_ns = bpf_ktime_get_ns();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->tid = (u32)bpf_get_current_pid_tgid();
    event->sk_ptr = (u64)sk;
    event->bytes = len;
    event->event_type = EVENT_TCP_RECV;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));
    BPF_CORE_READ_INTO(&event->dport, sk, __sk_common.skc_dport);
    BPF_CORE_READ_INTO(&event->sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&event->daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&event->saddr, sk, __sk_common.skc_rcv_saddr);

    bpf_ringbuf_submit(event, 0);
    return 0;
}
```

### 5.5 `ssl_trace.bpf.c`

```c
// Uprobe on SSL_write entry: stash (SSL*, buf, num) for use at return
SEC("uprobe/SSL_write")
int uprobe__SSL_write(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u64 ssl_ptr = PT_REGS_PARM1(ctx);
    bpf_map_update_elem(&ssl_args, &pid_tgid, &ssl_ptr, BPF_ANY);
    return 0;
}

// Uretprobe on SSL_write: read plaintext buffer
SEC("uretprobe/SSL_write")
int uretprobe__SSL_write(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u64 *ssl_ptr = bpf_map_lookup_elem(&ssl_args, &pid_tgid);
    if (!ssl_ptr) return 0;

    struct ssl_event_t *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;

    event->timestamp_ns = bpf_ktime_get_ns();
    event->pid = pid_tgid >> 32;
    event->tid = (u32)pid_tgid;
    event->ssl_ptr = *ssl_ptr;
    event->event_type = EVENT_SSL_WRITE;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    // Read buf argument (2nd param saved at entry — read from stash indirectly)
    // buf is PT_REGS_PARM2 at entry — re-read from /proc stack or pass via map
    // Implementation: use a second map entry keyed by pid_tgid to stash buf ptr
    void *buf = (void *)PT_REGS_PARM2(ctx);  // NOTE: valid only if using entry args map
    bpf_probe_read_user(event->data, sizeof(event->data), buf);
    event->bytes = PT_REGS_PARM3(ctx);

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&ssl_args, &pid_tgid);
    return 0;
}
// Repeat symmetrically for SSL_read / uretprobe__SSL_read with EVENT_SSL_READ
```

**Implementation note for agent:** For SSL_write/SSL_read, the correct pattern for capturing multi-arg uprobe state is to create a second BPF map `ssl_write_args` that stores a struct `{ssl_ptr, buf_ptr, num}` at entry, keyed by pid_tgid. Read all three params at entry uprobe, stash them, read data at uretprobe. Implement this pattern.

### 5.6 `sched_trace.bpf.c`

```c
SEC("tracepoint/sched/sched_process_exec")
int tracepoint__sched_process_exec(struct trace_event_raw_sched_process_exec *ctx) {
    struct proc_event_t *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;

    event->timestamp_ns = bpf_ktime_get_ns();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->ppid = (u32)bpf_get_current_pid_tgid(); // will be resolved in userspace
    event->event_type = EVENT_PROC_EXEC;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));
    // filename from ctx->filename offset
    bpf_probe_read_str(&event->filename, sizeof(event->filename),
                       (void *)ctx + ctx->__data_loc_filename);
    bpf_ringbuf_submit(event, 0);
    return 0;
}

SEC("tracepoint/sched/sched_process_exit")
int tracepoint__sched_process_exit(struct trace_event_raw_sched_process_template *ctx) {
    struct proc_event_t *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;

    event->timestamp_ns = bpf_ktime_get_ns();
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->event_type = EVENT_PROC_EXIT;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));
    bpf_ringbuf_submit(event, 0);
    return 0;
}
```

### 5.7 `http_trace.bpf.c`
Parses plaintext HTTP/1.1 from SSL data captured by `ssl_trace.bpf.c`. Called as a tail call or implemented as a helper inline in ssl_trace.

```c
// Inline function — called from ssl_trace uretprobe when data contains "HTTP"
static __always_inline void parse_http(struct ssl_event_t *ssl_ev) {
    // Check if data starts with known HTTP methods
    // Use explicit byte comparisons — no strcmp in BPF
    if (ssl_ev->data[0] == 'G' && ssl_ev->data[1] == 'E' &&
        ssl_ev->data[2] == 'T' && ssl_ev->data[3] == ' ') {
        // Parse GET request — extract path up to next space
        // Max 127 chars, bounds checked
    }
    // Repeat for POST, PUT, DELETE, PATCH, HEAD
    // For responses: check for "HTTP/1." prefix → extract status code digits
}
```

**Verifier compliance rules for http_trace:**
- All string iteration loops must have explicit `#pragma unroll` or bounded `for (int i = 0; i < MAX_LEN && i < actual_len; i++)`
- No dynamic stack allocations
- All data accesses bounds-checked with `if (offset + len <= MAX_DATA_SIZE)`

### 5.8 Build System (`Makefile`)

```makefile
CLANG    := clang
LLC      := llc
BPFTOOL  := bpftool
AGENT_DIR    := agent
COLLECTOR_DIR := collector
UI_DIR   := ui
BPF_DIR  := bpf
HEADERS  := $(BPF_DIR)/headers
VMLINUX  := $(HEADERS)/vmlinux.h
BPF_SRCS := $(wildcard $(BPF_DIR)/*.bpf.c)
BPF_OBJS := $(patsubst %.bpf.c,%.bpf.o,$(BPF_SRCS))

.PHONY: all bpf agent collector ui test bench clean

all: deps vmlinux bpf generate agent collector ui

deps:
	git submodule update --init --recursive
	cp vendor/libbpf/src/bpf_helpers.h $(HEADERS)/
	cp vendor/libbpf/src/bpf_helper_defs.h $(HEADERS)/
	cp vendor/libbpf/src/bpf_tracing.h $(HEADERS)/
	cp vendor/libbpf/src/bpf_core_read.h $(HEADERS)/

vmlinux: $(VMLINUX)
$(VMLINUX):
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > $@

# Compile each BPF program
$(BPF_DIR)/%.bpf.o: $(BPF_DIR)/%.bpf.c $(VMLINUX)
	$(CLANG) -g -O2 -target bpf \
		-D__TARGET_ARCH_x86 \
		-I$(HEADERS) \
		-Wall -Wno-unused-variable \
		-c $< -o $@

bpf: $(BPF_OBJS)

# bpf2go code generation — run from agent directory
generate:
	cd $(AGENT_DIR) && go generate ./loader/...

agent: generate
	cd $(AGENT_DIR) && go build -o ../bin/zerotrace-agent ./...

collector:
	cd $(COLLECTOR_DIR) && go build -o ../bin/zerotrace-collector ./...

ui:
	cd $(UI_DIR) && npm ci && npm run build

test:
	cd $(AGENT_DIR) && go test ./test/unit/...
	cd $(COLLECTOR_DIR) && go test ./...

bench:
	cd $(AGENT_DIR) && sudo go test ./bench/... -bench=. -benchtime=60s -v

clean:
	rm -f $(BPF_DIR)/*.bpf.o
	rm -f bin/*
	rm -rf $(VMLINUX)
	cd $(UI_DIR) && rm -rf dist node_modules
```

### 5.9 bpf2go Directive (exact comment required in `agent/loader/loader.go`)

```go
package loader

// Generate BPF Go bindings for each program.
// Run: make generate (calls go generate ./loader/...)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../bpf/headers" TcpTrace ../bpf/tcp_trace.bpf.c -- -I../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../bpf/headers" SslTrace ../bpf/ssl_trace.bpf.c -- -I../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../bpf/headers" SchedTrace ../bpf/sched_trace.bpf.c -- -I../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../bpf/headers" HttpTrace ../bpf/http_trace.bpf.c -- -I../bpf/headers
```

---

## 6. Go Agent (`agent/`)

### 6.1 Module Dependencies (`agent/go.mod`)

```
module github.com/zerotrace/zerotrace/agent

go 1.22

require (
    github.com/cilium/ebpf                  v0.14.0
    github.com/vishvananda/netlink          v1.1.0
    golang.org/x/sys                        v0.20.0
    google.golang.org/grpc                  v1.64.0
    google.golang.org/protobuf              v1.34.1
    go.uber.org/zap                         v1.27.0
    github.com/spf13/viper                  v1.19.0
    github.com/ianlancetaylor/demangle      v0.0.0-20240312041847-bd984b5ce465
)
```

### 6.2 Config Loader (`agent/config/config.go`)

Use `spf13/viper`. Load from `agent.yaml`. Fall back to these exact defaults if key is missing:

```go
package config

import (
    "github.com/spf13/viper"
    "go.uber.org/zap"
)

type Config struct {
    Collector struct {
        Address string `mapstructure:"address"`
        TLS     bool   `mapstructure:"tls"`
    } `mapstructure:"collector"`
    Probes struct {
        TCP       bool `mapstructure:"tcp"`
        SSL       bool `mapstructure:"ssl"`
        HTTPParse bool `mapstructure:"http_parse"`
        Process   bool `mapstructure:"process"`
    } `mapstructure:"probes"`
    Filters struct {
        ExcludeComms []string `mapstructure:"exclude_comms"`
        ExcludePorts []int    `mapstructure:"exclude_ports"`
    } `mapstructure:"filters"`
    RingBuffer struct {
        SizeMB int `mapstructure:"size_mb"`
    } `mapstructure:"ring_buffer"`
    Export struct {
        BatchSize       int `mapstructure:"batch_size"`
        FlushIntervalMS int `mapstructure:"flush_interval_ms"`
    } `mapstructure:"export"`
}

func Load(path string, log *zap.Logger) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(path)

    // Defaults — agent must not panic if config key is absent
    v.SetDefault("collector.address", "localhost:4317")
    v.SetDefault("collector.tls", false)
    v.SetDefault("probes.tcp", true)
    v.SetDefault("probes.ssl", true)
    v.SetDefault("probes.http_parse", true)
    v.SetDefault("probes.process", true)
    v.SetDefault("filters.exclude_comms", []string{"agent", "sshd", "systemd", "kworker"})
    v.SetDefault("filters.exclude_ports", []int{22, 53})
    v.SetDefault("ring_buffer.size_mb", 256)
    v.SetDefault("export.batch_size", 100)
    v.SetDefault("export.flush_interval_ms", 100)

    if err := v.ReadInConfig(); err != nil {
        log.Warn("Config file not found, using defaults", zap.String("path", path), zap.Error(err))
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### 6.3 Agent Startup Sequence (`agent/main.go`)

```go
func main() {
    // 1. Init structured logger (zap, production mode)
    log, _ := zap.NewProduction()
    defer log.Sync()

    // 2. Load config via viper
    cfg, err := config.Load("deploy/configs/agent.yaml", log)
    if err != nil { log.Fatal("config load failed", zap.Error(err)) }

    // 3. Check root privileges
    if os.Getuid() != 0 {
        log.Fatal("zerotrace-agent requires root privileges to load BPF programs")
    }

    // 4. Verify BTF availability
    if _, err := os.Stat("/sys/kernel/btf/vmlinux"); os.IsNotExist(err) {
        log.Fatal("kernel BTF not available — requires kernel >= 5.8 with CONFIG_DEBUG_INFO_BTF=y")
    }

    // 5. Context tied to OS signals for graceful shutdown
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // 6. Load BPF objects and attach probes
    mgr, err := loader.New(cfg, log)
    if err != nil { log.Fatal("BPF loader failed", zap.Error(err)) }
    defer mgr.Close() // MUST close all BPF objects to prevent kernel memory leak

    // 7. Start ring buffer reader (goroutine)
    eventCh := make(chan reader.RawEvent, 100_000)
    go reader.PollRingBuffer(ctx, mgr.RingBufReader(), eventCh, log)

    // 8. Start correlator (goroutine)
    spanCh := make(chan *proto.Span, 10_000)
    go correlator.Run(ctx, eventCh, spanCh, log)

    // 9. Connect to collector via gRPC
    exp, err := exporter.New(cfg.Collector.Address, log)
    if err != nil { log.Fatal("exporter init failed", zap.Error(err)) }
    defer exp.Close()

    // 10. Start exporter (goroutine)
    go exp.Run(ctx, spanCh, cfg.Export.BatchSize, cfg.Export.FlushIntervalMS)

    // 11. Start /proc scanner — attach uprobes to new SSL processes every 5s
    go loader.ScanAndAttachUprobes(ctx, mgr, cfg, log)

    log.Info("zerotrace-agent running", zap.String("collector", cfg.Collector.Address))
    <-ctx.Done()
    log.Info("shutting down — cleaning up BPF resources")
    // mgr.Close() called by defer above — closes ringbuf reader, detaches all links, frees maps
}
```

### 6.4 Graceful Shutdown Requirements (`agent/loader/loader.go`)

The `Manager.Close()` method must execute in this order:
1. Close `ringbuf.Reader` (stops ring buffer polling)
2. Call `link.Close()` on every attached kprobe, tracepoint, and uprobe link
3. Close the BPF collection objects (frees kernel map memory)
4. Log "all BPF resources released" at info level

Failure to close links leaks kernel memory until reboot. This is non-optional.

### 6.5 Causal Correlation Engine (`agent/correlator/correlator.go`)

#### Connection-Level Correlation
```go
type ConnKey struct {
    SrcIP   [4]byte
    SrcPort uint16
    DstIP   [4]byte
    DstPort uint16
}

type ConnState struct {
    TraceID      string
    SpanID       string
    ServiceName  string
    RequestStart time.Time
    LastSeen     time.Time
}

// connTable maps TCP 4-tuple to active trace context
// Protected by sync.RWMutex
var connTable = struct {
    sync.RWMutex
    m map[ConnKey]*ConnState
}{m: make(map[ConnKey]*ConnState)}
```

**Correlation rules (implement all):**

1. **New outbound connection** (`tcp_sendmsg`, no existing connTable entry):
   - Generate `TraceID = fmt.Sprintf("%x", sha256(sk_ptr + timestamp_ns)[:16])`
   - Generate `SpanID = fmt.Sprintf("%x", rand.Uint64())`
   - Create `ConnState`, insert into connTable

2. **Inbound connection matches existing outbound** (dstIP:dstPort == srcIP:srcPort of existing entry):
   - Inherit `TraceID` from existing entry
   - Generate new child `SpanID`
   - Set `ParentSpanID = existing entry's SpanID`

3. **Process exec with known parent PID**:
   - Look up parent PID in `pidTraceTable`
   - Child inherits `TraceID`

4. **Span completion triggers**:
   - HTTP response event received for matching `{pid, tid, conn_id}` → emit span immediately
   - TCP close event (`tcp_close` kprobe) → emit span for that connection
   - 30-second idle timeout → emit span with `tags["timeout"] = "true"`, GC the entry

5. **Garbage collection**: every 60 seconds, walk connTable and remove entries with `LastSeen > 30s ago`

#### HTTP-Level Correlation
```go
type HTTPRequestKey struct {
    PID    uint32
    TID    uint32
    ConnID uint64
}

// In-flight HTTP requests waiting for their response
var httpInflight = struct {
    sync.Mutex
    m map[HTTPRequestKey]*HTTPSpan
}{m: make(map[HTTPRequestKey]*HTTPSpan)}

// On HTTP request event: insert into httpInflight
// On HTTP response event: pop from httpInflight, compute duration, emit span
// On 30s timeout: emit partial span with tags["incomplete"] = "true"
```

### 6.6 DWARF Symbol Resolution (`agent/enricher/dwarf.go`)

```go
// FindFunctionOffset returns the ELF offset for a named function in a binary.
// Checks DWARF debug info first, falls back to .dynsym section.
func FindFunctionOffset(binaryPath, funcName string) (uint64, error)

// GetOpenSSLLibPath returns the path to libssl.so loaded by a given PID.
// Reads /proc/<pid>/maps, finds the line containing "libssl.so".
func GetOpenSSLLibPath(pid int) (string, error)
```

**OpenSSL version offset table** (hardcoded, do not derive dynamically):
```go
var sslVersionOffsets = map[string]SSLOffsets{
    "1.0.2": {RBIOOffset: 0x58, WBIOOffset: 0x60, NumOffset: 0x28},
    "1.1.0": {RBIOOffset: 0x10, WBIOOffset: 0x18, NumOffset: 0x30},
    "1.1.1": {RBIOOffset: 0x10, WBIOOffset: 0x18, NumOffset: 0x30},
    "3.0":   {RBIOOffset: 0x10, WBIOOffset: 0x18, NumOffset: 0x30},
    "3.1":   {RBIOOffset: 0x10, WBIOOffset: 0x18, NumOffset: 0x30},
    "3.2":   {RBIOOffset: 0x10, WBIOOffset: 0x18, NumOffset: 0x30},
}
```

---

## 7. Collector (`collector/`)

### 7.1 Module Dependencies (`collector/go.mod`)

```
module github.com/zerotrace/zerotrace/collector

go 1.22

require (
    github.com/dgraph-io/badger/v4       v4.2.0
    google.golang.org/grpc               v1.64.0
    google.golang.org/protobuf           v1.34.1
    github.com/gorilla/mux               v1.8.2
    github.com/gorilla/websocket         v1.5.1
    github.com/HdrHistogram/hdrhistogram-go v1.1.2
    go.uber.org/zap                      v1.27.0
    github.com/spf13/viper               v1.19.0
)
```

### 7.2 REST API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/traces` | List traces. Params: `service`, `min_duration_ms`, `limit` (default 50, max 500), `offset` |
| GET | `/api/traces/:traceID` | Full trace with all spans |
| GET | `/api/services` | All observed service names as `string[]` |
| GET | `/api/graph` | Service dependency graph |
| GET | `/api/stats` | Agent stats: events/s, spans/s, active connections, drop count |
| WebSocket | `/ws/traces` | Live stream — new complete traces pushed as JSON |

**CORS:** All endpoints must respond with `Access-Control-Allow-Origin: *` header.

**Response schemas:** (defined in Section 8 of original PRD — unchanged)

### 7.3 BadgerDB Key Schema

```
trace:{traceID}                    → JSON(Trace)          TTL: 24h
idx:ts:{unix_sec}:{traceID}        → ""                   TTL: 24h  (time range queries)
idx:svc:{serviceName}:{traceID}    → ""                   TTL: 24h  (service filter)
graph:edge:{src}->{dst}            → JSON(EdgeStats)       TTL: 1h   (refreshed on each new trace)
```

---

## 8. Protobuf Definition (`proto/zerotrace.proto`)

```protobuf
syntax = "proto3";
package zerotrace.v1;
option go_package = "github.com/zerotrace/zerotrace/proto;proto";

service TraceIngest {
  rpc SendSpans (SendSpansRequest) returns (SendSpansResponse);
  rpc StreamSpans (stream SpanBatch) returns (stream Ack);
}

message SpanBatch {
  string agent_id = 1;
  string host     = 2;
  repeated Span spans = 3;
}

message Span {
  string trace_id       = 1;
  string span_id        = 2;
  string parent_span_id = 3;
  string service_name   = 4;
  string operation_name = 5;
  int64  start_time_ns  = 6;
  int64  end_time_ns    = 7;
  map<string, string> tags = 8;
  repeated SpanLog logs    = 9;
  SpanKind kind            = 10;
}

message SpanLog {
  int64  timestamp_ns = 1;
  string message      = 2;
}

enum SpanKind {
  SPAN_KIND_UNSPECIFIED = 0;
  SPAN_KIND_CLIENT      = 1;
  SPAN_KIND_SERVER      = 2;
}

message SendSpansRequest  { SpanBatch batch = 1; }
message SendSpansResponse { bool accepted = 1; }
message Ack               { uint64 spans_accepted = 1; }
```

---

## 9. UI Design System

### 9.1 Design Tokens (`ui/src/styles/global.css`)

The entire UI is token-driven. These tokens are the single source of truth — every component reads from them. Never hardcode a color or spacing value.

```css
:root {
  /* ── Surfaces ── */
  --color-bg-page:       #F8F9FA;   /* page background */
  --color-bg-surface:    #FFFFFF;   /* card / panel background */
  --color-bg-subtle:     #F1F3F4;   /* table row hover, sidebar bg */
  --color-bg-overlay:    #E8EAED;   /* dividers, skeleton loaders */

  /* ── Text ── */
  --color-text-primary:  #202124;   /* main content */
  --color-text-secondary:#5F6368;   /* labels, captions */
  --color-text-disabled: #9AA0A6;   /* placeholder, muted */
  --color-text-inverse:  #FFFFFF;   /* text on dark/accent backgrounds */

  /* ── Accent (Cloudflare orange) ── */
  --color-accent:        #F6821F;
  --color-accent-hover:  #E5730E;   /* darken 8% on hover */
  --color-accent-light:  #FEF3E8;   /* tinted background behind accent elements */

  /* ── Semantic ── */
  --color-success:       #1E8E3E;
  --color-success-bg:    #E6F4EA;
  --color-warning:       #E37400;
  --color-warning-bg:    #FEF7E0;
  --color-error:         #D93025;
  --color-error-bg:      #FCE8E6;

  /* ── Borders ── */
  --color-border:        #DADCE0;   /* standard border */
  --color-border-focus:  #F6821F;   /* focus ring */

  /* ── Typography ── */
  --font-family:         'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  --font-size-xs:        11px;
  --font-size-sm:        12px;
  --font-size-base:      14px;
  --font-size-md:        16px;
  --font-size-lg:        20px;
  --font-weight-normal:  400;
  --font-weight-medium:  500;
  --font-weight-semibold:600;
  --line-height-tight:   1.3;
  --line-height-normal:  1.5;

  /* ── Spacing scale ── */
  --space-1:  4px;
  --space-2:  8px;
  --space-3:  12px;
  --space-4:  16px;
  --space-5:  20px;
  --space-6:  24px;
  --space-8:  32px;
  --space-10: 40px;

  /* ── Shape ── */
  --radius-sm: 4px;
  --radius-md: 6px;
  --radius-lg: 8px;

  /* ── Shadows — kept deliberately minimal ── */
  --shadow-sm: 0 1px 2px rgba(60,64,67,0.12);
  --shadow-md: 0 2px 6px rgba(60,64,67,0.15);

  /* ── Z-index ── */
  --z-sidebar:  100;
  --z-topnav:   200;
  --z-drawer:   300;
  --z-toast:    400;

  /* ── Transitions ── */
  --transition-fast: 120ms ease;
  --transition-base: 200ms ease;
}

/* Load Inter from Google Fonts (agent must add to index.html) */
/* <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet"> */

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  font-family: var(--font-family);
  font-size: var(--font-size-base);
  color: var(--color-text-primary);
  background: var(--color-bg-page);
  line-height: var(--line-height-normal);
  -webkit-font-smoothing: antialiased;
}
```

### 9.2 Layout (`components/layout/AppShell.tsx`)

```
┌─────────────────────────────────────────────────────────┐
│  TopNav (height: 56px, border-bottom: var(--color-border))│
│  Logo "ZeroTrace" left  |  host selector dropdown right  │
├─────────┬───────────────────────────────────────────────┤
│ Sidebar │  Main Content Area                            │
│ 220px   │  padding: var(--space-6)                      │
│         │                                               │
│ nav     │                                               │
│ items:  │                                               │
│ • Live  │                                               │
│ • Traces│                                               │
│ • Graph │                                               │
│         │                                               │
└─────────┴───────────────────────────────────────────────┘
```

- Sidebar background: `var(--color-bg-subtle)`
- Active sidebar item: left border `3px solid var(--color-accent)`, background `var(--color-accent-light)`, text `var(--color-accent)`
- No shadows on sidebar — just the right border `1px solid var(--color-border)`
- TopNav: white background, bottom border only, no shadow

### 9.3 Component Specifications

**NO external component library.** Build all components from scratch using CSS custom properties. This keeps the bundle lean and the look unmistakably custom — not shadcn, not MUI.

#### Table Rows
```css
.table-row {
  border-bottom: 1px solid var(--color-border);
  padding: var(--space-3) var(--space-4);
  font-size: var(--font-size-base);
  transition: background var(--transition-fast);
}
.table-row:hover { background: var(--color-bg-subtle); }
.table-row.selected { background: var(--color-accent-light); }
```

#### Status Badge (`shared/Badge.tsx`)
```tsx
// Pill badge — no border, background only
// success: bg var(--color-success-bg), text var(--color-success)
// error:   bg var(--color-error-bg),   text var(--color-error)
// warning: bg var(--color-warning-bg), text var(--color-warning)
// Dimensions: height 20px, padding 0 8px, border-radius var(--radius-sm), font-size var(--font-size-xs)
```

#### Buttons
```css
.btn-primary {
  background: var(--color-accent);
  color: var(--color-text-inverse);
  border: none;
  border-radius: var(--radius-md);
  padding: var(--space-2) var(--space-4);
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: background var(--transition-fast);
}
.btn-primary:hover { background: var(--color-accent-hover); }

.btn-secondary {
  background: transparent;
  color: var(--color-text-primary);
  border: 1px solid var(--color-border);
  /* same padding/radius/font as primary */
}
.btn-secondary:hover { background: var(--color-bg-subtle); }
```

#### Inputs
```css
input, select {
  height: 32px;
  padding: 0 var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-size: var(--font-size-base);
  font-family: var(--font-family);
  background: var(--color-bg-surface);
  color: var(--color-text-primary);
  outline: none;
  transition: border-color var(--transition-fast);
}
input:focus, select:focus {
  border-color: var(--color-border-focus);
}
```

### 9.4 Page Implementations

#### `/` Live Tail (`TraceLiveTable.tsx`)
- WebSocket: `ws://localhost:8080/ws/traces`
- Table columns: `Trace ID` (8 chars truncated + copy icon), `Service`, `Operation`, `Duration`, `Spans`, `Status`, `Time`
- Duration column: show number + inline `DurationBar` component (thin horizontal bar, width proportional to duration vs max in current view)
- Status: `Badge` component — 2xx → success, 4xx/5xx → error, pending → warning
- New rows animate in: `opacity: 0 → 1` over 200ms, no translate
- Pause button (top-right): freezes WebSocket consumption without disconnect; button turns orange when paused
- Max 200 rows — oldest drop off the bottom

#### `/traces` Trace List (`TraceListTable.tsx`)
- Filter bar (top): service dropdown, min duration input (ms), free-text search on operation name
- Same table structure as live tail but paginated (50 per page)
- Pagination: simple `< Prev  Page 2 of 14  Next >` — no complex pagination widget

#### `/traces/:traceID` Trace Detail
- Split layout: left 65% = `TraceTimeline`, right 35% = `SpanDetailDrawer`
- `TraceTimeline`: Gantt chart built with D3
  - Each span = horizontal rect
  - X-axis: milliseconds from trace start (0 to total duration)
  - Y-axis: one row per span, labeled with `service: operation`
  - Colors: deterministic per service — derive from `hash(serviceName) → hue`, use `hsl(hue, 60%, 45%)` — always readable, never neon
  - Click span → update `SpanDetailDrawer`
- `SpanDetailDrawer`:
  - Shows: span ID, service, operation, start/end time, duration, all tags as key-value table
  - No animation on open — instant render
  - Tags table: key in `var(--color-text-secondary)`, value in `var(--color-text-primary)`, monospace font for values

#### `/graph` Service Map (`ServiceMap.tsx`)
- D3 force-directed graph
- Nodes: circle, radius proportional to call volume (min 20px, max 50px), fill `var(--color-bg-surface)`, stroke `var(--color-border)` 2px, label below in `var(--font-size-sm)`
- Edges: directed arrows
  - Error rate < 1%: stroke `var(--color-success)`
  - Error rate 1–5%: stroke `var(--color-warning)`
  - Error rate ≥ 5%: stroke `var(--color-error)`
  - Stroke width: proportional to call count (1px min, 4px max)
- Hover node: tooltip showing `p50 / p95 / p99` latency — plain white box, `var(--shadow-sm)`, no animation
- Hover edge: tooltip showing `call count`, `error count`, latency percentiles
- Click node: opens `/traces?service=<name>` in same tab

### 9.5 UI Tech Stack

```json
{
  "react": "^18.3",
  "typescript": "^5.4",
  "vite": "^5.2",
  "zustand": "^4.5",
  "d3": "^7.9",
  "@tanstack/react-query": "^5.40",
  "react-router-dom": "^6.23"
}
```

No additional UI libraries. No Tailwind (it adds a compilation step and encourages class-soup). No MUI, shadcn, Ant Design, Chakra. CSS custom properties only.

### 9.6 Performance Requirements for UI
- Initial load: < 2s on localhost (Vite build, no lazy loading needed for v1)
- Live tail: WebSocket events must render within 1 frame (16ms) — do not batch in React state, use ref-based DOM manipulation for the live table append
- Service map D3 simulation: `alphaDecay` set so graph stabilizes within 2 seconds of load
- No `console.log` in production build — use `import.meta.env.DEV` guard

---

## 10. Dev Environment (`deploy/docker-compose.dev.yml`)

For testing the agent in a container (privileged mode for BPF):

```yaml
version: "3.9"

services:
  agent:
    build:
      context: .
      dockerfile: deploy/Dockerfile.agent
    privileged: true           # Required for BPF program loading
    pid: "host"                # Required for /proc access
    network_mode: "host"       # Required for seeing target app network traffic
    volumes:
      - /sys/kernel/btf/vmlinux:/sys/kernel/btf/vmlinux:ro
      - /sys/fs/bpf:/sys/fs/bpf                          # BPF filesystem
      - /proc:/proc:ro
      - ./deploy/configs/agent.yaml:/etc/zerotrace/agent.yaml:ro
    depends_on:
      - collector

  collector:
    build:
      context: .
      dockerfile: deploy/Dockerfile.collector
    ports:
      - "4317:4317"    # gRPC
      - "8080:8080"    # REST + WebSocket
    volumes:
      - ./deploy/configs/collector.yaml:/etc/zerotrace/collector.yaml:ro
      - zerotrace-data:/data/badger

  ui:
    build:
      context: ./ui
      dockerfile: Dockerfile
    ports:
      - "3000:80"
    depends_on:
      - collector

volumes:
  zerotrace-data:
```

For production (non-Docker): agent runs directly on host as root. Collector and UI can run in Docker or as systemd services.

---

## 11. Logging Standards

Use `go.uber.org/zap` everywhere. No `fmt.Println`, no `log.Printf`, no `fmt.Fprintf(os.Stderr, ...)`.

**Log levels:**
- `Debug`: ring buffer poll cycles, individual event receipt, correlation table lookups
- `Info`: agent startup/shutdown, probe attachment success, spans emitted per batch, collector connections
- `Warn`: ring buffer drops, SSL offset lookup miss, config file not found (using defaults)
- `Error`: BPF program load failure (with verifier log), gRPC send failure, BadgerDB write failure

**Required fields on every log line:**
```go
zap.String("component", "correlator")  // which subsystem
zap.String("pid", strconv.Itoa(pid))   // when relevant
zap.Error(err)                          // always include on error logs
```

Initialize logger once in `main.go`, pass as argument — no global logger variable.

---

## 12. Known Hard Problems and Required Solutions

### Problem 1: BPF Verifier Rejections
**Diagnosis:** `libbpf: prog 'kprobe__tcp_sendmsg': BPF program load failed: Permission denied`

**Required fixes:**
- All loops: explicit bound `for (int i = 0; i < MAX_LEN && i < actual; i++)`
- All `bpf_map_lookup_elem` returns: check for NULL before deref
- Stack usage per program: must be < 512 bytes. Move large buffers to BPF maps (`BPF_MAP_TYPE_PERCPU_ARRAY`)
- Enable verifier log on failure: set `opts.LogLevel = ebpf.LogLevelInstruction` in cilium/ebpf loader — print full verifier output to stderr before aborting

### Problem 2: SSL Uprobe Multi-Arg Stashing
**Diagnosis:** Reading garbage from SSL buffer in uretprobe

**Required fix:**
```c
// At uprobe entry (SSL_write entry):
struct ssl_write_args_t {
    __u64 ssl_ptr;
    __u64 buf_ptr;
    __u32 num;
};
// Map: BPF_MAP_TYPE_HASH, key=pid_tgid, value=ssl_write_args_t
// Stash all 3 args at entry. Read buf_ptr at uretprobe.
```

### Problem 3: Ring Buffer Backpressure
**Diagnosis:** Events silently dropped under 10k+ req/s

**Required fix:**
- Increment a `BPF_MAP_TYPE_PERCPU_ARRAY` counter on `bpf_ringbuf_reserve` returning NULL
- Read this counter every 10 seconds from Go, log warning with `zap.Warn("ring buffer drops", zap.Uint64("count", drops))`
- Go channel between ring buffer reader and correlator: capacity 100_000

### Problem 4: Process Start Race
**Diagnosis:** First 5 seconds of SSL traffic from new process not captured

**Required behavior:**
- This is a known limitation, not a bug to fix
- Document in README: "Processes starting after the agent may miss up to 5 seconds of SSL traces"
- Do not attempt to fix — any fix would require inotify on /proc which adds significant complexity

### Problem 5: Correlation Ambiguity for Non-HTTP TCP
**Diagnosis:** Multiple requests on same TCP connection can't be distinguished

**Required behavior:**
- For non-HTTP TCP (no ssl_trace): correlate at connection granularity, not request granularity
- One span per TCP connection (open to close), not per request
- Tag span with `tags["granularity"] = "connection"` so UI can show this distinction
- This is documented behavior, not a bug

---

## 13. Testing Requirements

### Unit Tests (≥ 80% coverage on `correlator` and `event_parser` packages)

`test/unit/correlator_test.go`:
```go
// TestNewOutboundConnection: new tcp_send event creates TraceID + SpanID
// TestInboundMatchesOutbound: matching 4-tuple creates child span with ParentSpanID set
// TestProcessInheritsParentTrace: exec event with known ppid inherits TraceID
// TestOrphanSpanGC: span with no close event after 30s is emitted with timeout tag
// TestHTTPRequestResponseMatching: http_req + http_resp events produce single span with duration
```

`test/unit/event_parser_test.go`:
```go
// TestTCPEventDeserialization: raw bytes → tcp_event_t Go struct
// TestHTTPGetParsing: "GET /api/v1/users HTTP/1.1\r\nHost: example.com\r\n" → method=GET, path=/api/v1/users
// TestHTTPResponseParsing: "HTTP/1.1 200 OK\r\n" → status_code=200
// TestMalformedHTTPNoPanic: garbage bytes don't panic, return partial/empty parse
// TestHTTPMethodsAll: verify GET, POST, PUT, DELETE, PATCH, HEAD all parsed correctly
```

### Integration Test (`test/integration/e2e_test.go`)
Requires root. Skipped with `testing.Short()`.

```go
// TestEndToEnd:
// 1. Start bench/target_apps/flask_app.py on :5001
// 2. Start bench/target_apps/go_http_server.go on :8081 (makes outbound calls to flask)
// 3. Start collector (goroutine)
// 4. Start agent (goroutine, requires root)
// 5. Send 100 HTTP GET /ping from go_http_server to flask_app
// 6. time.Sleep(5 * time.Second)
// 7. GET http://localhost:8080/api/traces → assert len(traces) >= 90
// 8. GET http://localhost:8080/api/graph → assert edge {go_http_server → flask_app} exists
// 9. Assert: all traces have span_count >= 2
// 10. Assert: no trace has duration_ns > 1_000_000_000 (1 second — sanity check)
```

---

## 14. Benchmark Suite (`bench/`)

### 14.1 CPU Overhead (`overhead_test.go`)
```
Setup:    go_http_server handling 10_000 req/s (use wrk or custom goroutine loop)
Baseline: measure CPU% of target process for 60s WITHOUT agent
Test:     attach agent, measure CPU% of target process for 60s WITH agent
Assert:   delta <= 1.0%
Tool:     read /proc/<pid>/stat (utime + stime) before and after
```

### 14.2 Ring Buffer Throughput (`throughput_test.go`)
```
Send synthetic tcp_event_t structs directly to ring buffer at controlled rate
Measure: events read by Go poller per second
Assert:  no drops at 80_000 events/second on 4-core machine
```

### 14.3 Latency Impact
```
Measure p99 latency of HTTP requests to go_http_server:
  Baseline: 1000 requests without agent
  Test:     1000 requests with agent attached
  Assert:   p99 delta <= 50 microseconds
Tool:       record time.Since(start) for each request in bench harness
```

Output all results as JSON to `bench/results/latest.json`.

---

## 15. Implementation Order (Execute Sequentially — No Skipping)

### Phase 1 — Foundation (complete before writing any Go or C)
1. `git init`, add libbpf submodule, `git submodule update --init`
2. Create full directory tree (all directories, empty files with correct names)
3. Write `CLAUDE.md` with project goal, directory map, build commands, status: "Phase 1 in progress"
4. Write `proto/zerotrace.proto`
5. Run protoc to generate Go files → verify no errors
6. Write `agent/go.mod`, `collector/go.mod`, `go.work`
7. Write `bpf/headers/common.h`
8. Run `make deps` to copy libbpf headers
9. Run `make vmlinux` to generate vmlinux.h — abort if /sys/kernel/btf/vmlinux missing
10. Update `CLAUDE.md` status: "Phase 1 complete"

### Phase 2 — BPF Programs (all must pass verifier before Phase 3)
1. Write `tcp_trace.bpf.c` → `make bpf/tcp_trace.bpf.o` → fix all errors
2. Write `sched_trace.bpf.c` → compile → fix
3. Write `ssl_trace.bpf.c` → compile → fix
4. Write `http_trace.bpf.c` → compile → fix
5. Write `xdp_classifier.bpf.c` → compile → fix
6. Run `make bpf` → ALL programs must compile cleanly
7. Update `CLAUDE.md`

### Phase 3 — Agent Core
1. Write `//go:generate` directives in `loader/loader.go`
2. Run `make generate` → verify Go bindings generated
3. Write `config/config.go` (viper, all defaults)
4. Write `loader/loader.go` (BPF loading, probe attachment, Close() cleanup)
5. Write `reader/ringbuf.go`
6. Write `reader/event_parser.go`
7. Write `enricher/proc.go`
8. Write `enricher/dwarf.go`
9. Write `enricher/ssl_detector.go`
10. Write `correlator/correlator.go` + `request_tracker.go`
11. Write `exporter/grpc_exporter.go` + `otel_span.go`
12. Write `main.go`
13. `cd agent && go build ./...` → fix all compile errors
14. Smoke test: run agent against local process, verify zap logs show events
15. Update `CLAUDE.md`

### Phase 4 — Collector
1. Write `config/config.go` (viper, collector defaults)
2. Write `ingest/grpc_server.go`
3. Write `ingest/span_assembler.go`
4. Write `store/badger_store.go` + `index.go`
5. Write `api/rest.go` (all endpoints, CORS header)
6. Write `api/websocket.go`
7. Write `graph/dependency_graph.go`
8. Write `main.go`
9. `cd collector && go build ./...` → fix all errors
10. Smoke test: start collector, send test span via grpcurl, verify `/api/traces` returns it
11. Update `CLAUDE.md`

### Phase 5 — UI
1. `cd ui && npm create vite@latest . -- --template react-ts`
2. Install dependencies from Section 9.5
3. Write `global.css` with all design tokens
4. Write `reset.css`
5. Write `types/trace.ts` (mirror Go structs exactly)
6. Write `api/client.ts`
7. Write `stores/traceStore.ts`
8. Write `hooks/useTraces.ts` + `useLiveTail.ts`
9. Write all `layout/` components
10. Write `shared/` components (Badge, DurationBar, EmptyState)
11. Write `TraceLiveTable.tsx`
12. Write `TraceListTable.tsx`
13. Write `TraceTimeline.tsx` (D3 Gantt)
14. Write `SpanDetailDrawer.tsx`
15. Write `ServiceMap.tsx` (D3 force graph)
16. Wire routing in `App.tsx`
17. `npm run build` → fix all TypeScript errors
18. `npm run dev` → verify all pages render without console errors
19. Update `CLAUDE.md`

### Phase 6 — Tests + Benchmarks
1. Write all unit tests
2. Run `go test ./test/unit/...` → all pass
3. Write integration test
4. Run integration test (requires root + running target apps)
5. Write benchmark suite + target apps
6. Run benchmarks → write results to `bench/results/latest.json`
7. Update `CLAUDE.md`

### Phase 7 — Final Polish
1. Write `README.md` (all sections from Section 16)
2. Add code comments to all BPF C functions and Go correlator logic
3. Run `go vet ./...` in agent + collector → zero warnings
4. Run `npm run build` → zero TypeScript errors
5. Verify `make all` succeeds from clean checkout
6. Update `CLAUDE.md` status: "Complete"

---

## 16. README.md Required Sections

1. **What is ZeroTrace** — 3 sentences
2. **Architecture diagram** — ASCII art matching Section 2.1
3. **Prerequisites** — Linux kernel ≥ 5.15, clang ≥ 14, libbpf-dev, bpftool, Go 1.22+, Node 20+, root privileges
4. **Quick Start** — exactly 4 commands: clone → make all → start collector → start agent
5. **How It Works** — 500-word explanation of the eBPF probe lifecycle, ring buffer, correlation
6. **UI Screenshots** — placeholder text "See /screenshots directory" (no actual screenshots needed)
7. **Benchmark Results** — table with CPU overhead, throughput, latency impact (populated from bench/results/latest.json)
8. **Known Limitations** — IPv6 unsupported, kernel < 5.15 unsupported, macOS/Windows unsupported, new processes miss first 5s of SSL traces, non-HTTP TCP correlated at connection granularity
9. **FAQ** — "Does this require a kernel module?" (No), "Does it work with HTTPS?" (Yes, via SSL uprobes), "What is the performance overhead?" (< 1% CPU)

---

## 17. Success Criteria (All Must Pass)

- [ ] `make all` completes without errors on Ubuntu 22.04, kernel 5.15+
- [ ] Agent attaches to running Python Flask app, captures HTTP spans, zero Flask code changes
- [ ] Agent attaches to running Go HTTP server, captures spans
- [ ] Collector receives spans, `/api/traces` returns ≥ 1 complete multi-span trace
- [ ] `/api/graph` returns edge between test services
- [ ] UI Live Tail page shows incoming traces via WebSocket
- [ ] UI Service Map renders with correct nodes and colored edges
- [ ] UI Trace Timeline renders Gantt chart for a multi-span trace
- [ ] CPU overhead benchmark: ≤ 1% additional CPU
- [ ] All unit tests pass
- [ ] Integration test passes (≥ 90 traces captured out of 100 sent)
- [ ] Zero hardcoded IPs, ports, or paths — all from config with defaults
- [ ] `go vet ./...` reports zero issues
- [ ] `npm run build` reports zero TypeScript errors
- [ ] README complete with all 9 sections
- [ ] CLAUDE.md updated with final status
