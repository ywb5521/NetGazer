import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ChartContainer, ChartTooltip } from '@/components/ui/chart';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { fetchHostTrafficHistory } from '@/lib/api';
import { formatBytes } from '@/lib/utils';
import { AreaChart, Area, XAxis, YAxis } from 'recharts';
import { useMemo, useState, useEffect, useRef, useCallback } from 'react';
import type { HostSnapshot } from '@/types';

const MAX_LIVE_POINTS = 60;

interface ChartPoint {
  time: string;
  sent: number;
  received: number;
}

const PRESETS = [
  { labelKey: 'live' as const, value: 0 },
  { label: '15m', value: 15 },
  { label: '1h', value: 60 },
  { label: '6h', value: 360 },
];

interface Props {
  ip: string;
}

export function HostTrafficTab({ ip }: Props) {
  const { snapshot } = useAppContext();
  const { t } = useI18n();
  const [livePoints, setLivePoints] = useState<ChartPoint[]>([]);
  const [historyPoints, setHistoryPoints] = useState<ChartPoint[]>([]);
  const [mode, setMode] = useState<'live' | 'historical'>('live');
  const [selectedPreset, setSelectedPreset] = useState(0);
  const [granularity, setGranularity] = useState('raw');
  const [loading, setLoading] = useState(false);
  const lastTimeRef = useRef(0);

  // Find host in snapshot
  const host = useMemo(() => {
    if (!snapshot) return undefined;
    return snapshot.hosts.find((h) => h.ip === ip);
  }, [snapshot, ip]);

  // Live mode: accumulate per-second data points from host totals
  const prevBytesRef = useRef<{ sent: number; received: number } | null>(null);

  useEffect(() => {
    if (mode !== 'live' || !host) return;
    const now = Date.now();
    if (now - lastTimeRef.current < 800) return;
    lastTimeRef.current = now;

    const time = new Date().toLocaleTimeString();
    setLivePoints((prev) => {
      const next = [...prev, {
        time,
        sent: host.bytes_sent,
        received: host.bytes_received,
      }];
      if (next.length > MAX_LIVE_POINTS) return next.slice(-MAX_LIVE_POINTS);
      return next;
    });
  }, [snapshot, mode, host]);

  // Compute deltas for live chart to show rate
  const liveChartData = useMemo(() => {
    const result: { time: string; sent: number; received: number }[] = [];
    for (let i = 1; i < livePoints.length; i++) {
      const prev = livePoints[i - 1];
      const cur = livePoints[i];
      const elapsed = 1; // ~1s between snapshots
      result.push({
        time: cur.time,
        sent: Math.max(0, (cur.sent - prev.sent) / elapsed),
        received: Math.max(0, (cur.received - prev.received) / elapsed),
      });
    }
    return result;
  }, [livePoints]);

  const loadHistory = useCallback(async (minutes: number) => {
    if (minutes === 0) {
      setMode('live');
      setSelectedPreset(0);
      return;
    }
    setSelectedPreset(minutes);
    setMode('historical');
    setLoading(true);
    try {
      const to = new Date().toISOString();
      const from = new Date(Date.now() - minutes * 60 * 1000).toISOString();
      const data = await fetchHostTrafficHistory(ip, undefined, from, to, granularity);
      if (data.length === 0) {
        setHistoryPoints([]);
      } else {
        const pts: ChartPoint[] = [];
        for (let i = 1; i < data.length; i++) {
          const prev = data[i - 1];
          const cur = data[i];
          const elapsed = (new Date(cur.timestamp).getTime() - new Date(prev.timestamp).getTime()) / 1000;
          if (elapsed > 0) {
            pts.push({
              time: new Date(cur.timestamp).toLocaleTimeString(),
              sent: Math.max(0, (cur.bytes_sent - prev.bytes_sent) / elapsed),
              received: Math.max(0, (cur.bytes_received - prev.bytes_received) / elapsed),
            });
          }
        }
        setHistoryPoints(pts);
      }
    } catch {
      // keep empty
    }
    setLoading(false);
  }, [ip]);

  const hostFlows = useMemo(() => {
    if (!snapshot) return [];
    return snapshot.flows.filter((f) => f.src_ip === ip || f.dst_ip === ip);
  }, [snapshot, ip]);

  const totalBytes = hostFlows.reduce((sum, f) => sum + f.bytes, 0);
  const totalPackets = hostFlows.reduce((sum, f) => sum + f.packets, 0);

  const chartConfig = {
    sent: { label: 'Sent/s', color: 'hsl(var(--chart-1))' },
    received: { label: 'Received/s', color: 'hsl(var(--chart-2))' },
  };

  const chartData = mode === 'live' ? liveChartData : historyPoints;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.hosts.activeFlows}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-bold">{hostFlows.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.dashboard.total}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-bold">{formatBytes(totalBytes)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.dashboard.packets}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-bold">{totalPackets.toLocaleString()}</p>
          </CardContent>
        </Card>
      </div>

      {/* Traffic history chart */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-sm font-medium">{t.dashboard.trafficHistory}</CardTitle>
            <div className="flex items-center gap-2">
              <div className="flex gap-1">
                {PRESETS.map((p) => (
                  <Button
                    key={p.value}
                    variant={selectedPreset === p.value ? 'secondary' : 'ghost'}
                    size="sm"
                    className="h-7 text-xs px-2"
                    onClick={() => loadHistory(p.value)}
                  >
                    {p.labelKey ? t.dashboard.live : p.label}
                  </Button>
                ))}
              </div>
              {mode === 'historical' && (
                <Select value={granularity} onValueChange={setGranularity}>
                  <SelectTrigger className="h-7 text-xs w-[90px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="raw">Raw (10s)</SelectItem>
                    <SelectItem value="hourly">Hourly</SelectItem>
                    <SelectItem value="daily">Daily</SelectItem>
                    <SelectItem value="weekly">Weekly</SelectItem>
                  </SelectContent>
                </Select>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="h-[200px] flex items-center justify-center text-sm text-muted-foreground">
              {t.dashboard.loadingHistory}
            </div>
          ) : chartData.length > 0 ? (
            <ChartContainer config={chartConfig} className="h-[200px] w-full">
              <AreaChart data={chartData} margin={{ top: 0, right: 0, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="sentGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="var(--color-sent)" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="var(--color-sent)" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="recvGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="var(--color-received)" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="var(--color-received)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis dataKey="time" hide />
                <YAxis hide domain={['auto', 'auto']} />
                <ChartTooltip
                  content={({ active, payload }) => {
                    if (active && payload?.length) {
                      const data = payload[0].payload as ChartPoint;
                      return (
                        <div className="rounded-lg border bg-background p-2 shadow-sm">
                          <div className="text-xs text-muted-foreground">{data.time}</div>
                          <div className="text-xs font-medium">
                            {t.hosts.bytesSent}: {formatBytes(data.sent)}/s
                          </div>
                          <div className="text-xs font-medium">
                            {t.hosts.bytesReceived}: {formatBytes(data.received)}/s
                          </div>
                        </div>
                      );
                    }
                    return null;
                  }}
                />
                <Area
                  dataKey="sent"
                  stroke="var(--color-sent)"
                  fill="url(#sentGradient)"
                  isAnimationActive={false}
                />
                <Area
                  dataKey="received"
                  stroke="var(--color-received)"
                  fill="url(#recvGradient)"
                  isAnimationActive={false}
                />
              </AreaChart>
            </ChartContainer>
          ) : (
            <div className="h-[200px] flex items-center justify-center text-sm text-muted-foreground">
              {mode === 'historical' ? t.hosts.noFlows : t.dashboard.accumulating}
            </div>
          )}
          {mode === 'live' && host && (
            <p className="text-[10px] text-muted-foreground mt-1 text-center">
              {t.hosts.bytesSent}: {formatBytes(host.bytes_sent)} · {t.hosts.bytesReceived}: {formatBytes(host.bytes_received)}
            </p>
          )}
        </CardContent>
      </Card>

      {/* Active flows table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">{t.hosts.activeFlows} ({hostFlows.length})</CardTitle>
        </CardHeader>
        <CardContent>
          {hostFlows.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t.hosts.noFlows}</p>
          ) : (
            <div className="max-h-96 overflow-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs text-muted-foreground">
                    <th className="pb-2 font-medium">{t.flows.source}</th>
                    <th className="pb-2 font-medium">{t.flows.destination}</th>
                    <th className="pb-2 font-medium">{t.flows.protocol}</th>
                    <th className="pb-2 font-medium text-right">{t.dashboard.bytes}</th>
                    <th className="pb-2 font-medium text-right">{t.dashboard.packets}</th>
                  </tr>
                </thead>
                <tbody>
                  {hostFlows.map((f) => (
                    <tr key={f.id} className="border-b border-border/50">
                      <td className="py-1.5 font-mono text-xs">
                        {f.src_ip}:{f.src_port}
                      </td>
                      <td className="py-1.5 font-mono text-xs">
                        {f.dst_ip}:{f.dst_port}
                      </td>
                      <td className="py-1.5 text-xs">{f.app_protocol || f.protocol}</td>
                      <td className="py-1.5 text-xs text-right">{formatBytes(f.bytes)}</td>
                      <td className="py-1.5 text-xs text-right">{f.packets.toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
