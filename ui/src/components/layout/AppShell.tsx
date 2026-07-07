import React from 'react';
import { Link } from 'react-router-dom';

export const TopNav: React.FC = () => {
  return (
    <div style={{ height: '56px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', padding: '0 var(--space-4)' }}>
      <h1 style={{ fontSize: 'var(--font-size-lg)', fontWeight: 'var(--font-weight-semibold)', color: 'var(--color-accent)' }}>ZeroTrace</h1>
    </div>
  );
};

export const Sidebar: React.FC = () => {
  return (
    <div style={{ width: '220px', backgroundColor: 'var(--color-bg-subtle)', borderRight: '1px solid var(--color-border)', height: 'calc(100vh - 56px)', padding: 'var(--space-4) 0' }}>
      <ul style={{ listStyle: 'none' }}>
        <li><Link to="/" style={{ display: 'block', padding: 'var(--space-2) var(--space-4)', textDecoration: 'none', color: 'var(--color-text-primary)' }}>Live</Link></li>
        <li><Link to="/traces" style={{ display: 'block', padding: 'var(--space-2) var(--space-4)', textDecoration: 'none', color: 'var(--color-text-primary)' }}>Traces</Link></li>
        <li><Link to="/graph" style={{ display: 'block', padding: 'var(--space-2) var(--space-4)', textDecoration: 'none', color: 'var(--color-text-primary)' }}>Graph</Link></li>
      </ul>
    </div>
  );
};

export const AppShell: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <TopNav />
      <div style={{ display: 'flex', flex: 1 }}>
        <Sidebar />
        <main style={{ flex: 1, padding: 'var(--space-6)', overflowY: 'auto' }}>
          {children}
        </main>
      </div>
    </div>
  );
};
