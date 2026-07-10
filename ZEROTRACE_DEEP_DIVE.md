# ZEROTRACE_DEEP_DIVE

## Section 1 — What ZeroTrace Actually Does (Plain English)

ZeroTrace is a distributed tracing tool that automatically captures network requests and maps dependencies between microservices using eBPF, without requiring any changes to the application code. It listens to system-level events (TCP sends/receives, process executions) directly from the Linux kernel and reconstructs the flow of requests.

Capturing network traffic is easy, but attributing that traffic to specific services, tracking it across process boundaries, and securely reading encrypted data (like TLS) is extremely difficult. The kernel only sees raw bytes and IP addresses; reconstructing logical HTTP requests and linking them together into a unified trace requires complex correlation algorithms in userspace, all while operating under the strict performance and safety constraints of the eBPF verifier.

Traditional tools like Datadog, Jaeger, and OpenTelemetry typically rely on developers manually adding libraries (SDKs) to their code to inject tracking headers (like `traceparent`) into every outbound request. This requires modifying code, recompiling, and redeploying. ZeroTrace bypasses this entirely by using eBPF to inspect socket operations and SSL plaintext buffers directly in memory before they are encrypted or after they are decrypted, correlating requests based on timing and thread activity rather than injected headers.

## Section 2 — The Full Data Flow (Step by Step)

1. `curl` sends an HTTP request, which reaches the Linux kernel network stack.
2. The kernel executes `tcp_recvmsg()` to read the incoming packets into the Flask app's socket buffer.
3. The eBPF kprobe attached to `tcp_recvmsg` in `bpf/tcp_trace.bpf.c: kprobe__tcp_recvmsg()` fires, extracting the socket pointer, IP addresses, ports, and process ID (using CO-RE `BPF_CORE_READ_INTO`).
4. The eBPF program writes this raw `tcp_event_t` event into a high-performance `BPF_MAP_TYPE_RINGBUF` map named `events`.
5. In the agent, `agent/reader/ringbuf.go: PollRingBuffer()` continuously polls the ring buffer, reads the raw bytes, and calls `ParseEvent()` to deserialize it into a `reader.TCPEvent`.
6. The event is sent over a Go channel to `agent/correlator/correlator.go: Run()`, which routes it to `HandleTCPEvent()`.
7. `HandleTCPEvent()` creates or updates a `ConnectionState` object using a 4-tuple key (Source IP/Port, Dest IP/Port), tracks byte counts, and attempts cross-process linking using the `activePIDs` map based on thread activity.
8. Every 5 seconds, `FlushOldConnections()` converts idle connections into `proto.Span` objects and flushes them to a span channel.
9. `agent/exporter/grpc_exporter.go: Run()` batches the spans and sends them over the network via gRPC using `proto.TraceIngestClient.SendSpans()`.
10. The collector receives the batch in `collector/ingest/grpc_server.go: SendSpans()`, which passes them to `SpanAssembler.Ingest()` to group them into complete `TraceTree`s by `trace_id`.
11. `processTrees()` writes the completed trace to BadgerDB via `store.BadgerStore.WriteTrace()`, updates the dependency graph in `graph.DependencyGraph`, and broadcasts the trace to WebSocket clients.
12. The frontend React app receives the WebSocket message in `ui/src/stores/traceStore.ts: addLiveTrace()` and updates the `liveTraces` Zustand state, which triggers a re-render in the UI components to display the trace.

## Section 3 — Every Hard Problem and How We Solved It

