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

struct ssl_write_args_t {
    __u64 ssl_ptr;
    __u64 buf_ptr;
    __u32 num;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u64);    // (pid << 32 | tid)
    __type(value, struct ssl_write_args_t);
} ssl_args SEC(".maps");

// Uprobe on SSL_write entry: stash (SSL*, buf, num) for use at return
SEC("uprobe/SSL_write")
int uprobe__SSL_write(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct ssl_write_args_t args = {};
    args.ssl_ptr = PT_REGS_PARM1(ctx);
    args.buf_ptr = PT_REGS_PARM2(ctx);
    args.num = PT_REGS_PARM3(ctx);
    bpf_map_update_elem(&ssl_args, &pid_tgid, &args, BPF_ANY);
    return 0;
}

// Uretprobe on SSL_write: read plaintext buffer
SEC("uretprobe/SSL_write")
int uretprobe__SSL_write(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct ssl_write_args_t *args = bpf_map_lookup_elem(&ssl_args, &pid_tgid);
    if (!args) return 0;

    struct ssl_event_t *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;

    event->timestamp_ns = bpf_ktime_get_ns();
    event->pid = pid_tgid >> 32;
    event->tid = (u32)pid_tgid;
    event->ssl_ptr = args->ssl_ptr;
    event->event_type = EVENT_SSL_WRITE;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    // Read buf argument
    void *buf = (void *)args->buf_ptr;
    bpf_probe_read_user(event->data, sizeof(event->data), buf);
    event->bytes = args->num;

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&ssl_args, &pid_tgid);
    return 0;
}

// Uprobe on SSL_read entry
SEC("uprobe/SSL_read")
int uprobe__SSL_read(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct ssl_write_args_t args = {};
    args.ssl_ptr = PT_REGS_PARM1(ctx);
    args.buf_ptr = PT_REGS_PARM2(ctx);
    args.num = PT_REGS_PARM3(ctx);
    bpf_map_update_elem(&ssl_args, &pid_tgid, &args, BPF_ANY);
    return 0;
}

// Uretprobe on SSL_read
SEC("uretprobe/SSL_read")
int uretprobe__SSL_read(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    struct ssl_write_args_t *args = bpf_map_lookup_elem(&ssl_args, &pid_tgid);
    if (!args) return 0;

    int ret = PT_REGS_RC(ctx);
    if (ret <= 0) {
        bpf_map_delete_elem(&ssl_args, &pid_tgid);
        return 0;
    }

    struct ssl_event_t *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) {
        bpf_map_delete_elem(&ssl_args, &pid_tgid);
        return 0;
    }

    event->timestamp_ns = bpf_ktime_get_ns();
    event->pid = pid_tgid >> 32;
    event->tid = (u32)pid_tgid;
    event->ssl_ptr = args->ssl_ptr;
    event->event_type = EVENT_SSL_READ;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    void *buf = (void *)args->buf_ptr;
    bpf_probe_read_user(event->data, sizeof(event->data), buf);
    event->bytes = ret; // bytes actually read

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&ssl_args, &pid_tgid);
    return 0;
}
