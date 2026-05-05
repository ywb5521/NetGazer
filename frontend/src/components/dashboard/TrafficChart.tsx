import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ChartContainer, ChartTooltip } from '@/components/ui/chart';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { fetchTrafficHistory } from '@/lib/api';
import { formatBytes } from '@/lib/utils';
import { AreaChart, Area, XAxis, YAxis } from 'recharts';
import { useState, useEffect, useRef, useCallback } from 'react';

const MAX_POINTS = 120;

interface ChartPoint {
  time: string;
  bytes: number;
  packets: number;
  flows: number;
}

const PRESETS = [
  { labelKey: 'live' as const, value: 0 },
  { label: '15m', value: 15 },
  { label: '1h', value: 60 },
  { label: '6h', value: 360 },
  { label: '24h', value: 1440 },
  { labelKey: 'custom' as const, value: -1 },
];

function toLocalDatetime(iso: string) {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function TrafficChart() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();
  const [points, setPoints] = useState<ChartPoint[]>([]);
  const [comparePoints, setComparePoints] = useState<ChartPoint[] | null>(null);
  const [compareEnabled, setCompareEnabled] = useState(false);
  const [mode, setMode] = useState<'live' | 'historical'>('live');
  const [selectedPreset, setSelectedPreset] = useState(0);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [granularity, setGranularity] = useState('raw');
  const [customFrom, setCustomFrom] = useState(toLocalDatetime(new Date(Date.now() - 30 * 60 * 1000).toISOString()));
  const [customTo, setCustomTo] = useState(toLocalDatetime(new Date().toISOString()));
  const lastTimeRef = useRef(0);

  useEffect(() => {
    if (mode !== 'live' || !snapshot) return;
    const now = Date.now();
    if (now - lastTimeRef.current < 800) return;
    lastTimeRef.current = now;

    const time = new Date().toLocaleTimeString();
    setPoints((prev) => {
      const next = [...prev, {
        time,
        bytes: snapshot.traffic.bytes_per_sec,
        packets: snapshot.traffic.packets_per_sec,
        flows: snapshot.traffic.flows_count,
      }];
      if (next.length > MAX_POINTS) return next.slice(-MAX_POINTS);
      return next;
    });
  }, [snapshot, mode]);

  const loadHistory = useCallback(async (minutes: number, fromOverride?: string, toOverride?: string) => {
    if (minutes === 0) {
      setMode('live');
      setSelectedPreset(0);
      setComparePoints(null);
      setCompareEnabled(false);
      return;
    }
    if (minutes === -1) {
      setSelectedPreset(-1);
      setMode('historical');
      setComparePoints(null);
      setCompareEnabled(false);
      return;
    }
    setSelectedPreset(minutes);
    setMode('historical');
    setHistoryLoading(true);
    setComparePoints(null);
    setCompareEnabled(false);
    try {
      const to = toOverride ? new Date(toOverride) : new Date();
      const from = fromOverride ? new Date(fromOverride) : new Date(to.getTime() - minutes * 60 * 1000);
      const data = await fetchTrafficHistory(undefined, from.toISOString(), to.toISOString(), granularity);
      const pts: ChartPoint[] = data.map((d) => ({
        time: new Date(d.timestamp).toLocaleTimeString(),
        bytes: d.bytes_per_sec,
        packets: d.packets_per_sec,
        flows: d.flows_count,
      }));
      setPoints(pts);
    } catch {
      // keep current points
    }
    setHistoryLoading(false);
  }, []);

  const loadCustom = useCallback(() => {
    const from = new Date(customFrom);
    const to = new Date(customTo);
    loadHistory(0, from.toISOString(), to.toISOString());
  }, [customFrom, customTo, loadHistory]);

  const handleCompareToggle = useCallback(async (enabled: boolean) => {
    setCompareEnabled(enabled);
    if (!enabled) {
      setComparePoints(null);
      return;
    }
    if (mode !== 'historical' || points.length === 0) return;

    // Determine the comparison period: same duration, offset backward
    let mainFrom: Date;
    let mainTo: Date;

    if (selectedPreset === -1) {
      mainFrom = new Date(customFrom);
      mainTo = new Date(customTo);
    } else {
      mainTo = new Date();
      mainFrom = new Date(mainTo.getTime() - selectedPreset * 60 * 1000);
    }

    const duration = mainTo.getTime() - mainFrom.getTime();
    const cmpTo = new Date(mainFrom);
    const cmpFrom = new Date(cmpTo.getTime() - duration);

    try {
      const data = await fetchTrafficHistory(undefined, cmpFrom.toISOString(), cmpTo.toISOString(), granularity);
      const pts: ChartPoint[] = data.map((d) => ({
        time: new Date(d.timestamp).toLocaleTimeString(),
        bytes: d.bytes_per_sec,
        packets: d.packets_per_sec,
        flows: d.flows_count,
      }));
      setComparePoints(pts);
    } catch {
      setCompareEnabled(false);
    }
  }, [mode, selectedPreset, customFrom, customTo, points.length]);

  const chartConfig = {
    bytes: { label: 'Bytes/s', color: 'hsl(var(--chart-1))' },
    cmpBytes: { label: 'Baseline B/s', color: 'hsl(var(--chart-2))' },
    packets: { label: 'Packets/s', color: 'hsl(var(--chart-3))' },
    flows: { label: 'Flows', color: 'hsl(var(--chart-4))' },
  };

  return (
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
                  {p.labelKey ? t.dashboard[p.labelKey === 'custom' ? 'live' : p.labelKey] : p.label}
                </Button>
              ))}
            </div>
            {mode === 'historical' && (
              <Select value={granularity} onValueChange={(v) => { setGranularity(v); setComparePoints(null); setCompareEnabled(false); }}>
                <SelectTrigger className="h-7 text-xs w-[90px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="raw">{t.common.raw}</SelectItem>
                  <SelectItem value="hourly">{t.common.hourly}</SelectItem>
                  <SelectItem value="daily">{t.common.daily}</SelectItem>
                  <SelectItem value="weekly">{t.common.weekly}</SelectItem>
                </SelectContent>
              </Select>
            )}
          </div>
        </div>
        {selectedPreset === -1 && (
          <div className="flex items-center gap-2 mt-2">
            <Input
              type="datetime-local"
              value={customFrom}
              onChange={(e) => { setCustomFrom(e.target.value); setComparePoints(null); setCompareEnabled(false); }}
              className="h-7 text-xs w-44"
            />
            <span className="text-xs text-muted-foreground">-</span>
            <Input
              type="datetime-local"
              value={customTo}
              onChange={(e) => { setCustomTo(e.target.value); setComparePoints(null); setCompareEnabled(false); }}
              className="h-7 text-xs w-44"
            />
            <Button size="sm" className="h-7 text-xs" onClick={loadCustom}>
              {t.common.load}
            </Button>
          </div>
        )}
        {mode === 'historical' && points.length > 0 && (
          <div className="flex items-center gap-2 mt-2">
            <Switch
              checked={compareEnabled}
              onCheckedChange={handleCompareToggle}
            />
            <span className="text-xs text-muted-foreground">{t.common.compareWithPrev}</span>
          </div>
        )}
      </CardHeader>
      <CardContent>
        {historyLoading ? (
          <div className="h-[200px] flex items-center justify-center text-sm text-muted-foreground">
            {t.dashboard.loadingHistory}
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-[280px] w-full">
            <AreaChart data={points} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="bytesGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--color-bytes)" stopOpacity={0.45} />
                  <stop offset="100%" stopColor="var(--color-bytes)" stopOpacity={0.02} />
                </linearGradient>
                <linearGradient id="cmpBytesGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--color-cmpBytes)" stopOpacity={0.35} />
                  <stop offset="100%" stopColor="var(--color-cmpBytes)" stopOpacity={0.02} />
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
                        <div className="text-xs font-medium">{formatBytes(data.bytes)}/s</div>
                        <div className="text-xs text-muted-foreground">
                          {data.packets.toLocaleString()} pps · {data.flows} flows
                        </div>
                      </div>
                    );
                  }
                  return null;
                }}
              />
              <Area
                dataKey="bytes"
                stroke="var(--color-bytes)"
                strokeWidth={2}
                fill="url(#bytesGradient)"
                isAnimationActive={false}
              />
              {compareEnabled && comparePoints && (
                <Area
                  data={comparePoints}
                  dataKey="bytes"
                  stroke="var(--color-cmpBytes)"
                  strokeWidth={2}
                  fill="url(#cmpBytesGradient)"
                  strokeDasharray="4 4"
                  isAnimationActive={false}
                />
              )}
            </AreaChart>
          </ChartContainer>
        )}
        {mode === 'live' && (
          <p className="text-[10px] text-muted-foreground mt-1 text-center">
            {t.dashboard.liveStatus}: {formatBytes(snapshot?.traffic?.bytes_per_sec || 0)}/s · {snapshot?.traffic?.packets_per_sec?.toLocaleString() || 0} pps · {snapshot?.traffic?.flows_count || 0} flows
          </p>
        )}
        {compareEnabled && comparePoints && (
          <div className="flex items-center justify-center gap-4 mt-1">
            <span className="text-[10px] text-muted-foreground inline-flex items-center gap-1">
              <span className="inline-block h-2 w-4 rounded-sm" style={{ backgroundColor: 'var(--color-bytes)' }} />
              {t.common.current}
            </span>
            <span className="text-[10px] text-muted-foreground inline-flex items-center gap-1">
              <span className="inline-block h-0.5 w-4 rounded-sm" style={{ backgroundColor: 'var(--color-cmpBytes)', borderTop: '1px dashed var(--color-cmpBytes)' }} />
              {t.common.baseline}
            </span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
