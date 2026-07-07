import { create } from 'zustand';
import type { Span } from '../types/trace';

interface TraceState {
  liveTraces: Span[][];
  addLiveTrace: (trace: Span[]) => void;
  isLivePaused: boolean;
  toggleLivePause: () => void;
}

export const useTraceStore = create<TraceState>((set) => ({
  liveTraces: [],
  addLiveTrace: (trace) => set((state) => {
    if (state.isLivePaused) return state;
    const newTraces = [trace, ...state.liveTraces];
    if (newTraces.length > 200) newTraces.pop();
    return { liveTraces: newTraces };
  }),
  isLivePaused: false,
  toggleLivePause: () => set((state) => ({ isLivePaused: !state.isLivePaused })),
}));
