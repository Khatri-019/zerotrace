import { create } from 'zustand';
import type { Span } from '../types/trace';
import type { TraceSummary } from '../types/api';

interface TraceState {
  /** Live-tail: most recent spans streamed via WebSocket */
  liveTraces: Span[][];
  addLiveTrace: (spans: Span[]) => void;
  isLivePaused: boolean;
  toggleLivePause: () => void;
  wsStatus: 'connecting' | 'connected' | 'disconnected';
  setWsStatus: (s: TraceState['wsStatus']) => void;

  /** Historical traces fetched from REST API */
  historicalTraces: TraceSummary[];
  historicalLoading: boolean;
  historicalError: string | null;
  setHistoricalTraces: (traces: TraceSummary[]) => void;
  setHistoricalLoading: (v: boolean) => void;
  setHistoricalError: (e: string | null) => void;

  /** Selected trace detail */
  selectedTraceSpans: Span[] | null;
  selectedTraceLoading: boolean;
  setSelectedTraceSpans: (spans: Span[] | null) => void;
  setSelectedTraceLoading: (v: boolean) => void;
}

const MAX_LIVE_TRACES = 200;

export const useTraceStore = create<TraceState>((set) => ({
  liveTraces: [],
  addLiveTrace: (spans) =>
    set((state) => {
      if (state.isLivePaused) return state;
      const next = [spans, ...state.liveTraces];
      if (next.length > MAX_LIVE_TRACES) next.length = MAX_LIVE_TRACES;
      return { liveTraces: next };
    }),
  isLivePaused: false,
  toggleLivePause: () => set((s) => ({ isLivePaused: !s.isLivePaused })),
  wsStatus: 'disconnected',
  setWsStatus: (wsStatus) => set({ wsStatus }),

  historicalTraces: [],
  historicalLoading: false,
  historicalError: null,
  setHistoricalTraces: (historicalTraces) => set({ historicalTraces }),
  setHistoricalLoading: (historicalLoading) => set({ historicalLoading }),
  setHistoricalError: (historicalError) => set({ historicalError }),

  selectedTraceSpans: null,
  selectedTraceLoading: false,
  setSelectedTraceSpans: (selectedTraceSpans) => set({ selectedTraceSpans }),
  setSelectedTraceLoading: (selectedTraceLoading) => set({ selectedTraceLoading }),
}));