- **BPF verifier compliance**: The BPF verifier strictly enforces bounded loops, memory safety, and program size. To comply, we minimized the logic inside the BPF C code (like in `tcp_trace.bpf.c` and `ssl_trace.bpf.c`). We only collect raw telemetry (socket pointers, IPs, bytes) and submit it to a ring buffer. All complex logic (HTTP parsing, correlation, string manipulation) was pushed to userspace in the Go agent (`correlator.go`).
- **Causal correlation without modifying the target app**: Linking a request coming into a service with a request leaving the same service without headers is challenging. `correlator.go` solves this using a time-based heuristic on thread activity. When a service receives data, `HandleTCPEvent()` stores the connection in an `activePIDs` map. When that same PID sends data shortly after (within a 2-second timeout window), `HandleTCPEvent` links the outgoing connection to the active incoming connection by copying the `TraceID` and setting the `ParentSpanID`.
- **Thread-based parent-child span linking**: The `activePIDs` map in `correlator.go` tracks which connection is currently "active" for a given Process ID (PID). When a `TCPRecv` occurs, it sets `activePIDs[ev.PID] = conn`. When a `TCPSend` occurs, it checks if `activePIDs[ev.PID]` exists and if the timestamp is within 2 billion nanoseconds (2s). If so, it links them. For localhost testing, it also correlates client/server sides of the same connection by tracking `reverseKey` (swapped IPs and ports).
- **Ring buffer backpressure**: If events arrive from the kernel faster than the userspace Go agent can read them, the ring buffer can drop events. We solved this by sizing the `BPF_MAP_TYPE_RINGBUF` to 256MB (`256 * 1024 * 1024` entries/bytes in `tcp_trace.bpf.c`). We also use a dedicated goroutine `PollRingBuffer()` that does nothing but read from the buffer and push to a buffered Go channel (`make(chan reader.RawEvent, 100000)`), decoupling the kernel read speed from the correlator's processing speed.
- **The proto package local resolution**: We needed both the `agent` and `collector` modules to share the same gRPC protobuf definitions without publishing a separate Go module to GitHub. We solved this by using Go workspaces (`go.work`) which points to `./agent`, `./collector`, and `./proto`. In the individual `go.mod` files, we also use the `replace github.com/zerotrace/zerotrace/proto => ../proto` directive to force local resolution during builds.

## Section 4 — Architecture Decisions and Why

- **Why BadgerDB instead of PostgreSQL or Redis**: BadgerDB is an embedded, pure-Go key-value store. We chose it because it avoids the operational overhead of running a separate database server, keeping the collector as a single lightweight binary. It supports native TTLs (`WithTTL`), which is perfect for trace retention, and it's highly optimized for SSDs.
- **Why gRPC instead of REST for agent→collector**: gRPC uses HTTP/2 and Protocol Buffers, providing persistent connections, multiplexing, and efficient binary serialization. For high-throughput trace telemetry streaming from the agent to the collector, gRPC streaming is significantly faster and uses less CPU/bandwidth than repeatedly opening HTTP/1.1 REST connections with JSON payloads.
- **Why bpf2go instead of loading BPF programs manually**: `bpf2go` automatically compiles the BPF C code and generates Go bindings (structs and load functions like `LoadTcpTraceObjects`). We chose it because it integrates seamlessly with `go generate`, embeds the compiled `.o` files directly into the Go binary (avoiding file path issues at runtime), and provides strongly-typed structs that match the C definitions.
- **Why CO-RE (BPF_CORE_READ) instead of raw pointer access**: Linux kernel structs (like `struct sock`) change offsets between kernel versions. Using raw pointer access would require compiling the BPF program on the target machine with the exact kernel headers. CO-RE (Compile Once, Run Everywhere) uses BTF (BPF Type Format) to automatically adjust struct offsets at runtime, allowing us to distribute a single pre-compiled agent binary that works across different kernel versions.
- **Why thread-based correlation instead of header injection**: Header injection requires modifying application source code, configuration, or restarting processes to inject agents. We chose thread-based correlation entirely in userspace to achieve true zero-instrumentation (eBPF). It allows us to observe unmodified, running binaries instantly.
- **Why Zustand instead of Redux for UI state**: Zustand is much lighter, has less boilerplate, and uses a simple hook-based API (`useTraceStore`). For our needs—tracking live traces, WebSocket status, and historical traces—Redux's actions and reducers were unnecessary overhead.
- **Why no external component library in the UI**: We chose Vanilla CSS (in `index.css` and `App.css`) and plain React components to maintain complete control over the design, ensuring a highly custom, dark-mode, glassmorphism aesthetic without fighting the default styles of component libraries like Material-UI or Tailwind CSS.

