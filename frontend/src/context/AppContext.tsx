import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react';
import { toast } from 'sonner';
import type { GlobalSnapshot } from '@/types';
import type { WSMessage } from '@/types';

interface AppState {
  snapshot: GlobalSnapshot | null;
  prevSnapshot: GlobalSnapshot | null;
  connected: boolean;
  selectedNode: string;
  setSelectedNode: (id: string) => void;
  selectedInterface: string;
  setSelectedInterface: (iface: string) => void;
  ackAlert: (id: string) => Promise<void>;
  autoRefresh: boolean;
  toggleAutoRefresh: () => void;
}

const AppContext = createContext<AppState>({
  snapshot: null,
  prevSnapshot: null,
  connected: false,
  selectedNode: '',
  setSelectedNode: () => {},
  selectedInterface: '',
  setSelectedInterface: () => {},
  ackAlert: async () => {},
  autoRefresh: true,
  toggleAutoRefresh: () => {},
});

export function AppContextProvider({ children }: { children: ReactNode }) {
  const [snapshot, setSnapshot] = useState<GlobalSnapshot | null>(null);
  const [prevSnapshot, setPrevSnapshot] = useState<GlobalSnapshot | null>(null);
  const [connected, setConnected] = useState(false);
  const [selectedNode, setSelectedNode] = useState('');
  const [selectedInterface, setSelectedInterface] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const toggleAutoRefresh = useCallback(() => setAutoRefresh((v) => !v), []);
  const wsRef = useRef<WebSocket | null>(null);
  const pendingRef = useRef<GlobalSnapshot | null>(null);
  const lastUpdateRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Flush pending snapshot to state at 2s intervals (respects autoRefresh)
  useEffect(() => {
    timerRef.current = setInterval(() => {
      if (!autoRefresh) return;
      const pending = pendingRef.current;
      if (pending) {
        pendingRef.current = null;
        setSnapshot((prev) => {
          setPrevSnapshot(prev);
          // Merge alerts: keep alerts that arrived via new_alert between snapshots
          const pendingAlerts = pending.alerts || [];
          const prevAlerts = prev?.alerts || [];
          const pendingIds = new Set(pendingAlerts.map((a: { id: string }) => a.id));
          const onlyInPrev = prevAlerts.filter((a: { id: string }) => !pendingIds.has(a.id));
          return { ...pending, alerts: [...onlyInPrev, ...pendingAlerts] };
        });
      }
    }, 2000);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, []);

  useEffect(() => {
    let stopped = false;

    function connect() {
      // Don't connect if not authenticated
      let token = '';
      try { token = localStorage.getItem('netgazer-token') || ''; } catch { /* ignore */ }
      if (!token || stopped) return;

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const ws = new WebSocket(`${protocol}//${window.location.host}${import.meta.env.BASE_URL}ws`);
      wsRef.current = ws;

      ws.onopen = () => setConnected(true);

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage;
          if (msg.type === 'snapshot') {
            // Throttle: store latest, flush every 2s
            pendingRef.current = msg.data;
            // If first snapshot, show immediately
            setSnapshot((prev) => {
              if (!prev) return msg.data;
              return prev;
            });
          } else if (msg.type === 'new_alert') {
            // Prepend new alert for real-time visibility; next snapshot will reconcile
            setSnapshot((prev) => {
              if (!prev) return prev;
              const exists = prev.alerts.some((a) => a.id === msg.data.id);
              if (exists) return prev;
              return { ...prev, alerts: [msg.data, ...prev.alerts] };
            });
            const sev = msg.data.severity;
            toast[sev === 'critical' ? 'error' : sev === 'warning' ? 'warning' : 'info'](
              `[${msg.data.type}] ${msg.data.message}`,
              { duration: 3000 }
            );
          } else if (msg.type === 'nodes_update') {
            setSnapshot((prev) => {
              if (!prev) return prev;
              return { ...prev, nodes: msg.data };
            });
          }
        } catch {
          // ignore parse errors
        }
      };

      ws.onclose = () => {
        setConnected(false);
        wsRef.current = null;
        if (!stopped) setTimeout(connect, 3000);
      };

      ws.onerror = () => ws.close();
    }

    connect();

    return () => {
      stopped = true;
      wsRef.current?.close();
    };
  }, []);

  const ackAlert = useCallback(async (id: string) => {
    try {
      const { acknowledgeAlert } = await import('@/lib/api');
      await acknowledgeAlert(id);
      setSnapshot((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          alerts: prev.alerts.map((a) =>
            a.id === id ? { ...a, acknowledged: true } : a
          ),
        };
      });
    } catch {
      // ignore
    }
  }, []);

  return (
    <AppContext.Provider value={{ snapshot, prevSnapshot, connected, selectedNode, setSelectedNode, selectedInterface, setSelectedInterface, ackAlert, autoRefresh, toggleAutoRefresh }}>
      {children}
    </AppContext.Provider>
  );
}

export function useAppContext() {
  return useContext(AppContext);
}
