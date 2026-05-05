import { useMemo } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';

export function DnsStatsCard() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();

  const topQueries = useMemo(() => {
    if (!snapshot?.dns_queries) return [];
    return snapshot.dns_queries.slice(0, 10);
  }, [snapshot]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.dashboard.dnsQueries}</CardTitle>
      </CardHeader>
      <CardContent>
        {topQueries.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t.dashboard.noDns}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">{t.dashboard.domain}</TableHead>
                <TableHead className="text-xs text-right">{t.dashboard.count}</TableHead>
                <TableHead className="text-xs text-right">{t.dashboard.bytes}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {topQueries.map((q) => (
                <TableRow key={q.query_name}>
                  <TableCell className="font-mono text-xs truncate max-w-[160px]">
                    {q.query_name}
                  </TableCell>
                  <TableCell className="text-xs text-right">{q.count}</TableCell>
                  <TableCell className="text-xs text-right text-muted-foreground">
                    {formatBytes(q.bytes)}
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
