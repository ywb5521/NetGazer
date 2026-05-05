import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { Search } from 'lucide-react';
import { useMemo, useState } from 'react';

export default function DnsPage() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();
  const [search, setSearch] = useState('');

  const queries = useMemo(() => {
    if (!snapshot?.dns_queries) return [];
    let list = [...snapshot.dns_queries];
    if (search) {
      const q = search.toLowerCase();
      list = list.filter((d) => d.query_name.toLowerCase().includes(q));
    }
    const total = list.reduce((s, d) => s + d.count, 0);
    return list.map((d) => ({ ...d, pct: total > 0 ? (d.count / total) * 100 : 0 }));
  }, [snapshot, search]);

  const maxCount = queries.length > 0 ? Math.max(...queries.map((q) => q.count)) : 1;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight">{t.nav.dns}</h1>

      {!snapshot || snapshot.dns_queries?.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Search className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">{t.dashboard.noDns}</p>
            <p className="text-xs text-muted-foreground mt-1">
              DNS queries are tracked from captured traffic. Ensure agent is running.
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="flex items-center gap-4">
            <input
              type="text"
              placeholder="Filter domain..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="flex h-9 w-full max-w-sm rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            />
            <span className="text-xs text-muted-foreground">{queries.length} domains</span>
          </div>

          <div className="grid grid-cols-1 gap-4">
            {queries.map((q) => (
              <Card key={q.query_name} className="overflow-hidden">
                <CardContent className="p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-sm font-mono font-medium truncate" title={q.query_name}>
                        {q.query_name}
                      </span>
                    </div>
                    <span className="text-xs text-muted-foreground shrink-0 ml-4">
                      {q.pct.toFixed(1)}% · {q.count.toLocaleString()} queries
                    </span>
                  </div>
                  <div className="h-2 bg-muted rounded-full overflow-hidden">
                    <div
                      className="h-full bg-primary rounded-full transition-all"
                      style={{ width: `${Math.max((q.count / maxCount) * 100, 0.5)}%` }}
                    />
                  </div>
                  <div className="mt-2 flex items-center gap-4 text-[10px] text-muted-foreground">
                    <span>{formatBytes(q.bytes)} total</span>
                    <span>{q.count.toLocaleString()} hits</span>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
