package reader

import "fmt"

// EventType constants mirror common.h defines.
const (
	EventTCPSend  uint8 = 1
	EventTCPRecv  uint8 = 2
	EventSSLWrite uint8 = 3
	EventSSLRead  uint8 = 4
	EventProcExec uint8 = 5
	EventProcExit uint8 = 6
	EventHTTPReq  uint8 = 7
	EventHTTPResp uint8 = 8
)

const (
	MaxDataSize  = 256
	TaskCommLen  = 16
	MaxPathLen   = 128
	MaxMethodLen = 8
	MaxHostLen   = 64
)

// TCPEvent mirrors struct tcp_event_t from common.h.
// Field order and sizes must exactly match the C struct.
//
//	struct tcp_event_t {
//	    __u64 timestamp_ns;   // 8
//	    __u32 pid;            // 4
//	    __u32 tid;            // 4
//	    __u64 sk_ptr;         // 8
//	    __u32 bytes;          // 4
//	    __u16 sport;          // 2
//	    __u16 dport;          // 2
//	    __u8  saddr[4];       // 4
//	    __u8  daddr[4];       // 4
//	    __u8  event_type;     // 1
//	    char  comm[16];       // 16  (TASK_COMM_LEN)
//	};                        // Total: 57 bytes (+ padding → 64)
type TCPEvent struct {
	TimestampNs uint64
	PID         uint32
	TID         uint32
	SkPtr       uint64
	Bytes       uint32
	Sport       uint16
	Dport       uint16
	Saddr       [4]uint8
	Daddr       [4]uint8
	EventType   uint8
	Comm        [TaskCommLen]byte
}

// SSLEvent mirrors struct ssl_event_t from common.h.
//
//	struct ssl_event_t {
//	    __u64 timestamp_ns;   // 8
//	    __u32 pid;            // 4
//	    __u32 tid;            // 4
//	    __u64 ssl_ptr;        // 8
//	    __u32 bytes;          // 4
//	    __u8  event_type;     // 1
//	    char  comm[16];       // 16
//	    char  data[256];      // 256
//	};
type SSLEvent struct {
	TimestampNs uint64
	PID         uint32
	TID         uint32
	SSLPtr      uint64
	Bytes       uint32
	EventType   uint8
	Comm        [TaskCommLen]byte
	Data        [MaxDataSize]byte
}

// ProcEvent mirrors struct proc_event_t from common.h.
//
//	struct proc_event_t {
//	    __u64 timestamp_ns;   // 8
//	    __u32 pid;            // 4
//	    __u32 ppid;           // 4
//	    __u8  event_type;     // 1
//	    char  comm[16];       // 16
//	    char  filename[128];  // 128
//	    __u32 exit_code;      // 4
//	};
type ProcEvent struct {
	TimestampNs uint64
	PID         uint32
	PPID        uint32
	EventType   uint8
	Comm        [TaskCommLen]byte
	Filename    [MaxPathLen]byte
	ExitCode    uint32
}

// RawEvent is an enriched event after parsing from ring buffer bytes.
type RawEvent struct {
	// Kind is one of the EventXxx constants.
	Kind uint8
	// Raw is the original byte slice (kept for debugging).
	Raw []byte
	// Exactly one of the following is populated, based on Kind.
	TCP  *TCPEvent
	SSL  *SSLEvent
	Proc *ProcEvent
}

// CommString returns the null-terminated comm field as a Go string.
func CommString(comm [TaskCommLen]byte) string {
	for i, b := range comm {
		if b == 0 {
			return string(comm[:i])
		}
	}
	return string(comm[:])
}

// Addr4String converts a 4-byte IPv4 address to dotted-decimal notation.
func Addr4String(addr [4]uint8) string {
	return fmt.Sprintf("%d.%d.%d.%d", addr[0], addr[1], addr[2], addr[3])
}
