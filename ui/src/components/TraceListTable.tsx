import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTraceStore } from '../stores/traceStore';
import { fetchTraces } from '../api/client';
import { fmtDuration } from '../types/api';

export const TraceListTable: React.FC = () => {
  const navigate = useNavigate();
  const {
    historicalTraces, historicalLoading, historicalError,
    setHistoricalTraces, setHistoricalLoading, setHistoricalError,
  } = useTraceStore();

  useEffect(() => {
    setHistoricalLoading(true);
    setHistoricalError(null);
    fetchTraces(100)
      .then((traces) => setHistoricalTraces(traces ?? []))
      .catch((e: Error) => setHistoricalError(e.message))
      .finally(() => setHistoricalLoading(false));
  }, [setHistoricalTraces, setHistoricalLoading, setHistoricalError]);

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 'var(--space-4)' }}>
        <h2>Traces</h2>
        <button
          id="traces-refresh-btn"
          className="btn"
          onClick={() => {
            setHistoricalLoading(true);
            fetchTraces(100)
              .then((t) => setHistoricalTraces(t ?? []))
              .catch((e: Error) => setHistoricalError(e.message))
              .finally(() => setHistoricalLoading(false));
          }}
        >
          ↻ Refresh
        </button>
      </div>

      {historicalError && (
        <div style={{
          background: 'var(--color-error-bg)',
          border: '1px solid var(--color-error)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--space-4)',
          color: 'var(--color-error)',
          marginBottom: 'var(--space-4)',
          fontSize: 'var(--font-size-sm)',
        }}>
          ⚠ {historicalError}
        </div>
      )}

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
              <th>Spans</th>
              <th>Time</th>
            </tr>
          </thead>
          <tbody>
            {historicalLoading && Array.from({ length: 8 }).map((_, i) => (
              <tr key={i}>
                {Array.from({ length: 6 }).map((_, j) => (
                  <td key={j} style={{ padding: 'var(--space-3)' }}>
                    <div className="skeleton" style={{ height: 14, width: j === 0 ? 80 : j === 4 ? 30 : 120 }} />
                  </td>
                ))}
              </tr>
            ))}

            {!historicalLoading && historicalTraces.map((t) => (
              <tr
                key={t.TraceID}
                id={`trace-row-${t.TraceID}`}
                onClick={() => navigate(`/traces/${t.TraceID}`)}
                style={{ cursor: 'pointer' }}
              >
                <td style={{ padding: 'var(--space-3)', fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)' }}>
                  {t.TraceID.substring(0, 12)}…
                </td>
                <td style={{ padding: 'var(--space-3)' }}>
                  <span style={{
                    background: 'var(--color-accent-light)',
                    color: 'var(--color-accent)',
                    borderRadius: 'var(--radius-sm)',
                    padding: '2px 8px',
                    fontSize: 'var(--font-size-xs)',
                    fontWeight: 500,
                  }}>
                    {t.RootService}
                  </span>
                </td>
                <td style={{ padding: 'var(--space-3)', fontSize: 'var(--font-size-sm)', maxWidth: 240, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {t.RootOp}
                </td>
                <td style={{ padding: 'var(--space-3)', fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)' }}>
                  {fmtDuration(t.StartTimeNs, t.EndTimeNs)}
                </td>
                <td style={{ padding: 'var(--space-3)', fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)', textAlign: 'center' }}>
                  {t.SpanCount}
                </td>
                <td style={{ padding: 'var(--space-3)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-disabled)' }}>
                  {new Date(Math.floor(t.StartTimeNs / 1_000_000)).toLocaleString()}
                </td>
              </tr>
            ))}

            {!historicalLoading && historicalTraces.length === 0 && !historicalError && (
              <tr>
                <td colSpan={6}>
                  <div className="empty-state">
                    <div className="empty-state-icon">🔍</div>
                    <div>No traces yet. Start the agent and generate some traffic.</div>
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
