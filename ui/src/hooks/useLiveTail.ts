import { useEffect } from 'react';
import { useTraceStore } from '../stores/traceStore';

export const useLiveTail = () => {
  const addLiveTrace = useTraceStore((state) => state.addLiveTrace);
  
  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8080/ws/traces');
    ws.onmessage = (event) => {
      try {
        const trace = JSON.parse(event.data);
        addLiveTrace(trace);
      } catch (e) {
        console.error('Invalid trace format', e);
      }
    };
    return () => ws.close();
  }, [addLiveTrace]);
};