## Section 5 — What Each File Does (File-by-File Reference)

### agent/
- `main.go`: The entry point for the agent. Initializes config, BPF loader, ring buffer poller, correlator, exporter, and uprobe scanner, tying the concurrent pipelines together.
- `config/config.go`: Uses Viper to parse the `agent.yaml` configuration and provides default fallback values.
- `correlator/correlator.go`: The core userspace logic. Tracks TCP connections, parses HTTP methods from plaintext, links spans causally using PID/thread heuristics, and flushes idle connections to the exporter.
- `correlator/helpers.go`: Contains utilities for cryptographic ID generation and network byte-order swapping.
- `enricher/boottime.go`, `dwarf.go`, `proc.go`: Utilities to read `/proc` files, translate monotonic kernel timestamps to Unix time, and extract executable paths for attaching uprobes.
- `exporter/grpc_exporter.go`: Manages the gRPC connection to the collector, batching outgoing `proto.Span` objects and handling reconnects.
- `loader/loader.go`: Uses `cilium/ebpf` to load BPF programs, attach kprobes/tracepoints, create the ring buffer, and dynamically scan for processes using `libssl` to attach uprobes.
- `reader/event_parser.go`: Inspects raw byte slices from the ring buffer, probes offsets to determine event type, and uses `binary.Read` to deserialize C structs into Go structs.
- `reader/ringbuf.go`: Runs a tight loop reading records from the BPF ring buffer and sending them to the `RawEvent` channel.
- `reader/types.go`: Defines the Go structs that mirror the C BPF structs for event deserialization.

### collector/
- `main.go`: Entry point for the collector. Initializes BadgerDB, dependency graph, WebSocket hub, REST API, and gRPC server.
- `api/rest.go`: Exposes HTTP endpoints (`/api/traces`, `/api/services`) to query the BadgerDB store and dependency graph.
- `api/websocket.go`: Manages real-time WebSocket connections, broadcasting new trace batches to connected UI clients.
- `config/config.go`: Parses `collector.yaml` via Viper for HTTP, gRPC, and storage settings.
- `graph/dependency_graph.go`: Maintains an in-memory directed graph of service dependencies and edge latencies based on parent-child span relationships.
- `ingest/grpc_server.go`: Implements the `TraceIngestServer` gRPC interface, receiving spans from agents, sending them to the assembler, and dispatching completed trees to storage and websockets.
- `ingest/span_assembler.go`: Buffers incoming spans in memory, grouping them by `TraceID` until a root span is found or a flush is triggered, forming complete `TraceTree`s.
- `store/badger_store.go`: Wraps BadgerDB to persist spans and index traces by root service, handling JSON serialization and TTL expiration.
- `store/index.go`: Provides a simple in-memory bounded LRU-like index to quickly list recent trace IDs.

### bpf/
- `tcp_trace.bpf.c`: BPF program attaching to `tcp_sendmsg` and `tcp_recvmsg` kprobes to capture network 4-tuples and byte counts, emitting `EVENT_TCP_SEND` and `EVENT_TCP_RECV`.
- `ssl_trace.bpf.c`: BPF program attaching uprobes to `SSL_read` and `SSL_write`. Uses a BPF hash map to stash buffer pointers on function entry and reads the unencrypted plaintext on function return.
- `sched_trace.bpf.c`: BPF program attaching to `sched_process_exec` and `sched_process_exit` tracepoints to track process lifecycles and executable names.
- `http_trace.bpf.c`: A placeholder/stub file (just an `xdp_pass`), as HTTP parsing was pushed to userspace in `correlator.go`.
- `xdp_classifier.bpf.c`: A minimal XDP stub program that just passes packets.

