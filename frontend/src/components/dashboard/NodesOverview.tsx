import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { CircleCheck, CircleX } from 'lucide-react';

export function NodesOverview() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();

  if (!snapshot || !snapshot.nodes || snapshot.nodes.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">{t.nav.nodes}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{t.common.empty}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.nav.nodes}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {snapshot.nodes.map((node) => (
            <div
              key={node.node_id}
              className="flex items-start gap-3 rounded-lg border border-border p-3"
            >
              <div className="mt-0.5">
                {node.online ? (
                  <CircleCheck className="h-4 w-4 text-green-500" />
                ) : (
                  <CircleX className="h-4 w-4 text-red-500" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium truncate">{node.node_id}</span>
                  {node.tags?.map((tag) => (
                    <Badge key={tag} variant="secondary" className="text-[10px] px-1.5 py-0">
                      {tag}
                    </Badge>
                  ))}
                </div>
                <div className="mt-1 text-xs text-muted-foreground">
                  <span>{formatBytes(node.bytes_per_sec)}/s</span>
                  <span className="mx-2">·</span>
                  <span>{node.hosts_count} {t.nav.hosts}</span>
                  <span className="mx-2">·</span>
                  <span>{node.flows_count} {t.nav.flows}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
