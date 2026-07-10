import type { Span } from '../types/trace';
import type { GraphSnapshot, TraceSummary } from '../types/api';

const BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`);
  if (!res.ok) {
    throw new Error(`API error ${res.status}: ${path}`);
  }
  return res.json() as Promise<T>;
}

/** Return the most recent traces (summary only). */
export const fetchTraces = (limit = 50, offset = 0): Promise<TraceSummary[]> =>
  fetchJSON(`/api/traces?limit=${limit}&offset=${offset}`);

/** Return all spans for a specific trace ID. */
export const fetchTrace = (traceID: string): Promise<Span[]> =>
  fetchJSON(`/api/traces/${traceID}`);

/** Return the list of known service names. */
export const fetchServices = (): Promise<string[]> =>
  fetchJSON('/api/services');

/** Return the service dependency graph snapshot. */
export const fetchGraph = (): Promise<GraphSnapshot> =>
  fetchJSON('/api/graph');

/** Return basic collector stats. */
export const fetchStats = (): Promise<{ index_size: number; status: string }> =>
  fetchJSON('/api/stats');