### ui/
- `src/App.tsx`: The main React layout container, routing between the Live traces, Historical traces, and Service Map views.
- `src/components/ServiceMap.tsx`: Uses D3.js to render a visual directed graph of service dependencies.
- `src/components/TraceDetail.tsx`: Renders a flame graph/waterfall view for a single selected trace tree.
- `src/components/TraceListTable.tsx`: Displays paginated historical traces fetched via REST.
- `src/components/TraceLiveTable.tsx`: Displays a scrolling list of real-time traces via WebSocket.
- `src/stores/traceStore.ts`: The Zustand state store holding WebSocket connection status, live trace buffer, and historical data.

## Section 6 — Interview Questions and Answers

1. **Q: How do you correlate a request across two services without modifying either service?**
   A: We use a time-based heuristic on thread activity. When a service receives an incoming TCP request, `correlator.go` tracks the connection as active for that PID in an `activePIDs` map. If that same PID initiates an outbound TCP request within a short timeout window (2 seconds), the correlator links them by copying the incoming connection's TraceID and setting it as the ParentSpanID for the outgoing connection.

2. **Q: Why use eBPF for this instead of a proxy like Envoy?**
   A: eBPF runs in the kernel and instruments the socket layer directly, meaning it works transparently without changing network routing, IP tables, or deploying sidecar containers. This drastically reduces operational complexity and overhead.

3. **Q: How did you capture encrypted HTTPS traffic?**
   A: We used eBPF uprobes (user-space probes). A scanner in `loader.go` periodically finds processes using `libssl.so`, resolves the memory offsets of `SSL_read` and `SSL_write`, and dynamically attaches uprobes. The probes stash the plaintext buffer pointer on function entry and read the decrypted data on function return (`uretprobe`), sending it to userspace before encryption or after decryption.

4. **Q: What is the BPF verifier and how did you satisfy it?**
   A: The BPF verifier ensures kernel safety by checking that BPF programs terminate, don't access invalid memory, and are within size limits. We satisfied it by keeping our BPF C code extremely simple—just copying raw struct fields and submitting to a ring buffer—and doing all complex looping and string parsing (like HTTP headers) in the Go userspace.

5. **Q: What is CO-RE and why is it important in your implementation?**
   A: CO-RE stands for Compile Once, Run Everywhere. It uses BPF Type Format (BTF) to dynamically adjust struct offsets (like `struct sock`) when the BPF program loads. We used `BPF_CORE_READ_INTO` in our C code, which allowed us to distribute a single pre-compiled agent that works across different Linux kernel versions without needing LLVM installed on the target machine.

6. **Q: How do you prevent the kernel from dropping events if the agent is too slow?**
   A: We used a `BPF_MAP_TYPE_RINGBUF` sized to 256MB. Unlike older perf buffers, the ring buffer is shared across all CPUs, reducing memory overhead. We also decoupled reading from processing by having `PollRingBuffer` immediately push raw bytes to a buffered Go channel (`size 100,000`), ensuring the ring buffer is drained as fast as possible.

7. **Q: Why did you use gRPC bidirectional streaming for ingest?**
   A: gRPC multiplexes over a single HTTP/2 connection. Instead of creating a new HTTP request for every trace batch, the agent keeps a persistent stream open and pushes binary Protobuf messages. This significantly lowers CPU usage and latency compared to a REST API.

8. **Q: How does the collector assemble partial spans into a full trace?**
   A: The `SpanAssembler` buffers incoming spans in a map keyed by TraceID. When a span with an empty `ParentSpanId` arrives (the root span), the assembler considers the trace tree complete, sorts the spans by timestamp, and flushes the entire `TraceTree` to BadgerDB and the WebSocket hub.

