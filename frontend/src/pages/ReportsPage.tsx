import { useState, useCallback } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { toast } from 'sonner';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, AreaChart, Area,
} from 'recharts';
import {
  fetchReportSummary, fetchReportTopTalkers, fetchReportTopProtocols,
  fetchReportAlerts, fetchReportTrend, downloadExport,
} from '@/lib/api';
import { useI18n } from '@/i18n/I18nContext';
import type { SummaryReport, TopTalker, TopProtocol, AlertSummary, TrendPoint } from '@/types';
import { formatBytes } from '@/lib/utils';

export default function ReportsPage() {
  const { t } = useI18n();
  const [from, setFrom] = useState(() => {
    const d = new Date();
    d.setHours(d.getHours() - 24);
    return d.toISOString().slice(0, 16);
  });
  const [to, setTo] = useState(() => new Date().toISOString().slice(0, 16));
  const [loading, setLoading] = useState(false);

  const [summary, setSummary] = useState<SummaryReport | null>(null);
  const [topTalkers, setTopTalkers] = useState<TopTalker[]>([]);
  const [topProtocols, setTopProtocols] = useState<TopProtocol[]>([]);
  const [alertSummary, setAlertSummary] = useState<AlertSummary | null>(null);
  const [trend, setTrend] = useState<TrendPoint[]>([]);

  const generate = useCallback(async () => {
    setLoading(true);
    const fromISO = new Date(from).toISOString();
    const toISO = new Date(to).toISOString();

    try {
      const [s, talkers, protocols, alerts, trendData] = await Promise.all([
        fetchReportSummary(undefined, fromISO, toISO),
        fetchReportTopTalkers(undefined, fromISO, toISO, 15),
        fetchReportTopProtocols(undefined, fromISO, toISO, 10),
        fetchReportAlerts(undefined, fromISO, toISO),
        fetchReportTrend(undefined, fromISO, toISO),
      ]);
      setSummary(s);
      setTopTalkers(talkers);
      setTopProtocols(protocols);
      setAlertSummary(alerts);
      setTrend(trendData);
      toast.success(t.reports.reportGenerated);
    } catch (e: any) {
      toast.error(e.message || t.reports.reportFailed);
    } finally {
      setLoading(false);
    }
  }, [from, to]);

  const handleExport = async (type: 'snapshots' | 'hosts' | 'alerts', format: string) => {
    try {
      await downloadExport(type, format, undefined, new Date(from).toISOString(), new Date(to).toISOString());
      toast.success(`${t.reports.exportSuccess} ${type} as ${format}`);
    } catch (e: any) {
      toast.error(e.message || t.reports.exportFailed);
    }
  };

  const trendData = trend.map((p) => ({
    time: new Date(p.timestamp).toLocaleTimeString(),
    bps: p.bytes_per_sec,
    pps: p.packets_per_sec,
  }));

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t.reports.title}</h1>
        <p className="text-muted-foreground">{t.reports.description}</p>
      </div>

      {/* Date Range & Actions */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap items-end gap-4">
            <div className="space-y-1">
              <label htmlFor="from" className="text-sm font-medium">{t.reports.from}</label>
              <Input id="from" type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} />
            </div>
            <div className="space-y-1">
              <label htmlFor="to" className="text-sm font-medium">{t.reports.to}</label>
              <Input id="to" type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} />
            </div>
            <Button onClick={generate} disabled={loading}>
              {loading ? t.reports.generating : t.reports.generateReport}
            </Button>
            <div className="ml-auto flex items-center gap-2">
              <span className="text-sm text-muted-foreground">{t.reports.export}</span>
              <Button variant="outline" size="sm" onClick={() => handleExport('snapshots', 'json')}>JSON</Button>
              <Button variant="outline" size="sm" onClick={() => handleExport('snapshots', 'csv')}>CSV</Button>
              <Button variant="outline" size="sm" onClick={() => handleExport('snapshots', 'ndjson')}>NDJSON</Button>
              <Button variant="outline" size="sm" onClick={() => handleExport('snapshots', 'clickhouse')}>ClickHouse</Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Summary Cards */}
      {summary && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">{t.reports.totalTraffic}</CardTitle></CardHeader>
            <CardContent><div className="text-2xl font-bold">{formatBytes(summary.total_bytes)}</div></CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">{t.reports.avgThroughput}</CardTitle></CardHeader>
            <CardContent><div className="text-2xl font-bold">{formatBytes(summary.avg_bps)}/s</div></CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">{t.reports.peakThroughput}</CardTitle></CardHeader>
            <CardContent><div className="text-2xl font-bold">{formatBytes(summary.peak_bps)}/s</div></CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">{t.reports.uniqueHosts}</CardTitle></CardHeader>
            <CardContent><div className="text-2xl font-bold">{summary.unique_hosts.toLocaleString()}</div></CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">{t.reports.alerts}</CardTitle></CardHeader>
            <CardContent><div className="text-2xl font-bold">{summary.alert_count.toLocaleString()}</div></CardContent>
          </Card>
        </div>
      )}

      {/* Trend Chart */}
      {trend.length > 0 && (
        <Card>
          <CardHeader><CardTitle>{t.reports.trafficTrend}</CardTitle></CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={280}>
              <AreaChart data={trendData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" fontSize={11} />
                <YAxis tickFormatter={(v) => formatBytes(v)} fontSize={11} />
                <Tooltip formatter={(v: number) => [formatBytes(v) + '/s', t.reports.throughput]} />
                <Area type="monotone" dataKey="bps" stroke="hsl(var(--primary))" fill="hsl(var(--primary)/.15)" />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      )}

      {/* Top Talkers + Top Protocols */}
      <div className="grid gap-6 lg:grid-cols-2">
        {topTalkers.length > 0 && (
          <Card>
            <CardHeader><CardTitle>{t.reports.topTalkers}</CardTitle></CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t.reports.ip}</TableHead>
                    <TableHead className="text-right">{t.reports.total}</TableHead>
                    <TableHead className="text-right">{t.reports.sent}</TableHead>
                    <TableHead className="text-right">{t.reports.received}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {topTalkers.map((t) => (
                    <TableRow key={t.ip}>
                      <TableCell className="font-mono text-xs">{t.ip}</TableCell>
                      <TableCell className="text-right">{formatBytes(t.total_bytes)}</TableCell>
                      <TableCell className="text-right text-green-600">{formatBytes(t.bytes_sent)}</TableCell>
                      <TableCell className="text-right text-blue-600">{formatBytes(t.bytes_received)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}

        {topProtocols.length > 0 && (
          <Card>
            <CardHeader><CardTitle>{t.reports.topProtocols}</CardTitle></CardHeader>
            <CardContent>
              <ResponsiveContainer width="100%" height={280}>
                <BarChart data={topProtocols.map(p => ({ name: p.name, bytes: p.total_bytes, pct: p.pct_bytes }))} layout="vertical">
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis type="number" tickFormatter={(v) => formatBytes(v)} fontSize={11} />
                  <YAxis type="category" dataKey="name" width={80} fontSize={11} />
                  <Tooltip formatter={(v: number) => [formatBytes(v), t.reports.traffic]} />
                  <Bar dataKey="bytes" fill="hsl(var(--chart-2))" />
                </BarChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Alert Summary */}
      {alertSummary && alertSummary.total > 0 && (
        <Card>
          <CardHeader><CardTitle>{t.reports.alertSummary} ({alertSummary.total} {t.reports.total})</CardTitle></CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-3">
              <div>
                <h4 className="text-sm font-medium mb-2">{t.reports.byType}</h4>
                <Table>
                  <TableHeader><TableRow><TableHead>{t.reports.type}</TableHead><TableHead className="text-right">{t.reports.count}</TableHead></TableRow></TableHeader>
                  <TableBody>
                    {Object.entries(alertSummary.by_type).map(([type, count]) => (
                      <TableRow key={type}><TableCell className="text-xs">{type}</TableCell><TableCell className="text-right">{count}</TableCell></TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <div>
                <h4 className="text-sm font-medium mb-2">{t.reports.bySeverity}</h4>
                <Table>
                  <TableHeader><TableRow><TableHead>{t.reports.severity}</TableHead><TableHead className="text-right">{t.reports.count}</TableHead></TableRow></TableHeader>
                  <TableBody>
                    {Object.entries(alertSummary.by_severity).map(([sev, count]) => (
                      <TableRow key={sev}>
                        <TableCell className="text-xs">
                          <span className={`inline-block w-2 h-2 rounded-full mr-1.5 ${sev === 'critical' ? 'bg-red-500' : sev === 'warning' ? 'bg-yellow-500' : 'bg-blue-500'}`} />
                          {sev}
                        </TableCell>
                        <TableCell className="text-right">{count}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <div>
                <h4 className="text-sm font-medium mb-2">{t.reports.recentAlerts}</h4>
                <div className="space-y-1 max-h-48 overflow-y-auto">
                  {alertSummary.recent.map((a) => (
                    <div key={a.id} className="text-xs py-1 border-b">
                      <span className="font-medium">{a.type}</span>
                      <span className="text-muted-foreground ml-2">{new Date(a.timestamp).toLocaleString()}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
