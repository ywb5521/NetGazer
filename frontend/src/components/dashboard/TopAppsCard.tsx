import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { baseProtocol } from '@/lib/categories';
import { useMemo } from 'react';

const APP_COLORS = [
  'bg-chart-1', 'bg-chart-2', 'bg-chart-3', 'bg-chart-4', 'bg-chart-5',
];

export function TopAppsCard() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();

  const apps = useMemo(() => {
    if (!snapshot?.flows) return [];
    const appMap: Record<string, number> = {};
    for (const f of snapshot.flows) {
      const base = baseProtocol(f.app_protocol) || f.protocol;
      appMap[base] = (appMap[base] || 0) + f.bytes;
    }
    const entries = Object.entries(appMap).sort((a, b) => b[1] - a[1]);
    const total = entries.reduce((s, [, v]) => s + v, 0);
    return entries.slice(0, 5).map(([name, bytes]) => ({
      name,
      bytes,
      percentage: total > 0 ? (bytes / total) * 100 : 0,
    }));
  }, [snapshot]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.dashboard.topApps}</CardTitle>
      </CardHeader>
      <CardContent>
        {apps.length > 0 ? (
          <div className="space-y-3">
            {apps.map((app, i) => (
              <div key={app.name} className="space-y-1">
                <div className="flex items-center justify-between text-xs">
                  <span className="font-medium">{app.name}</span>
                  <span className="text-muted-foreground">
                    {formatBytes(app.bytes)} ({app.percentage.toFixed(1)}%)
                  </span>
                </div>
                <div className="h-1.5 w-full rounded-full bg-muted overflow-hidden">
                  <div
                    className={`h-full rounded-full ${APP_COLORS[i] || 'bg-muted-foreground'}`}
                    style={{ width: `${Math.max(app.percentage, 2)}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground py-4 text-center">{t.common.empty}</p>
        )}
      </CardContent>
    </Card>
  );
}