9. **Q: How does BadgerDB handle data retention and pruning?**
   A: BadgerDB supports native key TTLs (Time-To-Live). When writing a span in `store/badger_store.go`, we append `.WithTTL(ttl)`. Badger automatically drops expired keys during compaction. We also run a background goroutine in the collector to trigger `RunValueLogGC` periodically to reclaim disk space.

10. **Q: How did you implement cross-process linking for localhost testing?**
    A: In `correlator.go`, we create a `connKey` from the 4-tuple. When a new connection is seen, we check if the `reverseKey` (swapped src/dst IPs and ports) already exists in the connection map. If it does, we immediately link the client and server sides of the loopback connection by sharing the TraceID.

11. **Q: What is the overhead of running ZeroTrace?**
    A: The overhead is very low. eBPF kprobes execute in nanoseconds within the kernel, and copying data to the ring buffer avoids context switches. The heaviest part is SSL plaintext capture due to uprobe context switches, but since we only read small chunks to identify HTTP methods, the impact is minimal.

12. **Q: Why did you write your own parsing logic for C structs in Go?**
    A: Because the ring buffer delivers raw byte slices containing C structs (like `tcp_event_t`). In `event_parser.go`, we probe specific byte offsets to determine the event type, then use `binary.Read` with LittleEndian layout to safely deserialize the packed C structs into typed Go structs without relying on cgo bridging.

13. **Q: How do you handle dynamically loaded libraries for SSL tracing?**
    A: `loader.go` runs a goroutine that periodically scans `/proc/[pid]/maps` for `libssl.so`. When a new process is found, it calculates the dynamic offsets of `SSL_read`/`SSL_write` and attaches uprobes specific to that PID and library path.

14. **Q: What happens if the gRPC connection to the collector drops?**
    A: `grpc_exporter.go` checks the connection state before sending a batch. If the state is `TransientFailure` or `Shutdown`, it triggers `reconnect()` to dial a fresh connection, while spans remain buffered in the Go channel until the batch succeeds or the channel fills up.

15. **Q: What are the limitations of WSL for this project?**
    A: WSL2 uses a custom Microsoft-compiled Linux kernel. While it supports eBPF, certain advanced features or older kernel BTF configs might be missing. We also noticed that tracing loopback traffic on WSL sometimes requires special network namespaces or skips hardware offloading paths.

16. **Q: How did you organize the frontend to handle real-time data?**
    A: We used Zustand for state management (`traceStore.ts`). A WebSocket connection pushes new trace batches. The store prepends these to a `liveTraces` array, capped at 200 items. We added a "Pause" toggle so the UI doesn't jump around while the user is inspecting a live trace.

17. **Q: Why is there a `replace` directive in your `go.mod`?**
    A: The project uses a multi-module workspace. The `proto` package is a separate directory. The `replace` directive allows the `agent` and `collector` modules to import `github.com/zerotrace/zerotrace/proto` but resolve it to the local `../proto` path on disk without needing to publish it to a remote repository.

18. **Q: How does the Service Map generate its graph?**
    A: The collector's `DependencyGraph` listens for completed traces. It iterates through the spans, looking at `ParentSpanId`. When it finds a parent-child relationship between two spans, it extracts their `ServiceName`s and adds a directed edge, recording the latency and error status. The frontend uses D3.js to render this node-link graph.

19. **Q: What is the biggest flaw in your correlation algorithm?**
    A: The time-based heuristic (assuming any outgoing request within 2 seconds of an incoming request is related) breaks down under high concurrency. If a Node.js or Go service handles 1,000 requests per second concurrently on the same threads, the correlator will mislink spans.

20. **Q: How would you fix the correlation issue at production scale?**
    A: We would need to implement deep protocol inspection (e.g., matching HTTP request bodies/IDs), or use eBPF to track thread-local storage (TLS) or Go routine IDs specifically, to trace exact execution contexts rather than relying on broad time windows.

## Section 7 — Known Limitations and How to Fix Them

