import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ChannelManager } from '@/components/settings/ChannelManager';
import { ThresholdEditor } from '@/components/settings/ThresholdEditor';
import { BpfFilterEditor } from '@/components/settings/BpfFilterEditor';
import { LuaEditor } from '@/components/settings/LuaEditor';
import { GeoIPEditor } from '@/components/settings/GeoIPEditor';
import { NodeTokenManager } from '@/components/settings/NodeTokenManager';
import { useI18n } from '@/i18n/I18nContext';
import { Settings } from 'lucide-react';

export default function SettingsPage() {
  const { t } = useI18n();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
        <Settings className="h-6 w-6" />
        {t.settings.title}
      </h1>

      <div className="grid grid-cols-1 gap-6 max-w-4xl">
        <ThresholdEditor />
        <BpfFilterEditor />
        <ChannelManager />
        <NodeTokenManager />
        <LuaEditor />
        <GeoIPEditor />

        <Card>
          <CardHeader>
            <CardTitle className="text-sm">{t.settings.systemInfo}</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.settings.version}</dt>
                <dd>netgazer 0.1.0</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.settings.frontend}</dt>
                <dd>React + TypeScript + shadcn/ui</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.settings.backend}</dt>
                <dd>Go + gRPC + WebSocket</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.settings.database}</dt>
                <dd>SQLite</dd>
              </div>
            </dl>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
