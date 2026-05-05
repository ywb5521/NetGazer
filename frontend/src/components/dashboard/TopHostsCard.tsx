import { useRef, useMemo } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import type { GlobalSnapshot, Host } from '@/types';

interface HostRate {
  ip: string;
  hostname: string;
  vendor: string;
  bytes_per_sec: number;
  total_bytes: number;
  packets: number;
}

export function TopHostsCard() {
  const { snapshot, prevSnapshot } = useAppContext();
  const { t } = useI18n();

  const topHosts = useMemo(() => {
    if (!snapshot) return [];
    if (!prevSnapshot) return [];

    const elapsed = 2; // our throttle interval
    const prevMap = new Map<string, Host>();
    for (const h of prevSnapshot.hosts) {
      prevMap.set(h.ip + '|' + h.node_id, h);
    }

    const rates: HostRate[] = [];
    for (const h of snapshot.hosts) {
      const key = h.ip + '|' + h.node_id;
      const ph = prevMap.get(key);
      const prevTotal = ph ? ph.bytes_sent + ph.bytes_received : 0;
      const currTotal = h.bytes_sent + h.bytes_received;
      const delta = currTotal - prevTotal;
      rates.push({
        ip: h.ip,
        hostname: h.hostname,
        vendor: h.vendor,
        bytes_per_sec: delta / elapsed,
        total_bytes: currTotal,
        packets: h.packets_sent + h.packets_received,
      });
    }

    return rates
      .filter((r) => r.bytes_per_sec > 0)
      .sort((a, b) => b.bytes_per_sec - a.bytes_per_sec)
      .slice(0, 5);
  }, [snapshot, prevSnapshot]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.dashboard.topHosts}</CardTitle>
      </CardHeader>
      <CardContent>
        {topHosts.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t.dashboard.accumulating}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">{t.dashboard.ip}</TableHead>
                <TableHead className="text-xs">{t.dashboard.hostname}</TableHead>
                <TableHead className="text-xs text-right">{t.dashboard.rate}</TableHead>
                <TableHead className="text-xs text-right">{t.dashboard.total}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {topHosts.map((host) => (
                <TableRow key={host.ip}>
                  <TableCell className="font-mono text-xs">{host.ip}</TableCell>
                  <TableCell className="text-xs">{host.hostname || host.vendor || '-'}</TableCell>
                  <TableCell className="text-xs text-right font-medium text-emerald-500">
                    {formatBytes(host.bytes_per_sec)}/s
                  </TableCell>
                  <TableCell className="text-xs text-right text-muted-foreground">
                    {formatBytes(host.total_bytes)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
