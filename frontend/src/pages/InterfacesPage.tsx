import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Input } from '@/components/ui/input';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { fetchAllInterfaces, fetchHosts, type InterfaceSummary, type PaginatedResponse } from '@/lib/api';
import { Search, Cable, ChevronDown, ChevronRight, ExternalLink } from 'lucide-react';
import type { Host } from '@/types';

export default function InterfacesPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [interfaces, setInterfaces] = useState<InterfaceSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [expanded, setExpanded] = useState<string | null>(null);
  const [detailHosts, setDetailHosts] = useState<Record<string, Host[]>>({});
  const [detailLoading, setDetailLoading] = useState(false);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      try {
        const result = await fetchAllInterfaces();
        setInterfaces(Array.isArray(result) ? result : []);
      } catch {
        setInterfaces([]);
      }
      setLoading(false);
    };
    load();
    const interval = setInterval(load, 15000);
    return () => clearInterval(interval);
  }, []);

  const filtered = useMemo(() => {
    if (!search) return interfaces;
    const q = search.toLowerCase();
    return interfaces.filter(
      (i) => i.node_id.toLowerCase().includes(q) || i.name.toLowerCase().includes(q),
    );
  }, [interfaces, search]);

  const totalBps = useMemo(() => filtered.reduce((s, i) => s + i.bytes_per_sec, 0), [filtered]);

  const handleToggle = async (iface: InterfaceSummary) => {
    const key = `${iface.node_id}/${iface.name}`;
    if (expanded === key) {
      setExpanded(null);
      return;
    }
    setExpanded(key);

    if (!detailHosts[key]) {
      setDetailLoading(true);
      try {
        const result = await fetchHosts({
          nodeId: iface.node_id,
          iface: iface.name,
          sort: 'bytes-desc',
          limit: 10,
        });
        setDetailHosts((prev) => ({ ...prev, [key]: result.items }));
      } catch {
        setDetailHosts((prev) => ({ ...prev, [key]: [] }));
      }
      setDetailLoading(false);
    }
  };

  const handleViewFlows = (iface: InterfaceSummary) => {
    navigate(`/flows?interface=${encodeURIComponent(iface.name)}&node_id=${encodeURIComponent(iface.node_id)}`);
  };

  const handleViewHost = (ip: string) => {
    navigate(`/hosts/${ip}`);
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
        <Cable className="h-6 w-6" />
        {t.interfaces.title} ({interfaces.length})
      </h1>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          className="pl-9 h-9"
          placeholder="搜索节点或网卡名..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {loading && filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground py-8 text-center">{t.interfaces.loading}</p>
      ) : filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground py-8 text-center">{t.interfaces.empty}</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((iface, idx) => {
            const key = `${iface.node_id}/${iface.name}`;
            const isExpanded = expanded === key;
            const hosts = detailHosts[key] || [];
            return (
              <Card
                key={`${iface.node_id}-${iface.name}-${idx}`}
                className={`hover:shadow-md transition-all ${isExpanded ? 'shadow-md ring-1 ring-primary/20 md:col-span-full lg:col-span-full' : ''}`}
              >
                <CardHeader className="pb-2 cursor-pointer" onClick={() => handleToggle(iface)}>
                  <CardTitle className="text-sm flex items-center justify-between">
                    <span className="flex items-center gap-2 truncate">
                      {isExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
                      {iface.name}
                    </span>
                    <span className="text-xs text-muted-foreground font-normal">{iface.node_id}</span>
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-2 gap-3 text-xs">
                    <div>
                      <span className="text-muted-foreground">{t.interfaces.throughput}</span>
                      <p className="font-semibold text-sm">{formatBytes(iface.bytes_per_sec)}/s</p>
                    </div>
                    <div>
                      <span className="text-muted-foreground">{t.interfaces.packets}</span>
                      <p className="font-semibold text-sm">{iface.packets_per_sec.toLocaleString()}/s</p>
                    </div>
                    <div>
                      <span className="text-muted-foreground">{t.interfaces.hosts}</span>
                      <p className="font-semibold text-sm">{iface.hosts_count}</p>
                    </div>
                    <div>
                      <span className="text-muted-foreground">{t.interfaces.flows}</span>
                      <p className="font-semibold text-sm">{iface.flows_count}</p>
                    </div>
                  </div>
                  {totalBps > 0 && (
                    <div className="mt-3 pt-3 border-t border-border">
                      <div className="h-1.5 rounded-full bg-muted overflow-hidden">
                        <div
                          className="h-full rounded-full bg-chart-1"
                          style={{ width: `${Math.min((iface.bytes_per_sec / totalBps) * 100, 100)}%` }}
                        />
                      </div>
                      <p className="text-xs text-muted-foreground mt-1">
                        {((iface.bytes_per_sec / totalBps) * 100).toFixed(1)}% {t.geo.percentage}
                      </p>
                    </div>
                  )}

                  {/* Expanded detail */}
                  {isExpanded && (
                    <div className="mt-4 pt-4 border-t border-border space-y-4">
                      {detailLoading ? (
                        <p className="text-xs text-muted-foreground text-center py-4">{t.common.loading}</p>
                      ) : (
                        <>
                          {hosts.length > 0 && (
                            <div>
                              <p className="text-xs font-semibold mb-2">{t.dashboard.topHosts}</p>
                              <Table>
                                <TableHeader>
                                  <TableRow>
                                    <TableHead className="text-xs">{t.dashboard.ip}</TableHead>
                                    <TableHead className="text-xs">{t.dashboard.hostname}</TableHead>
                                    <TableHead className="text-xs text-right">{t.dashboard.bytes}</TableHead>
                                  </TableRow>
                                </TableHeader>
                                <TableBody>
                                  {hosts.map((h) => (
                                    <TableRow
                                      key={h.ip}
                                      className="cursor-pointer hover:bg-muted/50"
                                      onClick={() => handleViewHost(h.ip)}
                                    >
                                      <TableCell className="font-mono text-xs">{h.ip}</TableCell>
                                      <TableCell className="text-xs">{h.hostname || '-'}</TableCell>
                                      <TableCell className="text-xs text-right font-medium">
                                        {formatBytes(h.bytes_sent + h.bytes_received)}
                                      </TableCell>
                                    </TableRow>
                                  ))}
                                </TableBody>
                              </Table>
                            </div>
                          )}
                          <div className="flex gap-2">
                            <button
                              onClick={(e) => { e.stopPropagation(); handleViewFlows(iface); }}
                              className="text-xs text-primary hover:underline flex items-center gap-1"
                            >
                              <ExternalLink className="h-3 w-3" />
                              查看该网卡的所有流
                            </button>
                          </div>
                        </>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
