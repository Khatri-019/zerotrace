import React, { useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTraceStore } from '../stores/traceStore';
import { fetchTrace } from '../api/client';
import { spanDurationMs, spanIsError } from '../types/trace';
import type { Span } from '../types/trace';

// ── Waterfall chart constants ─────────────────────────────────────────────────
const ROW_HEIGHT = 36;
const LABEL_WIDTH = 240;
const BAR_MIN_WIDTH = 3;

function WaterfallBar({ span, traceStart, traceDuration }: {
  span: Span;
  traceStart: number;
  traceDuration: number;
}) {
  const offsetMs = (span.start_time_ns - traceStart) / 1_000_000;
  const widthMs  = spanDurationMs(span);
  const totalMs  = traceDuration / 1_000_000 || 1;

  const leftPct  = (offsetMs / totalMs) * 100;
  const widthPct = Math.max((widthMs / totalMs) * 100, (BAR_MIN_WIDTH / 800) * 100);

  const isError = spanIsError(span);
  const barColor = isError
    ? 'var(--color-error)'
    : span.kind === 2
    ? 'var(--color-success)'
    : 'var(--color-accent)';

  return (
    <div style={{ position: 'relative', height: ROW_HEIGHT - 8, display: 'flex', alignItems: 'center' }}>
      <div style={{
        position: 'absolute',
        left: `${leftPct}%`,
        width: `${widthPct}%`,
        height: 18,
        background: barColor,
        borderRadius: 3,
        opacity: 0.85,
        transition: 'opacity var(--transition-fast)',
        minWidth: BAR_MIN_WIDTH,
      }}>
        <span style={{
          position: 'absolute',
          left: 4, top: 0, bottom: 0,
          display: 'flex', alignItems: 'center',
          fontSize: 10,
          fontFamily: 'var(--font-mono)',
          color: 'rgba(0,0,0,0.75)',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
        }}>
          {widthMs.toFixed(1)}ms
        </span>
      </div>
    </div>
  );
}

function SpanRow({ span, depth, traceStart, traceDuration }: {
  span: Span; depth: number; traceStart: number; traceDuration: number;
}) {
  const isError = spanIsError(span);
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      height: ROW_HEIGHT,
      borderBottom: '1px solid var(--color-border)',
    }}>
      {/* Label column */}
      <div style={{
        width: LABEL_WIDTH,
        minWidth: LABEL_WIDTH,
        paddingLeft: `${depth * 12 + 12}px`,
        paddingRight: 8,
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        gap: 1,
      }}>
        <span style={{ fontSize: 'var(--font-size-xs)', fontWeight: 500, color: isError ? 'var(--color-error)' : 'var(--color-text-primary)' }}>
          {span.service_name}
        </span>
        <span style={{ fontSize: 10, color: 'var(--color-text-disabled)', fontFamily: 'var(--font-mono)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {span.operation_name}
        </span>
      </div>
      {/* Waterfall column */}
      <div style={{ flex: 1, paddingRight: 12 }}>
        <WaterfallBar span={span} traceStart={traceStart} traceDuration={traceDuration} />
      </div>
    </div>
  );
}

// Recursively render spans in tree order
function renderTree(
  spanId: string,
  childMap: Map<string, Span[]>,
  allSpans: Map<string, Span>,
  traceStart: number,
  traceDuration: number,
  depth = 0
): React.ReactNode[] {
  const span = allSpans.get(spanId);
  const children = childMap.get(spanId) ?? [];
  const result: React.ReactNode[] = [];
  if (span) {
    result.push(
      <SpanRow key={span.span_id} span={span} depth={depth}
        traceStart={traceStart} traceDuration={traceDuration} />
    );
  }
  for (const child of children) {
    result.push(...renderTree(child.span_id, childMap, allSpans, traceStart, traceDuration, depth + 1));
  }
  return result;
}

