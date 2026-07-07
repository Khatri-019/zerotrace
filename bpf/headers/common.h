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
