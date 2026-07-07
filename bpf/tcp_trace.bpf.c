// SPDX-License-Identifier: Dual BSD/GPL
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>
#include "common.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024 * 1024);
} events SEC(".maps");

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
