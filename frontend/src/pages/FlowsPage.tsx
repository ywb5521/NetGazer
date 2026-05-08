import { useMemo, useState, useCallback, useEffect } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes, formatDuration } from '@/lib/utils';
import { fetchFlows, exportCSV, type PaginatedResponse } from '@/lib/api';
import { protocolMatches } from '@/lib/categories';
import { useLocalStorage } from '@/lib/useLocalStorage';
import { Search, X, Download, Filter, ArrowUpDown, RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react';
import type { Flow } from '@/types';

const PAGE_SIZE = 50;

const PROTOCOL_FILTERS = ['all', 'TCP', 'UDP', 'ICMP'];
const SORT_OPTIONS = [
  { value: 'bytes-desc', labelKey: 'sortBytesDesc' },
  { value: 'bytes-asc', labelKey: 'sortBytesAsc' },
  { value: 'packets-desc', labelKey: 'sortPacketsDesc' },
  { value: 'newest', labelKey: 'sortRecentActive' },
] as const;

type QuickFilter = { label: string; app?: string[]; minBytes?: number };

function getQuickFilters(t: any): QuickFilter[] {
  return [
    { label: t.flows.web, app: ['HTTP', 'HTTPS', 'HTTP2', 'QUIC', 'TLS', 'WebSocket'] },
    { label: t.flows.dns, app: ['DNS'] },
    { label: t.flows.sshRdp, app: ['SSH', 'RDP'] },
    { label: t.flows.gt1mb, minBytes: 1_000_000 },
  ];
}

function formatAppProto(raw: string): { proto: string; host?: string } {
  const encryptedSuffix = raw.endsWith(' (Encrypted)');
  const cleaned = encryptedSuffix ? raw.slice(0, -' (Encrypted)'.length) : raw;
  const m = cleaned.match(/^(.+?)\s*\((.+?)\)$/);
  if (m) return { proto: encryptedSuffix ? `${m[1]} (Encrypted)` : m[1], host: m[2] };
  return { proto: encryptedSuffix ? `${cleaned} (Encrypted)` : cleaned };
}

export default function FlowsPage() {
  const { selectedNode, selectedInterface } = useAppContext();
  const { t } = useI18n();
  const [protocolFilter, setProtocolFilter] = useLocalStorage('netgazer-flows-protocol', 'all');
  const [appFilter, setAppFilter] = useLocalStorage('netgazer-flows-app', 'all');
  const [sortBy, setSortBy] = useLocalStorage('netgazer-flows-sort', 'bytes-desc');
  const [minBytes, setMinBytes] = useLocalStorage('netgazer-flows-minbytes', '');
  const [search, setSearch] = useLocalStorage('netgazer-flows-search', '');
  const [activeQuick, setActiveQuick] = useLocalStorage<string | null>('netgazer-flows-quick', null);
  const [selectedFlow, setSelectedFlow] = useState<Flow | null>(null);
  const [page, setPage] = useState(0);
  const [data, setData] = useState<PaginatedResponse<Flow> | null>(null);
  const [loading, setLoading] = useState(false);

  const loadFlows = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetchFlows({
        nodeId: selectedNode || undefined,
        iface: selectedInterface || undefined,
        search,
        protocol: protocolFilter !== 'all' ? protocolFilter : undefined,
        app: appFilter !== 'all' ? appFilter : undefined,
        sort: sortBy,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      });
      setData(result);
    } catch {
      // keep old data
    }
    setLoading(false);
  }, [selectedNode, selectedInterface, search, protocolFilter, appFilter, sortBy, page]);

  useEffect(() => {
    loadFlows();
  }, [loadFlows]);

  // Apply client-side minBytes and quick filters on top of server results
  const flows = useMemo(() => {
    let list = data?.items || [];
    if (minBytes) {
      const threshold = parseInt(minBytes, 10);
      if (!isNaN(threshold) && threshold > 0) {
        list = list.filter((f) => f.bytes >= threshold);
      }
    }
    if (activeQuick) {
      const qf = getQuickFilters(t).find((q) => q.label === activeQuick);
      if (qf) {
        if ('app' in qf && qf.app) {
          list = list.filter((f) => protocolMatches(f.app_protocol, qf.app!));
        }
        if ('minBytes' in qf && qf.minBytes) {
          list = list.filter((f) => f.bytes >= qf.minBytes!);
        }
      }
    }
    return list;
  }, [data, minBytes, activeQuick]);

  const totalFlows = data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(totalFlows / PAGE_SIZE));

  const allAppProtocols = useMemo(() => {
    if (!data) return [];
    const apps = new Set(data.items.map((f) => f.app_protocol).filter(Boolean));
    return Array.from(apps).sort();
  }, [data]);

  const handleQuickFilter = useCallback((label: string) => {
    setActiveQuick((prev: string | null) => (prev === label ? null : label));
  }, [setActiveQuick]);

  const handleExport = useCallback(() => {
    const headers = ['Source', 'Destination', 'Protocol', 'App', 'Packets', 'Bytes', 'Duration', 'Node'];
    const rows = flows.map((f) => [
      `${f.src_ip}:${f.src_port}`,
      `${f.dst_ip}:${f.dst_port}`,
      f.protocol,
      f.app_protocol,
      String(f.packets),
      String(f.bytes),
      formatDuration(f.first_seen, f.last_seen),
      f.node_id,
    ]);
    exportCSV(headers, rows, `flows-${new Date().toISOString().slice(0, 10)}.csv`);
  }, [flows]);

  const activeFilters = [
    protocolFilter !== 'all',
    appFilter !== 'all',
    !!minBytes,
    !!activeQuick,
  ].filter(Boolean).length;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t.flows.title} ({totalFlows})</h1>
        <div className="flex gap-2">
          <div className="relative w-56">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder={t.flows.searchPlaceholder}
              value={search}
              onChange={(e) => { setSearch(e.target.value); setPage(0); }}
              className="pl-8 pr-8"
            />
            {search && (
              <button className="absolute right-2 top-2.5" onClick={() => { setSearch(''); setPage(0); }}>
                <X className="h-4 w-4 text-muted-foreground" />
              </button>
            )}
          </div>
          <Button variant="outline" size="sm" onClick={loadFlows} disabled={loading}>
            <RefreshCw className={`mr-1 h-3 w-3 ${loading ? 'animate-spin' : ''}`} />
          </Button>
          <Button variant="outline" size="sm" onClick={handleExport} disabled={flows.length === 0}>
            <Download className="mr-1 h-3 w-3" /> CSV
          </Button>
        </div>
      </div>

      {/* Quick filter chips */}
      <div className="flex flex-wrap items-center gap-2">
        <Filter className="h-3.5 w-3.5 text-muted-foreground" />
        {getQuickFilters(t).map((qf) => (
          <Badge
            key={qf.label}
            variant={activeQuick === qf.label ? 'default' : 'outline'}
            className="cursor-pointer text-xs hover:bg-accent"
            onClick={() => handleQuickFilter(qf.label)}
          >
            {qf.label}
          </Badge>
        ))}
        {activeFilters > 0 && (
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-xs text-muted-foreground"
            onClick={() => {
              setProtocolFilter('all');
              setAppFilter('all');
              setMinBytes('');
              setActiveQuick(null);
            }}
          >
            <X className="mr-1 h-3 w-3" />
            {t.flows.clear} ({activeFilters})
          </Button>
        )}
      </div>

      {/* Advanced filters */}
      <div className="flex flex-wrap items-center gap-2">
        <Select value={protocolFilter} onValueChange={(v) => { setProtocolFilter(v); setPage(0); }}>
          <SelectTrigger className="h-8 w-28 text-xs">
            <SelectValue placeholder="Protocol" />
          </SelectTrigger>
          <SelectContent>
            {PROTOCOL_FILTERS.map((p) => (
              <SelectItem key={p} value={p}>{p === 'all' ? t.flows.allProto : p}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={appFilter} onValueChange={(v) => { setAppFilter(v); setPage(0); }}>
          <SelectTrigger className="h-8 w-36 text-xs">
            <SelectValue placeholder="App Protocol" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t.flows.allApps}</SelectItem>
            {allAppProtocols.map((app) => (
              <SelectItem key={app} value={app}>{app}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={sortBy} onValueChange={(v) => { setSortBy(v); setPage(0); }}>
          <SelectTrigger className="h-8 w-28 text-xs">
            <ArrowUpDown className="mr-1 h-3 w-3" />
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SORT_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>{t.flows[opt.labelKey]}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="relative">
          <Input
            type="number"
            placeholder={t.flows.minBytes}
            value={minBytes}
            onChange={(e) => setMinBytes(e.target.value)}
            className="h-8 w-24 text-xs"
            min={0}
          />
        </div>
      </div>

      {/* Table */}
      <Card>
        <CardContent className="pt-6">
          {loading && flows.length === 0 ? (
            <p className="text-center text-muted-foreground py-8 text-sm">{t.common.loading}</p>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t.flows.source}</TableHead>
                    <TableHead>{t.flows.destination}</TableHead>
                    <TableHead>{t.flows.protocol}</TableHead>
                    <TableHead>{t.flows.app}</TableHead>
                    <TableHead>{t.flows.vlan}</TableHead>
                    <TableHead className="text-right">{t.dashboard.packets}</TableHead>
                    <TableHead className="text-right">{t.dashboard.bytes}</TableHead>
                    <TableHead>{t.flows.duration}</TableHead>
                    <TableHead>{t.flows.node}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {flows.length > 0 ? (
                    flows.map((flow) => (
                      <TableRow
                        key={flow.id}
                        className="cursor-pointer hover:bg-muted/50"
                        onClick={() => setSelectedFlow(flow)}
                      >
                        <TableCell className="font-mono text-xs">
                          {flow.src_ip}:{flow.src_port}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {flow.dst_ip}:{flow.dst_port}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="text-xs">{flow.protocol}</Badge>
                        </TableCell>
                        <TableCell>
                          {(() => {
                            const fp = formatAppProto(flow.app_protocol);
                            return (
                              <span className="inline-flex items-center gap-1">
                                <Badge className="text-xs">{fp.proto}</Badge>
                                {fp.host && <span className="text-[10px] text-muted-foreground truncate max-w-[120px]">{fp.host}</span>}
                              </span>
                            );
                          })()}
                        </TableCell>
                        <TableCell className="text-xs">
                          {flow.vlan_id ? <Badge variant="secondary" className="text-[10px]">{flow.vlan_id}</Badge> : '-'}
                        </TableCell>
                        <TableCell className="text-right text-xs font-mono">
                          {flow.packets.toLocaleString()}
                        </TableCell>
                        <TableCell className="text-right text-xs font-mono">
                          {formatBytes(flow.bytes)}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {formatDuration(flow.first_seen, flow.last_seen)}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {flow.node_id}
                        </TableCell>
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
                        {t.flows.noFlows || t.common.empty}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
              {totalPages > 1 && (
                <div className="flex items-center justify-between mt-4 pt-4 border-t border-border">
                  <span className="text-xs text-muted-foreground">
                    {t.flows.page} {page + 1} {t.flows.of} {totalPages} ({totalFlows} {t.flows.title})
                  </span>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
                      <ChevronLeft className="h-4 w-4" /> {t.flows.prev}
                    </Button>
                    <Button variant="outline" size="sm" disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>
                      {t.flows.next} <ChevronRight className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {/* Flow detail sheet */}
      <Sheet open={!!selectedFlow} onOpenChange={() => setSelectedFlow(null)}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{t.flows.flowDetail}</SheetTitle>
          </SheetHeader>
          {selectedFlow && (
            <div className="mt-6 space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-muted-foreground">{t.flows.source}</label>
                  <p className="font-mono text-sm">{selectedFlow.src_ip}:{selectedFlow.src_port}</p>
                </div>
                <div>
                  <label className="text-xs text-muted-foreground">{t.flows.destination}</label>
                  <p className="font-mono text-sm">{selectedFlow.dst_ip}:{selectedFlow.dst_port}</p>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-muted-foreground">{t.flows.protocol}</label>
                  <p><Badge variant="outline" className="text-xs">{selectedFlow.protocol}</Badge></p>
                </div>
                <div>
                  <label className="text-xs text-muted-foreground">{t.flows.app}</label>
                  <p>
                    {(() => {
                      const fp = formatAppProto(selectedFlow.app_protocol);
                      return (
                        <span className="inline-flex items-center gap-1">
                          <Badge className="text-xs">{fp.proto}</Badge>
                          {fp.host && <span className="text-xs text-muted-foreground">{fp.host}</span>}
                        </span>
                      );
                    })()}
                  </p>
                </div>
              </div>
              <hr className="border-border" />
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-muted-foreground">{t.dashboard.bytes}</label>
                  <p className="text-sm font-medium">{formatBytes(selectedFlow.bytes)}</p>
                </div>
                <div>
                  <label className="text-xs text-muted-foreground">{t.dashboard.packets}</label>
                  <p className="text-sm font-medium">{selectedFlow.packets.toLocaleString()}</p>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-muted-foreground">{t.hosts.firstSeen}</label>
                  <p className="text-sm">{new Date(selectedFlow.first_seen).toLocaleString()}</p>
                </div>
                <div>
                  <label className="text-xs text-muted-foreground">{t.hosts.lastSeen}</label>
                  <p className="text-sm">{new Date(selectedFlow.last_seen).toLocaleString()}</p>
                </div>
              </div>
              <div>
                <label className="text-xs text-muted-foreground">{t.flows.flowId}</label>
                <p className="font-mono text-xs text-muted-foreground break-all">{selectedFlow.id}</p>
              </div>
              <div>
                <label className="text-xs text-muted-foreground">{t.flows.node}</label>
                <p className="text-sm">{selectedFlow.node_id}</p>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
