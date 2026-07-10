package loader

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"go.uber.org/zap"

	"github.com/zerotrace/zerotrace/agent/config"
	"github.com/zerotrace/zerotrace/agent/enricher"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" TcpTrace ../../bpf/tcp_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" SslTrace ../../bpf/ssl_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" SchedTrace ../../bpf/sched_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" HttpTrace ../../bpf/http_trace.bpf.c -- -I../../bpf/headers
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -D__TARGET_ARCH_x86 -I../../bpf/headers" XdpClassifier ../../bpf/xdp_classifier.bpf.c -- -I../../bpf/headers

// Manager owns all eBPF objects and links.
type Manager struct {
	tcpObjs   TcpTraceObjects
	sslObjs   SslTraceObjects
	schedObjs SchedTraceObjects
	links     []link.Link
	// uprobeLinks tracks SSL uprobe links keyed by PID so we avoid duplicates.
	uprobeLinks map[int][]link.Link
	failedLibs  map[string]bool
	ringBuf     *ringbuf.Reader
	log         *zap.Logger
}

// New loads eBPF programs, attaches kernel probes and tracepoints, and
// creates the ring buffer reader.
func New(cfg *config.Config, log *zap.Logger) (*Manager, error) {
	m := &Manager{
		log:         log,
		uprobeLinks: make(map[int][]link.Link),
		failedLibs:  make(map[string]bool),
	}

	// ── TCP kprobes ──────────────────────────────────────────────────────────
	if cfg.Probes.TCP {
		log.Info("loading TCP trace BPF program")
		if err := LoadTcpTraceObjects(&m.tcpObjs, nil); err != nil {
			return nil, fmt.Errorf("loading TCP BPF objects: %w", err)
		}

		sendLink, err := link.Kprobe("tcp_sendmsg", m.tcpObjs.KprobeTcpSendmsg, nil)
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("attaching kprobe tcp_sendmsg: %w", err)
		}
		m.links = append(m.links, sendLink)
		log.Info("attached kprobe", zap.String("probe", "tcp_sendmsg"))

		recvLink, err := link.Kprobe("tcp_recvmsg", m.tcpObjs.KprobeTcpRecvmsg, nil)
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("attaching kprobe tcp_recvmsg: %w", err)
		}
		m.links = append(m.links, recvLink)
		log.Info("attached kprobe", zap.String("probe", "tcp_recvmsg"))
	}

	// ── SSL trace objects (uprobes attached dynamically per-process) ──────────
	if cfg.Probes.SSL {
		log.Info("loading SSL trace BPF program")
		if err := LoadSslTraceObjects(&m.sslObjs, nil); err != nil {
			m.Close()
			return nil, fmt.Errorf("loading SSL BPF objects: %w", err)
		}
	}

	// ── Sched tracepoints ────────────────────────────────────────────────────
	if cfg.Probes.Process {
		log.Info("loading sched trace BPF program")
		if err := LoadSchedTraceObjects(&m.schedObjs, nil); err != nil {
			m.Close()
			return nil, fmt.Errorf("loading sched BPF objects: %w", err)
		}

		execLink, err := link.Tracepoint("sched", "sched_process_exec",
			m.schedObjs.TracepointSchedProcessExec, nil)
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("attaching tracepoint sched_process_exec: %w", err)
		}
		m.links = append(m.links, execLink)
		log.Info("attached tracepoint", zap.String("probe", "sched_process_exec"))

		exitLink, err := link.Tracepoint("sched", "sched_process_exit",
			m.schedObjs.TracepointSchedProcessExit, nil)
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("attaching tracepoint sched_process_exit: %w", err)
		}
		m.links = append(m.links, exitLink)
		log.Info("attached tracepoint", zap.String("probe", "sched_process_exit"))
	}

	// ── Ring buffer reader — attached to TCP events map ───────────────────────
	rd, err := ringbuf.NewReader(m.tcpObjs.Events)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("creating ring buffer reader: %w", err)
	}
	m.ringBuf = rd
	log.Info("ring buffer reader created")

	return m, nil
}

// RingBufReader returns the ring buffer reader for the event poll loop.
func (m *Manager) RingBufReader() *ringbuf.Reader {
	return m.ringBuf
}

// Close releases all eBPF resources in reverse-attachment order.
func (m *Manager) Close() {
	if m.ringBuf != nil {
		m.ringBuf.Close()
	}

	// Release per-PID uprobe links
	for pid, ls := range m.uprobeLinks {
		for _, l := range ls {
			if l != nil {
				l.Close()
			}
		}
		delete(m.uprobeLinks, pid)
	}

	for _, l := range m.links {
		if l != nil {
			l.Close()
		}
	}

	m.tcpObjs.Close()
	m.sslObjs.Close()
	m.schedObjs.Close()

	if m.log != nil {
		m.log.Info("all BPF resources released")
	}
}

// ---------------------------------------------------------------------------
// SSL uprobe scanner
// ---------------------------------------------------------------------------

