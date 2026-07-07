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
