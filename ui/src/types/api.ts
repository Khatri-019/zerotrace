/** TraceSummary matches the store.TraceSummary Go struct. */
export interface TraceSummary {
  TraceID: string;
  RootService: string;
  RootOp: string;
  StartTimeNs: number;
  EndTimeNs: number;
  SpanCount: number;
}

/** GraphNode matches graph.GraphNode Go struct. */
export interface GraphNode {
  id: string;
  group: number;
  p50_ms: number;
  p99_ms: number;
  error_rate: number;
}

/** GraphEdge matches graph.GraphEdge Go struct. */
export interface GraphEdge {
  source: string;
  target: string;
  call_count: number;
  error_rate: number;
  p50_ms: number;
}

/** GraphSnapshot matches graph.GraphSnapshot Go struct. */
export interface GraphSnapshot {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

/** Format nanoseconds as a human-readable duration string. */
export function fmtDuration(startNs: number, endNs: number): string {
  const ms = (endNs - startNs) / 1_000_000;
  if (ms < 1) return `${((endNs - startNs) / 1000).toFixed(0)}µs`;
  if (ms < 1000) return `${ms.toFixed(1)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

/** Format a nanosecond timestamp as a relative time string. */
export function fmtRelative(ns: number): string {
  const diff = Date.now() - ns / 1_000_000;
  if (diff < 5000) return 'just now';
  if (diff < 60_000) return `${Math.round(diff / 1000)}s ago`;
  if (diff < 3_600_000) return `${Math.round(diff / 60_000)}m ago`;
  return `${Math.round(diff / 3_600_000)}h ago`;
}
