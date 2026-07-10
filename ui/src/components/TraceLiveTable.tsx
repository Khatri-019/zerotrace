import React from 'react';
import { useTraceStore } from '../stores/traceStore';
import { spanDurationMs, spanIsError } from '../types/trace';
import type { Span } from '../types/trace';

function durationLabel(ms: number): string {
  if (ms < 1) return `<1ms`;
  if (ms < 1000) return `${ms.toFixed(1)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function MethodBadge({ method }: { method: string }) {
  const colors: Record<string, string> = {
    GET: '#60a5fa', POST: '#34d399', PUT: '#fbbf24',
    DELETE: '#f87171', PATCH: '#a78bfa', HEAD: '#94a3b8',
  };
  const color = colors[method] ?? 'var(--color-text-secondary)';
  return (
    <span style={{
      fontFamily: 'var(--font-mono)',
      fontSize: 'var(--font-size-xs)',
      color,
      fontWeight: 600,
      minWidth: 52,
      display: 'inline-block',
    }}>
      {method}
    </span>
  );
}

function TraceRow({ trace, idx }: { trace: Span[]; idx: number }) {
  const root = trace[0];
  if (!root) return null;
  const durationMs = trace.reduce((max, sp) => {
    const d = spanDurationMs(sp);
    return d > max ? d : max;
  }, 0);
  const hasError = trace.some(spanIsError);
  const op = root.operation_name ?? '';
  const parts = op.split(' ');
  const method = parts.length > 1 ? parts[0] : '';
  const path   = parts.length > 1 ? parts.slice(1).join(' ') : op;

  const style: React.CSSProperties = {
    animationDelay: `${idx * 20}ms`,
  };

  return (
    <tr className="fade-in" style={style}>
      <td style={{ padding: 'var(--space-3)', fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)' }}>
        {root.trace_id.substring(0, 8)}…
      </td>
      <td style={{ padding: 'var(--space-3)' }}>
        <span style={{
          background: 'var(--color-bg-overlay)',
          borderRadius: 'var(--radius-sm)',
          padding: '2px 8px',
          fontSize: 'var(--font-size-xs)',
          fontWeight: 500,
          color: 'var(--color-accent)',
        }}>
          {root.service_name}
        </span>
      </td>
      <td style={{ padding: 'var(--space-3)', maxWidth: 260, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {method && <MethodBadge method={method} />}
        <span style={{ color: 'var(--color-text-primary)', fontSize: 'var(--font-size-sm)' }}>
          {path}
        </span>
      </td>
      <td style={{ padding: 'var(--space-3)', fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)' }}>
        <span style={{ color: durationMs > 500 ? 'var(--color-warning)' : 'var(--color-text-primary)' }}>
          {durationLabel(durationMs)}
        </span>
      </td>
      <td style={{ padding: 'var(--space-3)', textAlign: 'center' }}>
        <span style={{
          background: hasError ? 'var(--color-error-bg)' : 'var(--color-success-bg)',
          color: hasError ? 'var(--color-error)' : 'var(--color-success)',
          padding: '2px 8px',
          borderRadius: 999,
          fontSize: 'var(--font-size-xs)',
          fontWeight: 600,
        }}>
          {hasError ? 'ERR' : 'OK'}
        </span>
      </td>
      <td style={{ padding: 'var(--space-3)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)' }}>
        {trace.length}
      </td>
      <td style={{ padding: 'var(--space-3)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-disabled)' }}>
        {new Date(Math.floor(root.start_time_ns / 1_000_000)).toLocaleString()}
      </td>
    </tr>
  );
}

export const TraceLiveTable: React.FC = () => {
  const { liveTraces, isLivePaused, toggleLivePause, wsStatus } = useTraceStore();

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 'var(--space-4)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
          <h2>Live Tail</h2>
          {wsStatus === 'connected' && (
            <span className="pulse" style={{
              width: 8, height: 8, borderRadius: '50%',
              background: 'var(--color-success)',
              display: 'inline-block',
            }} />
          )}
        </div>
        <div style={{ display: 'flex', gap: 'var(--space-2)', alignItems: 'center' }}>
          <span style={{ fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)' }}>
            {liveTraces.length} traces
          </span>
          <button
            id="live-tail-pause-btn"
            onClick={toggleLivePause}
            className={isLivePaused ? 'btn btn-primary' : 'btn'}
            style={isLivePaused ? {} : { borderColor: 'var(--color-success)', color: 'var(--color-success)' }}
          >
            {isLivePaused ? '▶ Resume' : '⏸ Pause'}
          </button>
        </div>
      </div>

      {/* Table */}
      <div style={{
        background: 'var(--color-bg-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-lg)',
        overflow: 'hidden',
      }}>
        <table className="table">
          <thead>
            <tr>
              <th>Trace ID</th>
              <th>Service</th>
              <th>Operation</th>
              <th>Duration</th>
              <th>Status</th>
              <th>Spans</th>
              <th>Time</th>
            </tr>
          </thead>
          <tbody>
            {liveTraces.map((trace, i) => (
              <TraceRow key={trace[0]?.trace_id ?? i} trace={trace} idx={i} />
            ))}
            {liveTraces.length === 0 && (
              <tr>
                <td colSpan={7}>
                  <div className="empty-state">
                    <div className="empty-state-icon">📡</div>
                    <div>
                      {wsStatus === 'connected'
                        ? 'Waiting for traces… Generate some HTTP traffic!'
                        : wsStatus === 'connecting'
                        ? 'Connecting to collector…'
                        : 'Disconnected from collector. Reconnecting…'}
                    </div>
                  </div>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};
