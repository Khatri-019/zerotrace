import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AppShell } from './components/layout/AppShell';
import { TraceLiveTable } from './components/TraceLiveTable';
import { TraceListTable } from './components/TraceListTable';
import { TraceDetail } from './components/TraceDetail';
import { ServiceMap } from './components/ServiceMap';
import { useLiveTail } from './hooks/useLiveTail';
import './styles/global.css';
import './styles/reset.css';

function App() {
  useLiveTail();

  return (
    <BrowserRouter>
      <AppShell>
        <Routes>
          <Route path="/"                   element={<TraceLiveTable />} />
          <Route path="/traces"             element={<TraceListTable />} />
          <Route path="/traces/:traceID"    element={<TraceDetail />} />
          <Route path="/graph"              element={<ServiceMap />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  );
}

export default App;
