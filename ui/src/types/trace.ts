export interface SpanLog {
  timestamp_ns: number;
  message: string;
}

export interface Span {
  trace_id: string;
  span_id: string;
  parent_span_id: string;
  service_name: string;
  operation_name: string;
  start_time_ns: number;
  end_time_ns: number;
  tags: Record<string, string>;
  logs: SpanLog[];
  kind: number;
}
