CLANG    := clang
LLC      := llc
BPFTOOL  := bpftool
AGENT_DIR     := agent
COLLECTOR_DIR := collector
UI_DIR    := ui
BPF_DIR   := bpf
HEADERS   := $(BPF_DIR)/headers
VMLINUX   := $(HEADERS)/vmlinux.h
BPF_SRCS  := $(wildcard $(BPF_DIR)/*.bpf.c)
BPF_OBJS  := $(patsubst %.bpf.c,%.bpf.o,$(BPF_SRCS))

.PHONY: all bpf agent collector ui generate test test-unit test-collector lint clean

# ── Full build ────────────────────────────────────────────────────────────────
all: deps vmlinux bpf generate agent collector ui

# ── Dependencies (libbpf headers) ─────────────────────────────────────────────
deps:
	git submodule update --init --recursive
	cp vendor/libbpf/src/bpf_helpers.h     $(HEADERS)/
	cp vendor/libbpf/src/bpf_helper_defs.h $(HEADERS)/
	cp vendor/libbpf/src/bpf_tracing.h     $(HEADERS)/
	cp vendor/libbpf/src/bpf_core_read.h   $(HEADERS)/

# ── vmlinux.h from running kernel ────────────────────────────────────────────
vmlinux: $(VMLINUX)
$(VMLINUX):
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > $@

# ── eBPF C programs ───────────────────────────────────────────────────────────
$(BPF_DIR)/%.bpf.o: $(BPF_DIR)/%.bpf.c $(VMLINUX)
	$(CLANG) -g -O2 -target bpf \
		-D__TARGET_ARCH_x86 \
		-I$(HEADERS) \
		-Wall -Wno-unused-variable \
		-c $< -o $@

bpf: $(BPF_OBJS)

# ── bpf2go codegen ───────────────────────────────────────────────────────────
generate:
	cd $(AGENT_DIR) && go generate ./loader/...

# ── Go builds ─────────────────────────────────────────────────────────────────
agent: generate
	mkdir -p bin
	cd $(AGENT_DIR) && go build -o ../bin/zerotrace-agent .

collector:
	mkdir -p bin
	cd $(COLLECTOR_DIR) && go build -o ../bin/zerotrace-collector .

# ── UI build ──────────────────────────────────────────────────────────────────
ui:
	cd $(UI_DIR) && npm install && npm run build

# ── Tests ─────────────────────────────────────────────────────────────────────
test: test-unit test-collector

# Unit tests: correlator, event parser (no BPF kernel required)
test-unit:
	cd test/unit && go test ./... -v -count=1

# Collector tests: store, ingest, graph (no BPF kernel required)
test-collector:
	cd $(COLLECTOR_DIR) && go test ./store/... ./ingest/... ./graph/... -v -count=1

# TypeScript type-check
test-ui:
	cd $(UI_DIR) && npx tsc --noEmit

# All tests
test-all: test test-ui

# ── Linting ───────────────────────────────────────────────────────────────────
lint:
	cd $(AGENT_DIR) && go vet ./...
	cd $(COLLECTOR_DIR) && go vet ./...
	cd $(UI_DIR) && npx oxlint --deny-warnings .

# ── Benchmarks ────────────────────────────────────────────────────────────────
bench:
	cd $(AGENT_DIR) && sudo go test ./bench/... -bench=. -benchtime=60s -v

# ── Clean ─────────────────────────────────────────────────────────────────────
clean:
	rm -f $(BPF_DIR)/*.bpf.o
	rm -f bin/*
	rm -rf $(VMLINUX)
	cd $(UI_DIR) && rm -rf dist node_modules
