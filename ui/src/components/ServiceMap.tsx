import React, { useEffect, useRef, useState } from 'react';
import * as d3 from 'd3';
import { fetchGraph } from '../api/client';
import type { GraphSnapshot } from '../types/api';

interface SimNode extends d3.SimulationNodeDatum {
  id: string;
  p50_ms: number;
  p99_ms: number;
  error_rate: number;
}
interface SimLink extends d3.SimulationLinkDatum<SimNode> {
  source: string | SimNode;
  target: string | SimNode;
  call_count: number;
  error_rate: number;
  p50_ms: number;
}

// Node fill: orange for healthy, amber for some errors, red for high error rate
function nodeColor(n: SimNode): string {
  if (n.error_rate > 0.1) return '#D93025'; // error red
  if (n.error_rate > 0.01) return '#B06000'; // amber
  return '#F6821F'; // CF orange healthy
}

export const ServiceMap: React.FC = () => {
  const svgRef = useRef<SVGSVGElement | null>(null);
  const [graph, setGraph] = useState<GraphSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const simRef = useRef<d3.Simulation<SimNode, SimLink> | null>(null);

  useEffect(() => {
    setLoading(true);
    fetchGraph()
      .then(setGraph)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
    const id = setInterval(() => { fetchGraph().then(setGraph).catch(() => {}); }, 15_000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    if (!svgRef.current || !graph) return;

    const el = svgRef.current;
    const { width, height } = el.getBoundingClientRect();
    const W = width || 800;
    const H = height || 500;

    simRef.current?.stop();

    const svg = d3.select(el);
    svg.selectAll('*').remove();

    if (graph.nodes.length === 0) return;

    const nodes: SimNode[] = graph.nodes.map(n => ({ ...n }));
    const links: SimLink[] = (graph.edges || []).map(e => ({ ...e }));

    // ── Defs ──────────────────────────────────────────────────────────────────
    const defs = svg.append('defs');

    // Arrow marker (dark, visible on white)
    defs.append('marker')
      .attr('id', 'zt-arrow')
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', 30)
      .attr('refY', 0)
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-5L10,0L0,5')
      .attr('fill', '#9AA0A6');

    // Drop-shadow filter for nodes
    const filter = defs.append('filter').attr('id', 'zt-drop').attr('x', '-30%').attr('y', '-30%').attr('width', '160%').attr('height', '160%');
    filter.append('feDropShadow').attr('dx', 0).attr('dy', 2).attr('stdDeviation', 3).attr('flood-color', 'rgba(60,64,67,0.18)');

    // ── Container with zoom ───────────────────────────────────────────────────
    const container = svg.append('g');
    svg.call(d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.25, 4])
      .on('zoom', ev => container.attr('transform', ev.transform)));

    // ── Links ─────────────────────────────────────────────────────────────────
    const link = container.append('g')
      .selectAll<SVGLineElement, SimLink>('line')
      .data(links)
      .join('line')
      .attr('stroke', d => d.error_rate > 0.05 ? '#D93025' : '#DADCE0')
      .attr('stroke-width', d => Math.max(1.5, Math.log10(d.call_count + 1) * 2))
      .attr('stroke-opacity', 0.8)
      .attr('marker-end', 'url(#zt-arrow)');

    // Link latency label
    const linkLabel = container.append('g')
      .selectAll<SVGTextElement, SimLink>('text')
      .data(links)
      .join('text')
      .attr('text-anchor', 'middle')
      .attr('font-size', 9)
      .attr('font-family', "'JetBrains Mono', monospace")
      .attr('fill', '#9AA0A6')
      .text(d => d.p50_ms > 0 ? `${d.p50_ms.toFixed(0)}ms` : '');

    // ── Node groups ───────────────────────────────────────────────────────────
    const nodeGroup = container.append('g')
      .selectAll<SVGGElement, SimNode>('g')
      .data(nodes, d => d.id)
      .join('g')
      .call(d3.drag<SVGGElement, SimNode>()
        .on('start', (ev, d) => { if (!ev.active) simRef.current?.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; })
        .on('drag',  (ev, d) => { d.fx = ev.x; d.fy = ev.y; })
        .on('end',   (ev, d) => { if (!ev.active) simRef.current?.alphaTarget(0); d.fx = null; d.fy = null; }));

    // White card shadow behind circle
    nodeGroup.append('circle').attr('r', 28).attr('fill', 'white').attr('filter', 'url(#zt-drop)');

    // Coloured ring
    nodeGroup.append('circle')
      .attr('r', 24)
      .attr('fill', nodeColor)
      .attr('stroke', 'white')
      .attr('stroke-width', 3);

    // Error dashed ring
    nodeGroup.filter(d => d.error_rate > 0.01)
      .append('circle')
      .attr('r', 29)
      .attr('fill', 'none')
      .attr('stroke', '#D93025')
      .attr('stroke-width', 2)
      .attr('stroke-dasharray', '5 3')
      .attr('opacity', 0.6);

    // Initial letter inside node (white)
    nodeGroup.append('text')
      .attr('text-anchor', 'middle')
      .attr('dominant-baseline', 'central')
      .attr('font-size', 14)
      .attr('font-weight', 700)
      .attr('font-family', "'Inter', sans-serif")
      .attr('fill', 'white')
      .attr('pointer-events', 'none')
      .text(d => d.id.charAt(0).toUpperCase());

    // Service name label below
    nodeGroup.append('text')
      .attr('text-anchor', 'middle')
      .attr('dy', 42)
      .attr('font-size', 11)
      .attr('font-weight', 600)
      .attr('font-family', "'Inter', sans-serif")
      .attr('fill', '#202124')
      .text(d => d.id);

    // P50 label
    nodeGroup.append('text')
      .attr('text-anchor', 'middle')
      .attr('dy', 55)
      .attr('font-size', 9)
      .attr('font-family', "'JetBrains Mono', monospace")
      .attr('fill', '#9AA0A6')
      .text(d => d.p50_ms > 0 ? `p50 ${d.p50_ms.toFixed(1)}ms` : '');

    // Tooltip
    nodeGroup.append('title')
      .text(d => `${d.id}\nP50: ${d.p50_ms.toFixed(1)}ms  P99: ${d.p99_ms.toFixed(1)}ms\nError rate: ${(d.error_rate*100).toFixed(1)}%`);

    // ── Simulation ────────────────────────────────────────────────────────────
    const sim = d3.forceSimulation<SimNode>(nodes)
      .force('link',      d3.forceLink<SimNode, SimLink>(links).id(d => d.id).distance(180))
      .force('charge',    d3.forceManyBody<SimNode>().strength(-700))
      .force('center',    d3.forceCenter(W/2, H/2))
      .force('collision', d3.forceCollide<SimNode>(55));

    simRef.current = sim;

    sim.on('tick', () => {
      link
        .attr('x1', d => (d.source as SimNode).x ?? 0)
        .attr('y1', d => (d.source as SimNode).y ?? 0)
        .attr('x2', d => (d.target as SimNode).x ?? 0)
        .attr('y2', d => (d.target as SimNode).y ?? 0);
      linkLabel
        .attr('x', d => ((d.source as SimNode).x! + (d.target as SimNode).x!) / 2)
        .attr('y', d => ((d.source as SimNode).y! + (d.target as SimNode).y!) / 2 - 6);
      nodeGroup.attr('transform', d => `translate(${d.x ?? 0},${d.y ?? 0})`);
    });

    return () => { sim.stop(); };
  }, [graph]);

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 'var(--space-4)' }}>
        <div>
          <h2 style={{ color: 'var(--color-text-primary)' }}>Service Map</h2>
          <p style={{ fontSize: 'var(--font-size-sm)', color: 'var(--color-text-secondary)', marginTop: 2 }}>
            Auto-discovered from live traffic · refreshes every 15s
          </p>
        </div>
        <button className="btn" id="graph-refresh-btn"
          onClick={() => { fetchGraph().then(setGraph).catch(() => {}); }}>
          ↻ Refresh
        </button>
      </div>

      {error && (
        <div className="alert alert-error" style={{ marginBottom: 'var(--space-4)' }}>
          ⚠ {error}
        </div>
      )}

      {/* Legend */}
      {graph && graph.nodes.length > 0 && (
        <div style={{ display: 'flex', gap: 'var(--space-5)', marginBottom: 'var(--space-3)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-secondary)' }}>
          {[{ color: '#F6821F', label: 'Healthy' }, { color: '#B06000', label: 'Some errors' }, { color: '#D93025', label: 'High error rate' }].map(x => (
            <div key={x.label} style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
              <div style={{ width: 10, height: 10, borderRadius: '50%', background: x.color }}/>
              {x.label}
            </div>
          ))}
          <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 5 }}>
            <span style={{ color: 'var(--color-text-disabled)' }}>Scroll to zoom · Drag to pan · Drag nodes to rearrange</span>
          </div>
        </div>
      )}

      <div style={{
        background: 'var(--color-bg-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-lg)',
        overflow: 'hidden',
        height: 520,
        boxShadow: 'var(--shadow-sm)',
      }}>
        {loading && (
          <div className="empty-state">
            <div style={{ fontSize: 32, animation: 'spin 1s linear infinite' }}>⚙</div>
            <div>Loading service map…</div>
          </div>
        )}
        {!loading && (!graph || graph.nodes.length === 0) && (
          <div className="empty-state">
            <div className="empty-state-icon">🗺</div>
            <div style={{ color: 'var(--color-text-secondary)' }}>
              No services discovered yet.
            </div>
            <div style={{ fontSize: 'var(--font-size-sm)', color: 'var(--color-text-disabled)', maxWidth: 360 }}>
              The service map builds automatically from multi-service traces. Start the agent and generate traffic across service boundaries.
            </div>
          </div>
        )}
        <svg
          ref={svgRef}
          style={{
            width: '100%',
            height: '100%',
            background: 'var(--color-bg-page)',
            display: loading || !graph || graph.nodes.length === 0 ? 'none' : 'block',
          }}
        />
      </div>
    </div>
  );
};
