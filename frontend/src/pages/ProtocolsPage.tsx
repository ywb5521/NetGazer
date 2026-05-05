import { useMemo, useEffect, useState, useCallback } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { getCategory, baseProtocol, CATEGORY_COLORS } from '@/lib/categories';
import { fetchProtocols, exportCSV, type PaginatedResponse } from '@/lib/api';
import { useLocalStorage } from '@/lib/useLocalStorage';
import { PieChart, Pie, Cell, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { RefreshCw, Download } from 'lucide-react';

const COLORS = [
  'hsl(var(--chart-1))',
  'hsl(var(--chart-2))',
  'hsl(var(--chart-3))',
  'hsl(var(--chart-4))',
  'hsl(var(--chart-5))',
  '#8b5cf6',
  '#06b6d4',
  '#f59e0b',
  '#ef4444',
  '#10b981',
];

export default function ProtocolsPage() {
  const { selectedNode, selectedInterface } = useAppContext();
  const { t } = useI18n();
  const [protoTab, setProtoTab] = useLocalStorage('gtopng-protocols-tab', 'chart');
  const [data, setData] = useState<PaginatedResponse<import('@/types').ProtocolStat> | null>(null);
  const [loading, setLoading] = useState(false);

  const loadProtocols = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetchProtocols(selectedNode || undefined, selectedInterface || undefined, 100, 0);
      setData(result);
    } catch {}
    setLoading(false);
  }, [selectedNode, selectedInterface]);

  useEffect(() => {
    loadProtocols();
  }, [loadProtocols]);

  const protocols = data?.items || [];
  const totalProtocols = data?.total || 0;

  const categories = useMemo(() => {
    const catMap: Record<string, { bytes: number; packets: number }> = {};
    for (const p of protocols) {
      const cat = getCategory(p.protocol);
      if (!catMap[cat]) catMap[cat] = { bytes: 0, packets: 0 };
      catMap[cat].bytes += p.bytes;
      catMap[cat].packets += p.packets;
    }
    const totalBytes = Object.values(catMap).reduce((s, c) => s + c.bytes, 0);
    return Object.entries(catMap)
      .map(([name, stats]) => ({
        name,
        bytes: stats.bytes,
        packets: stats.packets,
        percentage: totalBytes > 0 ? (stats.bytes / totalBytes) * 100 : 0,
      }))
      .sort((a, b) => b.bytes - a.bytes);
  }, [protocols]);

  const chartData = useMemo(() => {
    const protoMap: Record<string, { bytes: number; packets: number }> = {};
    for (const p of protocols) {
      const base = baseProtocol(p.protocol) || 'Unknown';
      if (!protoMap[base]) protoMap[base] = { bytes: 0, packets: 0 };
      protoMap[base].bytes += p.bytes;
      protoMap[base].packets += p.packets;
    }
    const totalBytes = Object.values(protoMap).reduce((s, c) => s + c.bytes, 0);
    const sorted = Object.entries(protoMap)
      .map(([name, stats]) => ({
        name,
        bytes: stats.bytes,
        packets: stats.packets,
        percentage: totalBytes > 0 ? (stats.bytes / totalBytes) * 100 : 0,
      }))
      .sort((a, b) => b.bytes - a.bytes);

    const TOP_N = 7;
    if (sorted.length <= TOP_N + 1) return sorted;

    const top = sorted.slice(0, TOP_N);
    const rest = sorted.slice(TOP_N);
    const otherBytes = rest.reduce((s, c) => s + c.bytes, 0);
    const otherPackets = rest.reduce((s, c) => s + c.packets, 0);
    top.push({
      name: 'Other',
      bytes: otherBytes,
      packets: otherPackets,
      percentage: totalBytes > 0 ? (otherBytes / totalBytes) * 100 : 0,
    });
    return top;
  }, [protocols]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t.protocols.title} ({totalProtocols})</h1>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={loadProtocols} disabled={loading}>
            <RefreshCw className={`mr-1 h-3 w-3 ${loading ? 'animate-spin' : ''}`} />
          </Button>
          <Button variant="outline" size="sm" disabled={protocols.length === 0} onClick={() => {
            const headers = ['Protocol', 'Category', 'Bytes', 'Packets', 'Percentage'];
            const rows = protocols.map((p) => [
              p.protocol, getCategory(p.protocol), String(p.bytes), String(p.packets),
              p.percentage.toFixed(2) + '%',
            ]);
            exportCSV(headers, rows, `protocols-${new Date().toISOString().slice(0, 10)}.csv`);
          }}>
            <Download className="mr-1 h-3 w-3" /> CSV
          </Button>
        </div>
      </div>

      <Tabs value={protoTab} onValueChange={setProtoTab}>
        <TabsList>
          <TabsTrigger value="chart">{t.protocols.chart}</TabsTrigger>
          <TabsTrigger value="categories">{t.protocols.categories}</TabsTrigger>
          <TabsTrigger value="table">{t.protocols.table}</TabsTrigger>
        </TabsList>

        <TabsContent value="chart">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">{t.protocols.title}</CardTitle>
            </CardHeader>
            <CardContent>
              {chartData.length > 0 ? (
                <div className="flex flex-col items-center gap-4">
                  <div className="w-full max-w-[500px]">
                    <ResponsiveContainer width="100%" aspect={1.4}>
                      <PieChart>
                        <Pie
                          data={chartData}
                          dataKey="bytes"
                          nameKey="name"
                          cx="50%"
                          cy="50%"
                          innerRadius="45%"
                          outerRadius="70%"
                          label={({ name, percentage }) => `${name} ${percentage.toFixed(0)}%`}
                          labelLine={{ strokeWidth: 1 }}
                          className="text-xs font-medium"
                        >
                          {chartData.map((c, i) => (
                            <Cell key={c.name} fill={COLORS[i % COLORS.length]} stroke="var(--background)" strokeWidth={2} />
                          ))}
                        </Pie>
                        <Tooltip
                          content={({ active, payload }) => {
                            if (active && payload?.length) {
                              const d = payload[0].payload;
                              return (
                                <div className="rounded-lg border bg-background p-2.5 shadow-md text-sm">
                                  <div className="font-semibold">{d.name}</div>
                                  <div className="text-muted-foreground">{formatBytes(d.bytes)}</div>
                                  <div>{d.percentage.toFixed(1)}%</div>
                                </div>
                              );
                            }
                            return null;
                          }}
                        />
                        <Legend
                          layout="horizontal"
                          verticalAlign="bottom"
                          wrapperStyle={{ fontSize: '12px', paddingTop: '12px' }}
                        />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground py-8 text-center">{loading ? t.common.loading : t.common.empty}</p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="categories">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">{t.protocols.categories}</CardTitle>
            </CardHeader>
            <CardContent>
              {categories.length > 0 ? (
                <div className="space-y-6">
                  <div className="w-full max-w-[500px] mx-auto">
                    <ResponsiveContainer width="100%" aspect={1.4}>
                      <PieChart>
                        <Pie
                          data={categories}
                          dataKey="bytes"
                          nameKey="name"
                          cx="50%"
                          cy="50%"
                          innerRadius="45%"
                          outerRadius="70%"
                          label={({ name, percentage }) => `${name} ${percentage.toFixed(0)}%`}
                          labelLine={{ strokeWidth: 1 }}
                          className="text-xs font-medium"
                        >
                          {categories.map((c, i) => (
                            <Cell key={c.name} fill={CATEGORY_COLORS[c.name] || COLORS[i % COLORS.length]} stroke="var(--background)" strokeWidth={2} />
                          ))}
                        </Pie>
                        <Tooltip
                          content={({ active, payload }) => {
                            if (active && payload?.length) {
                              const d = payload[0].payload;
                              return (
                                <div className="rounded-lg border bg-background p-2.5 shadow-md text-sm">
                                  <div className="font-semibold">{d.name}</div>
                                  <div className="text-muted-foreground">{formatBytes(d.bytes)}</div>
                                  <div>{d.percentage.toFixed(1)}%</div>
                                </div>
                              );
                            }
                            return null;
                          }}
                        />
                        <Legend
                          layout="horizontal"
                          verticalAlign="bottom"
                          wrapperStyle={{ fontSize: '12px', paddingTop: '12px' }}
                        />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-border text-left text-xs text-muted-foreground">
                          <th className="pb-2 font-medium">{t.protocols.categories}</th>
                          <th className="pb-2 font-medium text-right">{t.dashboard.bytes}</th>
                          <th className="pb-2 font-medium text-right">%</th>
                        </tr>
                      </thead>
                      <tbody>
                        {categories.map((c) => (
                          <tr key={c.name} className="border-b border-border/50">
                            <td className="py-1.5 text-xs">
                              <div className="flex items-center gap-2">
                                <div className="h-3 w-3 rounded-full" style={{ backgroundColor: CATEGORY_COLORS[c.name] || '#6b7280' }} />
                                {c.name}
                              </div>
                            </td>
                            <td className="py-1.5 text-xs text-right">{formatBytes(c.bytes)}</td>
                            <td className="py-1.5 text-xs text-right">
                              <div className="flex items-center justify-end gap-2">
                                <div className="h-1.5 w-12 rounded-full bg-muted overflow-hidden">
                                  <div className="h-full rounded-full bg-primary" style={{ width: `${c.percentage}%` }} />
                                </div>
                                {c.percentage.toFixed(1)}%
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground py-8 text-center">{loading ? t.common.loading : t.common.empty}</p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="table">
          <Card>
            <CardContent className="pt-6">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t.flows.protocol}</TableHead>
                    <TableHead className="text-right">{t.dashboard.bytes}</TableHead>
                    <TableHead className="text-right">{t.dashboard.packets}</TableHead>
                    <TableHead className="text-right">%</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {protocols.length > 0 ? (
                    protocols.map((p) => (
                      <TableRow key={p.protocol}>
                        <TableCell className="text-sm font-medium">{p.protocol}</TableCell>
                        <TableCell className="text-right text-xs">{formatBytes(p.bytes)}</TableCell>
                        <TableCell className="text-right text-xs">{p.packets.toLocaleString()}</TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-2">
                            <div className="h-2 w-16 rounded-full bg-muted overflow-hidden">
                              <div className="h-full rounded-full bg-chart-1" style={{ width: `${p.percentage}%` }} />
                            </div>
                            <span className="text-xs text-muted-foreground">{p.percentage.toFixed(1)}%</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell colSpan={4} className="text-center text-muted-foreground py-8">
                        {loading ? t.common.loading : t.common.empty}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
