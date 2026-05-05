import { useEffect, useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { useI18n } from '@/i18n/I18nContext';
import { fetchNodeTokens, createNodeToken, deleteNodeToken, type NodeToken } from '@/lib/api';
import { Key, Plus, Trash2, Copy, Check } from 'lucide-react';

export function NodeTokenManager() {
  const { t } = useI18n();
  const [tokens, setTokens] = useState<NodeToken[]>([]);
  const [desc, setDesc] = useState('');
  const [newToken, setNewToken] = useState<{ id: string; token: string; warning: string } | null>(null);
  const [copied, setCopied] = useState(false);

  const load = async () => {
    try {
      const data = await fetchNodeTokens();
      setTokens(data);
    } catch { /* ignore */ }
  };

  useEffect(() => { load(); }, []);

  const handleGenerate = async () => {
    if (!desc.trim()) return;
    try {
      const result = await createNodeToken(desc.trim());
      setNewToken(result);
      setDesc('');
      load();
    } catch { /* ignore */ }
  };

  const handleCopy = () => {
    if (newToken) {
      navigator.clipboard.writeText(newToken.token);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleRevoke = async (id: string) => {
    try {
      await deleteNodeToken(id);
      load();
    } catch { /* ignore */ }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <Key className="h-4 w-4" />
          {t.settings.nodeTokens}
        </CardTitle>
        <CardDescription className="text-xs">{t.settings.nodeTokensDesc}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Generate form */}
        <div className="flex gap-2">
          <Input
            className="h-8 text-xs flex-1"
            placeholder={t.settings.tokenDescriptionPlaceholder}
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleGenerate()}
          />
          <Button size="sm" className="h-8 text-xs" onClick={handleGenerate} disabled={!desc.trim()}>
            <Plus className="h-3.5 w-3.5 mr-1" />
            {t.settings.generateToken}
          </Button>
        </div>

        {/* New token display (one-time) */}
        {newToken && (
          <div className="rounded-lg border border-border bg-muted/30 p-3 space-y-2">
            <div className="text-xs font-medium text-foreground">{t.settings.tokenCreated}</div>
            <div className="flex items-center gap-2">
              <code className="flex-1 text-xs bg-background rounded px-2 py-1.5 break-all font-mono border">
                {newToken.token}
              </code>
              <Button size="sm" variant="outline" className="h-7 text-xs shrink-0" onClick={handleCopy}>
                {copied ? <Check className="h-3 w-3 mr-1" /> : <Copy className="h-3 w-3 mr-1" />}
                {copied ? t.settings.tokenCopied : t.settings.copyToken}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">{t.settings.tokenCreatedWarning}</p>
          </div>
        )}

        {/* Token list */}
        {tokens.length === 0 ? (
          <p className="text-xs text-muted-foreground py-4 text-center">{t.settings.noTokens}</p>
        ) : (
          <div className="space-y-1 max-h-64 overflow-y-auto">
            {tokens.map((tok) => (
              <div
                key={tok.id}
                className="flex items-center gap-3 rounded-md border border-border/50 px-3 py-2 text-xs"
              >
                <code className="font-mono text-muted-foreground flex-1 min-w-0 truncate">{tok.token}</code>
                <span className="text-muted-foreground truncate max-w-[140px]">{tok.description || '—'}</span>
                <span className="text-muted-foreground whitespace-nowrap shrink-0">
                  {tok.last_used_at
                    ? new Date(tok.last_used_at).toLocaleDateString()
                    : '—'}
                </span>
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-6 w-6 p-0 shrink-0 text-muted-foreground hover:text-destructive"
                  onClick={() => handleRevoke(tok.id)}
                  title={t.settings.revokeToken}
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