export const TraceDetail: React.FC = () => {
  const { traceID } = useParams<{ traceID: string }>();
  const navigate = useNavigate();
  const { selectedTraceSpans, selectedTraceLoading, setSelectedTraceSpans, setSelectedTraceLoading } = useTraceStore();

  useEffect(() => {
    if (!traceID) return;
    setSelectedTraceLoading(true);
    fetchTrace(traceID)
      .then(setSelectedTraceSpans)
      .catch(() => setSelectedTraceSpans(null))
      .finally(() => setSelectedTraceLoading(false));
    return () => setSelectedTraceSpans(null);
  }, [traceID, setSelectedTraceSpans, setSelectedTraceLoading]);

  if (selectedTraceLoading) {
    return (
      <div>
        <button className="btn btn-ghost" onClick={() => navigate('/traces')} style={{ marginBottom: 'var(--space-4)' }}>← Back</button>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="skeleton" style={{ height: ROW_HEIGHT, borderRadius: 'var(--radius-md)' }} />
          ))}
        </div>
      </div>
    );
  }

  if (!selectedTraceSpans || selectedTraceSpans.length === 0) {
    return (
      <div>
        <button className="btn btn-ghost" onClick={() => navigate('/traces')} style={{ marginBottom: 'var(--space-4)' }}>← Back</button>
        <div className="empty-state"><div className="empty-state-icon">🔍</div><div>Trace not found.</div></div>
      </div>
    );
  }

  const spans = [...selectedTraceSpans].sort((a, b) => a.start_time_ns - b.start_time_ns);
  const traceStart = spans[0].start_time_ns;
  const traceEnd   = Math.max(...spans.map(s => s.end_time_ns));
  const traceDuration = traceEnd - traceStart;
  const totalMs = traceDuration / 1_000_000;

  // Build child map
  const childMap = new Map<string, Span[]>();
  const allSpans = new Map<string, Span>(spans.map(s => [s.span_id, s]));
  for (const sp of spans) {
    if (!childMap.has(sp.parent_span_id)) childMap.set(sp.parent_span_id, []);
    childMap.get(sp.parent_span_id)!.push(sp);
  }

  // Find root spans (empty parent)
  const roots = spans.filter(s => !s.parent_span_id || !allSpans.has(s.parent_span_id));

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-4)', marginBottom: 'var(--space-5)' }}>
        <button id="trace-back-btn" className="btn btn-ghost" onClick={() => navigate('/traces')}>← Back</button>
        <h2>Trace Detail</h2>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)', background: 'var(--color-bg-elevated)', padding: '4px 8px', borderRadius: 'var(--radius-sm)' }}>
          {traceID}
        </span>
      </div>

      {/* Summary stats */}
      <div style={{ display: 'flex', gap: 'var(--space-4)', marginBottom: 'var(--space-5)' }}>
        {[
          { label: 'Duration', value: `${totalMs.toFixed(2)}ms` },
          { label: 'Spans', value: spans.length },
          { label: 'Services', value: new Set(spans.map(s => s.service_name)).size },
        ].map(({ label, value }) => (
          <div key={label} style={{
            background: 'var(--color-bg-surface)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-lg)',
            padding: 'var(--space-4) var(--space-5)',
            minWidth: 120,
          }}>
            <div style={{ fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)', marginBottom: 4 }}>{label}</div>
            <div style={{ fontSize: 'var(--font-size-lg)', fontWeight: 600, fontFamily: 'var(--font-mono)' }}>{value}</div>
          </div>
        ))}
      </div>

      {/* Waterfall */}
      <div style={{
        background: 'var(--color-bg-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-lg)',
        overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{ display: 'flex', height: 32, alignItems: 'center', background: 'var(--color-bg-subtle)', borderBottom: '1px solid var(--color-border)' }}>
          <div style={{ width: LABEL_WIDTH, minWidth: LABEL_WIDTH, paddingLeft: 12, fontSize: 'var(--font-size-xs)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--color-text-secondary)' }}>
            Span
          </div>
          <div style={{ flex: 1, fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)', fontFamily: 'var(--font-mono)', paddingRight: 12, textAlign: 'right' }}>
            0ms → {totalMs.toFixed(1)}ms
          </div>
        </div>
        {/* Rows */}
        <div style={{ overflowY: 'auto', maxHeight: '60vh' }}>
          {roots.flatMap(root => renderTree(root.span_id, childMap, allSpans, traceStart, traceDuration))}
        </div>
      </div>
    </div>
  );
};
