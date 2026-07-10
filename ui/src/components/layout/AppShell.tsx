import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useTraceStore } from '../../stores/traceStore';
import '../../styles/global.css';

// ── Icons ─────────────────────────────────────────────────────────────────────
const IconLive = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3" fill="currentColor"/>
  </svg>
);
const IconTraces = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/>
  </svg>
);
const IconGraph = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/>
    <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/>
    <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
  </svg>
);

// ── NavItem ───────────────────────────────────────────────────────────────────
interface NavItemProps { to: string; icon: React.ReactNode; label: string; }

const NavItem: React.FC<NavItemProps> = ({ to, icon, label }) => {
  const loc = useLocation();
  const isActive = loc.pathname === to || (to !== '/' && loc.pathname.startsWith(to));

  return (
    <Link
      to={to}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 'var(--space-3)',
        padding: '9px var(--space-4)',
        margin: '1px var(--space-2)',
        borderRadius: 'var(--radius-md)',
        textDecoration: 'none',
        fontSize: 'var(--font-size-sm)',
        fontWeight: isActive ? 'var(--font-weight-semibold)' : 'var(--font-weight-normal)',
        color: isActive ? 'var(--color-accent-dark)' : 'var(--color-text-secondary)',
        background: isActive ? 'var(--color-accent-light)' : 'transparent',
        borderLeft: isActive ? `3px solid var(--color-accent)` : '3px solid transparent',
        transition: 'all var(--transition-fast)',
      }}
      onMouseEnter={e => {
        if (!isActive) {
          (e.currentTarget as HTMLElement).style.background = 'var(--color-bg-overlay)';
          (e.currentTarget as HTMLElement).style.color = 'var(--color-text-primary)';
        }
      }}
      onMouseLeave={e => {
        if (!isActive) {
          (e.currentTarget as HTMLElement).style.background = 'transparent';
          (e.currentTarget as HTMLElement).style.color = 'var(--color-text-secondary)';
        }
      }}
    >
      {icon}
      {label}
    </Link>
  );
};

// ── TopNav ────────────────────────────────────────────────────────────────────
export const TopNav: React.FC = () => {
  const wsStatus = useTraceStore(s => s.wsStatus);

  const statusColor =
    wsStatus === 'connected'  ? 'var(--color-success)'  :
    wsStatus === 'connecting' ? 'var(--color-warning)'  :
    'var(--color-text-disabled)';

  const statusLabel =
    wsStatus === 'connected'  ? 'Live' :
    wsStatus === 'connecting' ? 'Connecting…' :
    'Disconnected';

  return (
    <div style={{
      height: 'var(--topnav-height)',
      background: 'var(--color-bg-surface)',
      borderBottom: '1px solid var(--color-border)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '0 var(--space-6)',
      position: 'sticky',
      top: 0,
      zIndex: 'var(--z-topnav)' as any,
      boxShadow: 'var(--shadow-xs)',
    }}>
      {/* Logo */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
        {/* Orange bolt icon */}
        <div style={{
          width: 32, height: 32,
          background: 'var(--color-accent)',
          borderRadius: 'var(--radius-md)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          boxShadow: '0 1px 3px rgba(246,130,31,0.4)',
          flexShrink: 0,
        }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="white">
            <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>
          </svg>
        </div>
        <div>
          <div style={{
            fontSize: 'var(--font-size-md)',
            fontWeight: 'var(--font-weight-bold)',
            color: 'var(--color-text-primary)',
            lineHeight: 1.1,
            letterSpacing: '-0.01em',
          }}>
            Zero<span style={{ color: 'var(--color-accent)' }}>Trace</span>
          </div>
          <div style={{
            fontSize: 9,
            color: 'var(--color-text-disabled)',
            fontFamily: 'var(--font-mono)',
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
          }}>
            eBPF Distributed Tracing
          </div>
        </div>
      </div>

      {/* WS Status pill */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 'var(--space-2)',
        padding: '4px 12px',
        borderRadius: 999,
        background: wsStatus === 'connected' ? 'var(--color-success-bg)' : 'var(--color-bg-overlay)',
        border: `1px solid ${wsStatus === 'connected' ? '#A8D5B5' : 'var(--color-border)'}`,
      }}>
        <span
          className={wsStatus === 'connected' ? 'pulse' : ''}
          style={{ width: 7, height: 7, borderRadius: '50%', background: statusColor, display: 'inline-block', flexShrink: 0 }}
        />
        <span style={{ fontSize: 'var(--font-size-xs)', fontWeight: 'var(--font-weight-medium)', color: statusColor }}>
          {statusLabel}
        </span>
      </div>
    </div>
  );
};

// ── Sidebar ───────────────────────────────────────────────────────────────────
export const Sidebar: React.FC = () => (
  <div style={{
    width: 'var(--sidebar-width)',
    background: 'var(--color-bg-surface)',
    borderRight: '1px solid var(--color-border)',
    height: 'calc(100vh - var(--topnav-height))',
    padding: 'var(--space-4) 0',
    display: 'flex',
    flexDirection: 'column',
    position: 'sticky',
    top: 'var(--topnav-height)',
    flexShrink: 0,
  }}>
    <div style={{
      padding: '0 var(--space-4) var(--space-2)',
      fontSize: 'var(--font-size-xs)',
      fontWeight: 'var(--font-weight-semibold)',
      textTransform: 'uppercase',
      letterSpacing: '0.08em',
      color: 'var(--color-text-disabled)',
    }}>
      Observability
    </div>
    <NavItem to="/"       icon={<IconLive />}   label="Live Tail" />
    <NavItem to="/traces" icon={<IconTraces />} label="Traces" />
    <NavItem to="/graph"  icon={<IconGraph />}  label="Service Map" />

    <div style={{
      marginTop: 'auto',
      padding: 'var(--space-4)',
      borderTop: '1px solid var(--color-border)',
      fontSize: 'var(--font-size-xs)',
      color: 'var(--color-text-disabled)',
      fontFamily: 'var(--font-mono)',
    }}>
      v2.0 · eBPF
    </div>
  </div>
);

// ── AppShell ──────────────────────────────────────────────────────────────────
export const AppShell: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', background: 'var(--color-bg-page)' }}>
    <TopNav />
    <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
      <Sidebar />
      <main style={{
        flex: 1,
        overflowY: 'auto',
        padding: 'var(--space-6)',
        background: 'var(--color-bg-page)',
      }}>
        {children}
      </main>
    </div>
  </div>
);
