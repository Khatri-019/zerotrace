
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AppShell } from './components/layout/AppShell';
import { TraceLiveTable } from './components/TraceLiveTable';
import { TraceListTable } from './components/TraceListTable';
import { ServiceMap } from './components/ServiceMap';
import { useLiveTail } from './hooks/useLiveTail';

function App() {
  useLiveTail();

  return (
    <BrowserRouter>
      <AppShell>
        <Routes>
          <Route path="/" element={<TraceLiveTable />} />
          <Route path="/traces" element={<TraceListTable />} />
          <Route path="/graph" element={<ServiceMap />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  );
}

export default App;