- **Single-span traces (no HTTP-level parsing yet)**:
  - *What it is*: The agent captures TCP connections as spans but doesn't fully split multiplexed HTTP/1.1 requests or HTTP/2 streams into distinct child spans.
  - *Why it exists*: Parsing HTTP boundaries inside eBPF is verifier-hostile, and doing it in userspace requires reassembling TCP streams, which is highly complex.
  - *Production fix*: Implement a full TCP stream reassembler in userspace (like `gopacket`) and an HTTP state machine to emit individual spans per HTTP request.

- **SSL uprobes stubbed out**:
  - *What it is*: While the uprobe attachment logic exists in `loader.go` and `ssl_trace.bpf.c`, parsing HTTP methods from the raw decrypted buffer relies on simple string matching which fails for fragmented TLS records.
  - *Why it exists*: Proper TLS payload reconstruction requires tracking OpenSSL state contexts.
  - *Production fix*: Hook into OpenSSL's internal BIO structure to read complete, unencrypted HTTP buffers accurately.

- **Service map showing internal processes**:
  - *What it is*: The service map often displays `agent`, `collector`, or `systemd` alongside actual application services.
  - *Why it exists*: `correlator.go` uses the process `comm` name as the service name. We have basic filters in config, but they don't catch everything.
  - *Production fix*: Enhance the filtering logic, allow regex exclusions, or cross-reference PIDs with container IDs (via cgroups) to only track target applications.

- **WSL kernel limitations vs bare metal Linux**:
  - *What it is*: eBPF behavior on WSL2 can be inconsistent, especially with local loopback traffic bridging between Windows and the Linux VM.
  - *Why it exists*: WSL2 uses a paravirtualized network adapter.
  - *Production fix*: Run the agent on a native Linux VM or Kubernetes cluster for accurate production telemetry.

- **No persistence beyond 24h**:
  - *What it is*: BadgerDB uses a strict TTL, dropping all data after 24 hours.
  - *Why it exists*: To prevent the collector from running out of disk space on a single node.
  - *Production fix*: Replace BadgerDB with a distributed OLAP database like ClickHouse for long-term telemetry storage.

- **Timestamps were in nanoseconds (already fixed)**:
  - *What it is*: eBPF `bpf_ktime_get_ns()` returns monotonic uptime, which didn't align with Unix timestamps, causing UI display issues.
  - *Why it exists*: Kernel events don't use wall-clock time.
  - *Production fix*: We already fixed this in `enricher.MonotonicToUnixNs()` by calculating the boot time offset at startup and adding it to the kernel timestamps.

## Section 8 — How to Run It (Exact Commands)

The exact sequence of commands to start the full stack from a clean WSL terminal.

**1. Start the Collector**
```bash
cd /mnt/d/zerotrace/collector
go mod tidy
go run main.go
```
*(Keep this terminal open)*

**2. Start the Agent (requires root for eBPF)**
Open a new terminal:
```bash
cd /mnt/d/zerotrace/agent
go mod tidy
sudo -E go run main.go
```
*(Keep this terminal open)*

**3. Start the UI**
Open a new terminal:
```bash
cd /mnt/d/zerotrace/ui
npm install
npm run dev
```
*(Open http://localhost:5173 in your browser)*

**4. Generate Traffic (Test)**
Open a new terminal:
```bash
curl http://example.com
```
You should immediately see the trace appear in the UI's Live Traces view.

### Troubleshooting

- **Error: "config load failed"**: Make sure you run the agent/collector from their respective directories (`cd agent/` or `cd collector/`) so they can find their configuration file at `deploy/configs/agent.yaml` / `deploy/configs/collector.yaml`.
- **Error: "failed to load BPF objects"**: Ensure `clang` is installed and you are running the agent with `sudo` because eBPF requires root privileges.
- **Error: "port 4317 already in use"**: The collector is already running, or another OpenTelemetry tool is using the default gRPC port. Kill it with `sudo fuser -k 4317/tcp`.
