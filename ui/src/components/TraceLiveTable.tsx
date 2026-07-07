import React from 'react';
import { useTraceStore } from '../stores/traceStore';

export const TraceLiveTable: React.FC = () => {
  const { liveTraces, isLivePaused, toggleLivePause } = useTraceStore();

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 'var(--space-4)' }}>
        <h2>Live Tail</h2>
        <button 
          onClick={toggleLivePause} 
          className="btn-primary" 
          style={{ backgroundColor: isLivePaused ? 'var(--color-accent)' : 'var(--color-success)' }}
        >
          {isLivePaused ? 'Resume' : 'Pause'}
        </button>
      </div>
      <table style={{ width: '100%', borderCollapse: 'collapse', backgroundColor: 'var(--color-bg-surface)' }}>
        <thead>
          <tr style={{ borderBottom: '2px solid var(--color-border)', textAlign: 'left' }}>
            <th style={{ padding: 'var(--space-2)' }}>Trace ID</th>
            <th>Service</th>
            <th>Operation</th>
            <th>Duration</th>
            <th>Spans</th>
          </tr>
        </thead>
        <tbody>
          {liveTraces.map((trace, i) => (
            <tr key={i} className="table-row" style={{ borderBottom: '1px solid var(--color-border)' }}>
              <td style={{ padding: 'var(--space-2)' }}>{trace[0]?.trace_id.substring(0, 8)}</td>
              <td>{trace[0]?.service_name}</td>
              <td>{trace[0]?.operation_name}</td>
              <td>{(trace[trace.length - 1]?.end_time_ns - trace[0]?.start_time_ns) / 1000000} ms</td>
              <td>{trace.length}</td>
            </tr>
          ))}
          {liveTraces.length === 0 && (
            <tr>
              <td colSpan={5} style={{ padding: 'var(--space-4)', textAlign: 'center', color: 'var(--color-text-secondary)' }}>
                Waiting for traces...
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
};
