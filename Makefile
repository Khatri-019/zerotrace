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
