import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import {
  fetchNotificationChannels, createNotificationChannel,
  updateNotificationChannel, deleteNotificationChannel, testNotificationChannel,
} from '@/lib/api';
import { useI18n } from '@/i18n/I18nContext';
import type { NotificationChannel, NotificationChannelType } from '@/types';
import { Plus, Trash2, Send, Bell, Globe, MessageCircle, Zap, Mail } from 'lucide-react';

const CHANNEL_ICONS: Record<NotificationChannelType, typeof Bell> = {
  generic_webhook: Globe,
  slack: MessageCircle,
  dingtalk: Zap,
  feishu: Bell,
  email: Mail,
  telegram: Send,
};

const CHANNEL_LABELS: Record<NotificationChannelType, string> = {
  generic_webhook: 'Webhook',
  slack: 'Slack',
  dingtalk: 'DingTalk',
  feishu: 'Feishu',
  email: 'Email',
  telegram: 'Telegram',
};

export function ChannelManager() {
  const { t } = useI18n();
  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, 'success' | 'fail'>>({});

  // New channel form
  const [newName, setNewName] = useState('');
  const [newType, setNewType] = useState<NotificationChannelType>('generic_webhook');
  const [newConfig, setNewConfig] = useState('{"url":""}');

  const loadChannels = useCallback(async () => {
    setLoading(true);
    try {
      setChannels(await fetchNotificationChannels());
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { loadChannels(); }, [loadChannels]);

  const handleToggle = async (ch: NotificationChannel) => {
    await updateNotificationChannel(ch.id, { enabled: !ch.enabled });
    loadChannels();
  };

  const handleDelete = async (id: string) => {
    await deleteNotificationChannel(id);
    loadChannels();
  };

  const handleTest = async (id: string) => {
    setTesting(id);
    try {
      await testNotificationChannel(id);
      setTestResults((prev) => ({ ...prev, [id]: 'success' }));
    } catch {
      setTestResults((prev) => ({ ...prev, [id]: 'fail' }));
    }
    setTesting(null);
    setTimeout(() => {
      setTestResults((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
    }, 5000);
  };

  const handleCreate = async () => {
    let cfg;
    try { cfg = JSON.parse(newConfig); } catch { cfg = { url: '' }; }
    await createNotificationChannel({
      name: newName || CHANNEL_LABELS[newType],
      type: newType,
      enabled: true,
      config: cfg,
    });
    setShowForm(false);
    setNewName('');
    setNewType('generic_webhook');
    setNewConfig('{"url":""}');
    loadChannels();
  };

  const getTypeDefaults = (type: NotificationChannelType) => {
    switch (type) {
      case 'email': return JSON.stringify({ smtp_server: '', smtp_port: 587, username: '', password: '', from: '', to: [''] }, null, 2);
      case 'telegram': return JSON.stringify({ bot_token: '', chat_id: '' }, null, 2);
      default: return '{"url":""}';
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">{t.settings.notificationChannels}</CardTitle>
        <CardDescription className="text-xs">
          {t.settings.notificationChannelsDesc}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {loading ? (
          <p className="text-xs text-muted-foreground">{t.common.loading}</p>
        ) : (
          <>
            {channels.map((ch) => {
              const Icon = CHANNEL_ICONS[ch.type] || Globe;
              const tr = testResults[ch.id];
              return (
                <div key={ch.id} className="flex items-center gap-3 rounded-lg border border-border p-3">
                  <Icon className="h-4 w-4 text-muted-foreground shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-xs font-medium">{ch.name}</p>
                    <Badge variant="outline" className="text-[10px]">{CHANNEL_LABELS[ch.type]}</Badge>
                  </div>
                  <Switch checked={ch.enabled} onCheckedChange={() => handleToggle(ch)} />
                  <Button
                    variant="outline" size="sm" className="h-7 text-xs"
                    onClick={() => handleTest(ch.id)}
                    disabled={testing === ch.id || !ch.enabled}
                  >
                    <Send className="mr-1 h-3 w-3" />
                    {testing === ch.id ? t.settings.testing : tr === 'success' ? t.settings.ok : tr === 'fail' ? t.settings.fail : t.settings.test}
                  </Button>
                  <Button variant="ghost" size="sm" className="h-7" onClick={() => handleDelete(ch.id)}>
                    <Trash2 className="h-3 w-3 text-muted-foreground" />
                  </Button>
                </div>
              );
            })}
            {channels.length === 0 && (
              <p className="text-xs text-muted-foreground text-center py-4">{t.settings.noChannels}</p>
            )}
          </>
        )}

        {showForm ? (
          <div className="border-t border-border pt-3 space-y-2">
            <Input
              placeholder={t.settings.channelName}
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              className="h-8 text-xs"
            />
            <Select value={newType} onValueChange={(v) => { setNewType(v as NotificationChannelType); setNewConfig(getTypeDefaults(v as NotificationChannelType)); }}>
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(CHANNEL_LABELS).map(([k, v]) => (
                  <SelectItem key={k} value={k}>{v}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <textarea
              value={newConfig}
              onChange={(e) => setNewConfig(e.target.value)}
              className="w-full h-20 rounded-md border border-input bg-background px-3 py-2 text-xs font-mono"
              spellCheck={false}
            />
            <div className="flex gap-2">
              <Button size="sm" className="h-7 text-xs" onClick={handleCreate}>
                <Plus className="mr-1 h-3 w-3" /> {t.settings.add}
              </Button>
              <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => setShowForm(false)}>
                {t.settings.cancel}
              </Button>
            </div>
          </div>
        ) : (
          <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setShowForm(true)}>
            <Plus className="mr-1 h-3 w-3" /> {t.settings.addChannel}
          </Button>
        )}
      </CardContent>
    </Card>
  );
}
