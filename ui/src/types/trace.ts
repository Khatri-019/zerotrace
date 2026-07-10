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
  kind: number; // 0=unspecified, 1=client, 2=server
}

/** Duration of span in milliseconds. */
export function spanDurationMs(span: Span): number {
  return (span.end_time_ns - span.start_time_ns) / 1_000_000;
}

/** True if this span has a status code ≥ 500. */
export function spanIsError(span: Span): boolean {
  const code = parseInt(span.tags?.['http.status_code'] ?? '0', 10);
  return code >= 500;
}
