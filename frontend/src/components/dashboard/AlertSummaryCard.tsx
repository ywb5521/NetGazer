import { useMemo } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { AlertTriangle, AlertCircle, Info, ShieldCheck } from 'lucide-react';

const severityConfig: Record<string, { icon: typeof AlertTriangle; color: string }> = {
  critical: { icon: AlertTriangle, color: 'text-red-500' },
  warning: { icon: AlertCircle, color: 'text-yellow-500' },
  info: { icon: Info, color: 'text-blue-500' },
};

export function AlertSummaryCard() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();

  const { counts, unacked } = useMemo(() => {
    if (!snapshot) return { counts: { critical: 0, warning: 0, info: 0 }, unacked: 0 };

    const c: Record<string, number> = { critical: 0, warning: 0, info: 0 };
    let u = 0;
    for (const a of snapshot.alerts) {
      c[a.severity] = (c[a.severity] || 0) + 1;
      if (!a.acknowledged) u++;
    }
    return { counts: c, unacked: u };
  }, [snapshot]);

  const severityLabels: Record<string, string> = {
    critical: t.alerts.critical,
    warning: t.alerts.warning,
    info: t.alerts.info,
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.dashboard.alertSummary}</CardTitle>
      </CardHeader>
      <CardContent>
        {snapshot && unacked === 0 && counts.critical === 0 && counts.warning === 0 && counts.info === 0 ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <ShieldCheck className="h-4 w-4 text-emerald-500" />
            {t.dashboard.noAlerts}
          </div>
        ) : !snapshot ? (
          <p className="text-sm text-muted-foreground">{t.common.loading}</p>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground">{t.dashboard.unacked}</span>
              <Badge variant={unacked > 0 ? 'destructive' : 'outline'}>{unacked}</Badge>
            </div>
            <div className="space-y-1.5">
              {Object.entries(severityConfig).map(([severity, cfg]) => (
                <div key={severity} className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5">
                    <cfg.icon className={`h-3.5 w-3.5 ${cfg.color}`} />
                    <span className="text-xs">{severityLabels[severity]}</span>
                  </div>
                  <span className="text-xs font-medium tabular-nums">{counts[severity] || 0}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
