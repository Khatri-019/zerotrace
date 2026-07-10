import { useEffect, useRef } from 'react';
import { useTraceStore } from '../stores/traceStore';
import type { Span } from '../types/trace';

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws/traces';
const INITIAL_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;

/**
 * useLiveTail connects to the WebSocket endpoint and pushes incoming span
 * batches into the Zustand store. Reconnects automatically with exponential
 * backoff when the connection is lost.
 */
export const useLiveTail = () => {
  const addLiveTrace = useTraceStore((s) => s.addLiveTrace);
  const setWsStatus  = useTraceStore((s) => s.setWsStatus);

  const backoffRef = useRef(INITIAL_BACKOFF_MS);
  const timerRef   = useRef<ReturnType<typeof setTimeout> | null>(null);
  const wsRef      = useRef<WebSocket | null>(null);
  const unmounted  = useRef(false);

  useEffect(() => {
    unmounted.current = false;

    function connect() {
      if (unmounted.current) return;

      setWsStatus('connecting');
      const ws = new WebSocket(WS_URL);
      wsRef.current = ws;

      ws.onopen = () => {
        backoffRef.current = INITIAL_BACKOFF_MS; // reset on success
        setWsStatus('connected');
      };

      ws.onmessage = (ev) => {
        try {
          const spans = JSON.parse(ev.data as string) as Span[];
          if (Array.isArray(spans) && spans.length > 0) {
            addLiveTrace(spans);
          }
        } catch {
          // Ignore malformed frames
        }
      };

      ws.onclose = () => {
        if (unmounted.current) return;
        setWsStatus('disconnected');
        // Exponential backoff reconnect
        timerRef.current = setTimeout(() => {
          backoffRef.current = Math.min(backoffRef.current * 2, MAX_BACKOFF_MS);
          connect();
        }, backoffRef.current);
      };

      ws.onerror = () => {
        ws.close(); // triggers onclose → reconnect
      };
    }

    connect();

    return () => {
      unmounted.current = true;
      if (timerRef.current) clearTimeout(timerRef.current);
      wsRef.current?.close();
    };
  }, [addLiveTrace, setWsStatus]);
};