// ScanAndAttachUprobes periodically scans /proc for processes that have
// libssl.so mapped and attaches SSL uprobes to any new ones found.
func ScanAndAttachUprobes(ctx context.Context, m *Manager, cfg *config.Config, log *zap.Logger) {
	if !cfg.Probes.SSL {
		log.Info("SSL probing disabled, uprobe scanner not started")
		<-ctx.Done()
		return
	}

	log.Info("SSL uprobe scanner started")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Run immediately on startup, then every tick.
	m.scanAndAttach(log)

	for {
		select {
		case <-ctx.Done():
			log.Info("SSL uprobe scanner stopped")
			return
		case <-ticker.C:
			m.scanAndAttach(log)
		}
	}
}

// scanAndAttach finds all processes using libssl and attaches uprobes if not
// already done.
func (m *Manager) scanAndAttach(log *zap.Logger) {
	pids, err := enricher.GetOpenSSLProcList()
	if err != nil {
		log.Warn("uprobe scanner: failed to list SSL processes", zap.Error(err))
		return
	}

	for _, pid := range pids {
		if _, alreadyAttached := m.uprobeLinks[pid]; alreadyAttached {
			continue
		}

		libPath, err := enricher.GetOpenSSLLibPath(pid)
		if err != nil {
			log.Debug("uprobe scanner: no libssl for pid", zap.Int("pid", pid), zap.Error(err))
			continue
		}

		links, err := m.attachSSLUprobes(pid, libPath)
		if err != nil {
			// Only warn once per library path to avoid log spam every 10s.
			if !m.failedLibs[libPath] {
				log.Warn("SSL uprobe attach failed (will not retry this lib)",
					zap.Int("pid", pid),
					zap.String("lib", libPath),
					zap.Error(err))
				m.failedLibs[libPath] = true
			}
			continue
		}

		m.uprobeLinks[pid] = links
		log.Info("SSL uprobes attached",
			zap.Int("pid", pid),
			zap.String("lib", libPath),
		)
	}

	// Clean up links for processes that have exited.
	m.cleanDeadPIDs(log)
}

// attachSSLUprobes attaches entry/return uprobes for SSL_write and SSL_read
// to the given PID using the specified libssl.so path.
func (m *Manager) attachSSLUprobes(pid int, libPath string) ([]link.Link, error) {
	ex, err := link.OpenExecutable(libPath)
	if err != nil {
		return nil, fmt.Errorf("open executable %s: %w", libPath, err)
	}

	sslWriteOffset, err := enricher.FindFunctionOffset(libPath, "SSL_write")
	if err != nil {
		return nil, fmt.Errorf("find SSL_write offset: %w", err)
	}

	sslReadOffset, err := enricher.FindFunctionOffset(libPath, "SSL_read")
	if err != nil {
		return nil, fmt.Errorf("find SSL_read offset: %w", err)
	}

	var links []link.Link

	wEntry, err := ex.Uprobe("SSL_write", m.sslObjs.UprobeSSL_write, &link.UprobeOptions{
		PID:    pid,
		Offset: sslWriteOffset,
	})
	if err != nil {
		return nil, fmt.Errorf("uprobe SSL_write entry: %w", err)
	}
	links = append(links, wEntry)

	wRet, err := ex.Uretprobe("SSL_write", m.sslObjs.UretprobeSSL_write, &link.UprobeOptions{
		PID:    pid,
		Offset: sslWriteOffset,
	})
	if err != nil {
		closeLinks(links)
		return nil, fmt.Errorf("uretprobe SSL_write: %w", err)
	}
	links = append(links, wRet)

	rEntry, err := ex.Uprobe("SSL_read", m.sslObjs.UprobeSSL_read, &link.UprobeOptions{
		PID:    pid,
		Offset: sslReadOffset,
	})
	if err != nil {
		closeLinks(links)
		return nil, fmt.Errorf("uprobe SSL_read entry: %w", err)
	}
	links = append(links, rEntry)

	rRet, err := ex.Uretprobe("SSL_read", m.sslObjs.UretprobeSSL_read, &link.UprobeOptions{
		PID:    pid,
		Offset: sslReadOffset,
	})
	if err != nil {
		closeLinks(links)
		return nil, fmt.Errorf("uretprobe SSL_read: %w", err)
	}
	links = append(links, rRet)

	return links, nil
}

// cleanDeadPIDs releases uprobe links for PIDs that no longer exist.
func (m *Manager) cleanDeadPIDs(log *zap.Logger) {
	for pid, links := range m.uprobeLinks {
		if !pidExists(pid) {
			for _, l := range links {
				l.Close()
			}
			delete(m.uprobeLinks, pid)
			log.Debug("released uprobes for dead pid", zap.Int("pid", pid))
		}
	}
}

// pidExists returns true if /proc/<pid> directory exists.
func pidExists(pid int) bool {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

// closeLinks is a helper to close a slice of links on error paths.
func closeLinks(links []link.Link) {
	for _, l := range links {
		if l != nil {
			l.Close()
		}
	}
}
