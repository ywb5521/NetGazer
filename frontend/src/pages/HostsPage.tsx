import { useState, useMemo, useCallback, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes, countryToFlag } from '@/lib/utils';
import { fetchHosts, exportCSV, type PaginatedResponse } from '@/lib/api';
import { useLocalStorage } from '@/lib/useLocalStorage';
import { Search, Download, ChevronRight, ChevronDown, ChevronLeft, RefreshCw, X } from 'lucide-react';
import type { Host } from '@/types';

const PAGE_SIZE = 50;

const CATEGORY_VARIANT: Record<string, 'default' | 'secondary' | 'outline'> = {
  Localhost: 'secondary',
  Local: 'default',
  Remote: 'outline',
  Multicast: 'default',
  Broadcast: 'default',
};

function subnet24(ip: string): string {
  const lastDot = ip.lastIndexOf('.');
  if (lastDot < 0) return ip;
  return ip.slice(0, lastDot) + '.0/24';
}

interface SubnetGroup {
  subnet: string;
  hosts: Host[];
  totalBytes: number;
  totalPackets: number;
  totalFlows: number;
}

export default function HostsPage() {
  const { selectedNode, selectedInterface } = useAppContext();
  const { t } = useI18n();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [search, setSearch] = useLocalStorage('gtopng-hosts-search', '');
  const [hostsTab, setHostsTab] = useLocalStorage('gtopng-hosts-tab', 'flat');
  const [expandedSubnet, setExpandedSubnet] = useState<string | null>(null);
  const [page, setPage] = useState(0);
  const [data, setData] = useState<PaginatedResponse<Host> | null>(null);
  const [loading, setLoading] = useState(false);

  const countryFilter = searchParams.get('country') || '';
  const asnFilter = searchParams.get('asn') || '';

  const loadHosts = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetchHosts({
        nodeId: selectedNode || undefined,
        iface: selectedInterface || undefined,
        search: hostsTab === 'flat' ? search : '',
        country: countryFilter || undefined,
        asn: asnFilter || undefined,
        sort: 'bytes-desc',
        limit: hostsTab === 'flat' ? PAGE_SIZE : 200,
        offset: hostsTab === 'flat' ? page * PAGE_SIZE : 0,
      });
      setData(result);
    } catch {
      // keep old data
    }
    setLoading(false);
  }, [selectedNode, selectedInterface, search, countryFilter, asnFilter, hostsTab, page]);

  const clearFilters = () => {
    const newParams = new URLSearchParams(searchParams);
    newParams.delete('country');
    newParams.delete('asn');
    setSearchParams(newParams, { replace: true });
    setPage(0);
  };

  useEffect(() => {
    loadHosts();
  }, [loadHosts]);

  const hosts = data?.items || [];
  const totalHosts = data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(totalHosts / PAGE_SIZE));

  const subnetGroups = useMemo(() => {
    const groups: Record<string, SubnetGroup> = {};
    for (const h of hosts) {
      const sub = subnet24(h.ip);
      if (!groups[sub]) {
        groups[sub] = { subnet: sub, hosts: [], totalBytes: 0, totalPackets: 0, totalFlows: 0 };
      }
      groups[sub].hosts.push(h);
      groups[sub].totalBytes += h.bytes_sent + h.bytes_received;
      groups[sub].totalPackets += h.packets_sent + h.packets_received;
      groups[sub].totalFlows += h.active_flows;
    }
    return Object.values(groups).sort((a, b) => b.totalBytes - a.totalBytes);
  }, [hosts]);

  const haveHosts = hosts.length > 0;

  const handleExport = useCallback(() => {
    const headers = ['IP', 'MAC', 'Hostname', 'Vendor', 'Category', 'Country', 'Node', 'Bytes Sent', 'Bytes Received', 'Total', 'Active Flows'];
    const rows = hosts.map((h) => [
      h.ip, h.mac, h.hostname || '', h.vendor || '', h.category || '', h.country || '',
      h.node_id, String(h.bytes_sent), String(h.bytes_received),
      String(h.bytes_sent + h.bytes_received), String(h.active_flows),
    ]);
    exportCSV(headers, rows, `hosts-${new Date().toISOString().slice(0, 10)}.csv`);
  }, [hosts]);

  const hostRow = (host: Host) => (
    <TableRow
      key={host.ip + host.node_id}
      className="cursor-pointer hover:bg-muted/50"
      onClick={() => navigate(`/hosts/${host.ip}`)}
    >
      <TableCell className="font-mono text-xs">{host.ip}</TableCell>
      <TableCell className="font-mono text-xs text-muted-foreground">
        {host.mac || '-'}
      </TableCell>
      <TableCell className="text-xs">{host.hostname || '-'}</TableCell>
      <TableCell className="text-xs">{host.vendor || '-'}</TableCell>
      <TableCell>
        {host.category ? (
          <Badge variant={CATEGORY_VARIANT[host.category] || 'outline'} className="text-[10px]">{host.category}</Badge>
        ) : '-'}
      </TableCell>
      <TableCell className="text-xs">
        {host.country ? <>{countryToFlag(host.country) || ''} {host.country}</> : '-'}
      </TableCell>
      <TableCell className="text-xs">{host.os_info || '-'}</TableCell>
      <TableCell className="text-xs">{host.asn || '-'}</TableCell>
      <TableCell className="text-xs">{host.node_id}</TableCell>
      <TableCell className="text-xs text-right">
        {formatBytes(host.bytes_sent)}
      </TableCell>
      <TableCell className="text-xs text-right">
        {formatBytes(host.bytes_received)}
      </TableCell>
      <TableCell className="text-xs text-right font-medium">
        {formatBytes(host.bytes_sent + host.bytes_received)}
      </TableCell>
      <TableCell className="text-xs text-right">{host.active_flows}</TableCell>
    </TableRow>
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold tracking-tight">{t.nav.hosts} ({totalHosts})</h1>
          {(countryFilter || asnFilter) && (
            <Badge variant="secondary" className="gap-1 pl-2 pr-1.5 h-7 text-xs">
              {countryFilter ? `${t.geo.country}: ${countryFilter}` : `ASN: ${asnFilter}`}
              <Button variant="ghost" size="icon" className="h-5 w-5 ml-0.5" onClick={clearFilters}>
                <X className="h-3 w-3" />
              </Button>
            </Badge>
          )}
        </div>
        <div className="flex gap-2">
          <div className="relative w-64">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder={t.hosts.searchPlaceholder || 'Search IP, hostname, MAC...'}
              value={search}
              onChange={(e) => { setSearch(e.target.value); setPage(0); }}
              className="pl-8"
            />
          </div>
          <Button variant="outline" size="sm" onClick={loadHosts} disabled={loading}>
            <RefreshCw className={`mr-1 h-3 w-3 ${loading ? 'animate-spin' : ''}`} />
          </Button>
          <Button variant="outline" size="sm" onClick={handleExport} disabled={!haveHosts}>
            <Download className="mr-1 h-3 w-3" /> CSV
          </Button>
        </div>
      </div>

      <Tabs value={hostsTab} onValueChange={(v) => { setHostsTab(v); setPage(0); }}>
        <TabsList>
          <TabsTrigger value="flat">{t.hosts.flat}</TabsTrigger>
          <TabsTrigger value="subnet">{t.hosts.bySubnet}</TabsTrigger>
        </TabsList>

        <TabsContent value="flat">
          <Card>
            <CardContent className="pt-6">
              {loading && hosts.length === 0 ? (
                <p className="text-center text-muted-foreground py-8 text-sm">{t.common.loading}</p>
              ) : (
                <>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t.dashboard.ip}</TableHead>
                        <TableHead>{t.hosts.mac}</TableHead>
                        <TableHead>{t.dashboard.hostname}</TableHead>
                        <TableHead>{t.hosts.vendor}</TableHead>
                        <TableHead>{t.hosts.category}</TableHead>
                        <TableHead>{t.hosts.country}</TableHead>
                        <TableHead>{t.hosts.os}</TableHead>
                        <TableHead>{t.hosts.asn}</TableHead>
                        <TableHead>{t.flows.node}</TableHead>
                        <TableHead className="text-right">{t.hosts.bytesSent}</TableHead>
                        <TableHead className="text-right">{t.hosts.bytesReceived}</TableHead>
                        <TableHead className="text-right">{t.dashboard.total}</TableHead>
                        <TableHead className="text-right">{t.hosts.activeFlows}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {haveHosts ? (
                        hosts.map(hostRow)
                      ) : (
                        <TableRow>
                          <TableCell colSpan={11} className="text-center text-muted-foreground py-8">
                            {t.hosts.noHosts || t.common.empty}
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                  {totalPages > 1 && (
                    <div className="flex items-center justify-between mt-4 pt-4 border-t border-border">
                      <span className="text-xs text-muted-foreground">
                        {t.hosts.page} {page + 1} {t.hosts.of} {totalPages} ({totalHosts} {t.hosts.hosts})
                      </span>
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
                          <ChevronLeft className="h-4 w-4" /> {t.hosts.prev}
                        </Button>
                        <Button variant="outline" size="sm" disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>
                          {t.hosts.next} <ChevronRight className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  )}
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="subnet">
          <Card>
            <CardContent className="pt-6">
              {loading && hosts.length === 0 ? (
                <p className="text-center text-muted-foreground py-8 text-sm">{t.common.loading}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-8" />
                      <TableHead>{t.hosts.subnet}</TableHead>
                      <TableHead className="text-right">{t.hosts.hosts}</TableHead>
                      <TableHead className="text-right">{t.dashboard.total}</TableHead>
                      <TableHead className="text-right">{t.dashboard.packets}</TableHead>
                      <TableHead className="text-right">{t.hosts.activeFlows}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {subnetGroups.length > 0 ? (
                      subnetGroups.map((g) => (
                        <>
                          <TableRow
                            key={g.subnet}
                            className="cursor-pointer hover:bg-muted/50"
                            onClick={() => setExpandedSubnet(expandedSubnet === g.subnet ? null : g.subnet)}
                          >
                            <TableCell>
                              {expandedSubnet === g.subnet ? (
                                <ChevronDown className="h-3 w-3" />
                              ) : (
                                <ChevronRight className="h-3 w-3" />
                              )}
                            </TableCell>
                            <TableCell className="font-mono text-xs font-medium">{g.subnet}</TableCell>
                            <TableCell className="text-xs text-right">{g.hosts.length}</TableCell>
                            <TableCell className="text-xs text-right font-medium">
                              {formatBytes(g.totalBytes)}
                            </TableCell>
                            <TableCell className="text-xs text-right">{g.totalPackets.toLocaleString()}</TableCell>
                            <TableCell className="text-xs text-right">{g.totalFlows}</TableCell>
                          </TableRow>
                          {expandedSubnet === g.subnet && g.hosts.map(hostRow)}
                        </>
                      ))
                    ) : (
                      <TableRow>
                        <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                          {t.hosts.noHosts || t.common.empty}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
