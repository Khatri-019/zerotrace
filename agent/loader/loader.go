package loader

import (
	"context"
	"go.uber.org/zap"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/zerotrace/zerotrace/agent/config"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" TcpTrace ../../bpf/tcp_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" SslTrace ../../bpf/ssl_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" SchedTrace ../../bpf/sched_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" HttpTrace ../../bpf/http_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" XdpClassifier ../../bpf/xdp_classifier.bpf.c -- -I../../bpf/headers

type Manager struct {
	tcpObjs   TcpTraceObjects
	sslObjs   SslTraceObjects
	schedObjs SchedTraceObjects
	links     []link.Link
	ringBuf   *ringbuf.Reader
	log       *zap.Logger
}

func New(cfg *config.Config, log *zap.Logger) (*Manager, error) {
	m := &Manager{log: log}
	return m, nil
}

func (m *Manager) RingBufReader() *ringbuf.Reader {
	return m.ringBuf
}

func (m *Manager) Close() {
	if m.ringBuf != nil {
		m.ringBuf.Close()
	}
	for _, l := range m.links {
		l.Close()
	}
	m.tcpObjs.Close()
	m.sslObjs.Close()
	m.schedObjs.Close()
	m.log.Info("all BPF resources released")
}

func ScanAndAttachUprobes(ctx context.Context, m *Manager, cfg *config.Config, log *zap.Logger) {
}
