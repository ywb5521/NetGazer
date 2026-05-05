import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useI18n } from '@/i18n/I18nContext';
import {
  listInterceptRules,
  createInterceptRule,
  updateInterceptRule,
  deleteInterceptRule,
  applyInterceptRules,
  getInterceptNodeRules,
  fetchNodes,
} from '@/lib/api';
import type { InterceptRule, NodeInfo } from '@/types';
import { Shield, Plus, Trash2, Send, ChevronDown, ChevronUp, X, BookOpen } from 'lucide-react';

const actionColors: Record<string, 'destructive' | 'secondary' | 'outline'> = {
  block: 'destructive',
  drop: 'secondary',
  allow: 'outline',
};

export default function InterceptPage() {
  const { t } = useI18n();
  const [rules, setRules] = useState<InterceptRule[]>([]);
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [form, setForm] = useState({ name: '', expression: '', action: 'block', enabled: true });
  const [saving, setSaving] = useState(false);
  const [deployOpen, setDeployOpen] = useState(false);
  const [selectedNodes, setSelectedNodes] = useState<Set<string>>(new Set());
  const [selectedRules, setSelectedRules] = useState<Set<string>>(new Set());
  const [deploying, setDeploying] = useState(false);
  const [deployResult, setDeployResult] = useState('');
  const [showNodeRules, setShowNodeRules] = useState(false);
  const [showReference, setShowReference] = useState(false);
  const [nodeRulesMap, setNodeRulesMap] = useState<Record<string, InterceptRule[]>>({});
  const [loadingNodeRules, setLoadingNodeRules] = useState<string | null>(null);

  const loadRules = useCallback(async () => {
    try {
      const res = await listInterceptRules();
      setRules(res.rules || []);
    } catch { /* ignore */ }
  }, []);

  const loadNodes = useCallback(async () => {
    try {
      const list = await fetchNodes();
      setNodes(list.filter((n) => n.online));
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { loadRules(); loadNodes(); }, [loadRules, loadNodes]);

  const resetForm = () => {
    setForm({ name: '', expression: '', action: 'block', enabled: true });
    setEditingId(null);
    setIsNew(false);
  };

  const handleEdit = (rule: InterceptRule) => {
    setForm({ name: rule.name, expression: rule.expression, action: rule.action, enabled: rule.enabled });
    setEditingId(rule.id);
    setIsNew(false);
  };

  const handleNew = () => {
    resetForm();
    setIsNew(true);
  };

  const handleSave = async () => {
    if (!form.name.trim() || !form.expression.trim()) return;
    setSaving(true);
    try {
      if (isNew) {
        await createInterceptRule({
          name: form.name.trim(),
          expression: form.expression.trim(),
          action: form.action,
          enabled: form.enabled,
        });
      } else if (editingId) {
        await updateInterceptRule(editingId, {
          name: form.name.trim(),
          expression: form.expression.trim(),
          action: form.action,
          enabled: form.enabled,
        });
      }
      await loadRules();
      resetForm();
    } catch { /* ignore */ }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm(t.intercept.deleteConfirm)) return;
    try {
      await deleteInterceptRule(id);
      await loadRules();
      if (editingId === id) resetForm();
    } catch { /* ignore */ }
  };

  const handleDeploy = async () => {
    setDeploying(true);
    setDeployResult('');
    try {
      const nodeIds = selectedNodes.size > 0 ? Array.from(selectedNodes) : [];
      const ruleIds = selectedRules.size > 0 ? Array.from(selectedRules) : [];
      const res = await applyInterceptRules(nodeIds, ruleIds);
      setDeployResult(t.intercept.deployed.replace('{count}', String(res.sent_to?.length || 0)));
    } catch (e: any) {
      setDeployResult(e.message || 'Deploy failed');
    }
    setDeploying(false);
  };

  const toggleNode = (nodeId: string) => {
    setSelectedNodes((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) next.delete(nodeId);
      else next.add(nodeId);
      return next;
    });
  };

  const toggleRule = (ruleId: string) => {
    setSelectedRules((prev) => {
      const next = new Set(prev);
      if (next.has(ruleId)) next.delete(ruleId);
      else next.add(ruleId);
      return next;
    });
  };

  const handleShowNodeRules = async () => {
    if (showNodeRules) { setShowNodeRules(false); return; }
    setShowNodeRules(true);
    const online = nodes.filter((n) => n.online);
    for (const node of online) {
      setLoadingNodeRules(node.node_id);
      try {
        const res = await getInterceptNodeRules(node.node_id);
        setNodeRulesMap((prev) => ({ ...prev, [node.node_id]: res.rules || [] }));
      } catch {
        setNodeRulesMap((prev) => ({ ...prev, [node.node_id]: [] }));
      }
      setLoadingNodeRules(null);
    }
  };

  const actionLabel = (a: string) => {
    if (a === 'block') return t.intercept.block;
    if (a === 'drop') return t.intercept.drop;
    if (a === 'allow') return t.intercept.allow;
    return a;
  };

  const editing = isNew || editingId !== null;
  const enabledRules = rules.filter((r) => r.enabled);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Shield className="h-6 w-6" />
            {t.intercept.title}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">{t.intercept.description}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => setShowReference(!showReference)}>
            <BookOpen className="h-4 w-4 mr-1" /> {t.intercept.reference}
          </Button>
          <Button variant="outline" size="sm" onClick={handleShowNodeRules}>
            {showNodeRules ? <ChevronUp className="h-4 w-4 mr-1" /> : <ChevronDown className="h-4 w-4 mr-1" />}
            {t.intercept.nodeRules}
          </Button>
          <Button variant="default" size="sm" onClick={() => setDeployOpen(true)}>
            <Send className="h-4 w-4 mr-1" /> {t.intercept.deploy}
          </Button>
        </div>
      </div>

      {/* Expression reference */}
      {showReference && (
        <Card>
          <CardHeader className="py-3 flex flex-row items-center justify-between">
            <CardTitle className="text-sm">{t.intercept.reference}</CardTitle>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowReference(false)}>
              <X className="h-4 w-4" />
            </Button>
          </CardHeader>
          <CardContent>
            <div className="text-sm leading-relaxed space-y-4">
              <div>
                <p className="font-medium mb-2">语法</p>
                <p className="text-muted-foreground">规则使用 <a href="https://expr-lang.org/docs/language-definition" target="_blank" rel="noopener noreferrer" className="underline text-primary">expr-lang/expr</a> 表达式语法，引用协议分析器提取的属性。每个分析器名称为根对象，包含 <code className="bg-muted px-1 rounded text-xs">req</code>（请求）和 <code className="bg-muted px-1 rounded text-xs">resp</code>（响应）两个子对象。</p>
              </div>

              <div>
                <p className="font-medium mb-2">协议分析器属性</p>
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b text-left text-muted-foreground">
                        <th className="py-1.5 pr-3 font-medium">属性</th>
                        <th className="py-1.5 pr-3 font-medium">类型</th>
                        <th className="py-1.5 font-medium">说明</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">tls.req.sni</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">TLS SNI 域名</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">tls.req.versions</td><td className="py-1.5 pr-3 text-muted-foreground">list</td><td className="py-1.5">客户端支持的 TLS 版本</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">tls.req.ciphers</td><td className="py-1.5 pr-3 text-muted-foreground">list</td><td className="py-1.5">客户端密码套件列表</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">tls.resp.version</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">服务端选择的 TLS 版本</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">http.req.method</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">HTTP 请求方法</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">http.req.host</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">HTTP Host 请求头</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">http.req.path</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">HTTP 请求路径</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">http.req.user_agent</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">User-Agent 请求头</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">ssh.req.version</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">SSH 协议版本</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">ssh.req.software</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">SSH 客户端软件名</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">socks.req.target_addr</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">SOCKS 代理目标地址</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">dns.req.questions</td><td className="py-1.5 pr-3 text-muted-foreground">list</td><td className="py-1.5">DNS 查询域名列表</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">dns.resp.answers</td><td className="py-1.5 pr-3 text-muted-foreground">list</td><td className="py-1.5">DNS 应答记录列表</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">quic.req.sni</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">QUIC 连接 SNI</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">wireguard.message_type</td><td className="py-1.5 pr-3 text-muted-foreground">int</td><td className="py-1.5">1=握手 2=数据 3=Cookie</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">openvpn.opcode</td><td className="py-1.5 pr-3 text-muted-foreground">int</td><td className="py-1.5">OpenVPN 操作码</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">trojan.yes</td><td className="py-1.5 pr-3 text-muted-foreground">bool</td><td className="py-1.5">Trojan 协议检测为真</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">fet.yes</td><td className="py-1.5 pr-3 text-muted-foreground">bool</td><td className="py-1.5">全加密流量 (疑似 Shadowsocks/VMess)</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">ip.src</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">源 IP 地址</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">ip.dst</td><td className="py-1.5 pr-3 text-muted-foreground">string</td><td className="py-1.5">目标 IP 地址</td></tr>
                      <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">tcp.src_port</td><td className="py-1.5 pr-3 text-muted-foreground">int</td><td className="py-1.5">TCP 源端口</td></tr>
                      <tr><td className="py-1.5 pr-3 font-mono">tcp.dst_port</td><td className="py-1.5 pr-3 text-muted-foreground">int</td><td className="py-1.5">TCP 目标端口</td></tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <div>
                <p className="font-medium mb-2">可用函数</p>
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="py-1.5 pr-3 font-medium">函数</th>
                      <th className="py-1.5 font-medium">说明</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">endsWith(suffix)</td><td className="py-1.5">字符串后缀匹配</td></tr>
                    <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">startsWith(prefix)</td><td className="py-1.5">字符串前缀匹配</td></tr>
                    <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">contains(substr)</td><td className="py-1.5">字符串包含</td></tr>
                    <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">in(list)</td><td className="py-1.5">元素在列表中</td></tr>
                    <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">cidrMatch(ip, cidr)</td><td className="py-1.5">IP 在 CIDR 范围内</td></tr>
                    <tr className="border-b border-border/50"><td className="py-1.5 pr-3 font-mono">geoip(ip, code)</td><td className="py-1.5">判断 IP 是否属于指定国家/地区 (ISO 3166-1 代码，如 CN/US)</td></tr>
                    <tr><td className="py-1.5 pr-3 font-mono">lookup(host, server)</td><td className="py-1.5">DNS 查询</td></tr>
                  </tbody>
                </table>
              </div>

              <div>
                <p className="font-medium mb-2">动作说明</p>
                <ul className="text-muted-foreground space-y-1">
                  <li><Badge variant="destructive" className="text-xs mr-1">block</Badge> 阻断 — 立即阻断整条 TCP 连接，发送 RST 给双方</li>
                  <li><Badge variant="secondary" className="text-xs mr-1">drop</Badge> 丢弃 — 静默丢弃当前数据包 (UDP)，不通知发送方</li>
                  <li><Badge variant="outline" className="text-xs mr-1">allow</Badge> 放行 — 明确放行连接，跳过后续规则匹配</li>
                </ul>
              </div>

              <div>
                <p className="font-medium mb-2">表达式示例</p>
                <div className="bg-muted rounded-md p-3 space-y-1.5">
                  <p className="font-mono text-xs"><span className="text-muted-foreground">// 阻断特定域名的 TLS 连接</span></p>
                  <code className="text-xs bg-background rounded px-2 py-0.5 block">tls.req.sni endsWith ".bad-site.com"</code>
                  <p className="font-mono text-xs mt-2"><span className="text-muted-foreground">// 阻断 QUIC 通向特定 IP</span></p>
                  <code className="text-xs bg-background rounded px-2 py-0.5 block">quic.req.sni == "target.example.com"</code>
                  <p className="font-mono text-xs mt-2"><span className="text-muted-foreground">// 阻断疑似 Shadowsocks 的高端口流量</span></p>
                  <code className="text-xs bg-background rounded px-2 py-0.5 block">fet.yes and tcp.dst_port &gt; 1024</code>
                  <p className="font-mono text-xs mt-2"><span className="text-muted-foreground">// 阻断到特定 IP 的 POST API 请求</span></p>
                  <code className="text-xs bg-background rounded px-2 py-0.5 block">http.req.method == "POST" and http.req.path contains "/api"</code>
                  <p className="font-mono text-xs mt-2"><span className="text-muted-foreground">// 阻断来自特定国家的源 IP</span></p>
                  <code className="text-xs bg-background rounded px-2 py-0.5 block">geoip(ip.src, "RU")</code>
                  <p className="font-mono text-xs mt-2"><span className="text-muted-foreground">// 阻断发往特定国家的目标 IP</span></p>
                  <code className="text-xs bg-background rounded px-2 py-0.5 block">geoip(ip.dst, "CN") and tcp.dst_port == 443</code>
                  <p className="font-mono text-xs mt-2"><span className="text-muted-foreground">// 阻断来自非授权国家的加密流量</span></p>
                  <code className="text-xs bg-background rounded px-2 py-0.5 block">fet.yes and not (geoip(ip.src, "US") or geoip(ip.src, "GB"))</code>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Node rules status */}
      {showNodeRules && (
        <Card>
          <CardHeader className="py-3">
            <CardTitle className="text-sm">{t.intercept.nodeRules}</CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            {nodes.filter((n) => n.online).length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.intercept.noNodeRules}</p>
            ) : (
              <div className="space-y-2">
                {nodes.filter((n) => n.online).map((node) => (
                  <div key={node.node_id} className="border rounded-md p-3">
                    <p className="text-sm font-medium mb-1">{node.node_id}</p>
                    {loadingNodeRules === node.node_id ? (
                      <p className="text-xs text-muted-foreground">{t.common.loading}</p>
                    ) : (nodeRulesMap[node.node_id] || []).length === 0 ? (
                      <p className="text-xs text-muted-foreground">{t.intercept.noRules}</p>
                    ) : (
                      <div className="flex flex-wrap gap-1">
                        {(nodeRulesMap[node.node_id] || []).map((r) => (
                          <Badge key={r.id} variant={r.enabled ? actionColors[r.action] || 'outline' : 'outline'} className="text-xs">
                            {r.name} ({actionLabel(r.action)})
                          </Badge>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Rules table */}
      <Card>
        <CardHeader className="py-3 flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Rules ({rules.length})</CardTitle>
          <Button variant="outline" size="sm" onClick={handleNew} disabled={editing}>
            <Plus className="h-3 w-3 mr-1" /> {t.intercept.newRule}
          </Button>
        </CardHeader>
        <CardContent className="pt-0">
          {rules.length === 0 && !editing ? (
            <p className="text-sm text-muted-foreground py-4 text-center">{t.intercept.noRules}</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="py-2 px-2 font-medium">{t.intercept.ruleName}</th>
                    <th className="py-2 px-2 font-medium">{t.intercept.expression}</th>
                    <th className="py-2 px-2 font-medium w-20">{t.intercept.action}</th>
                    <th className="py-2 px-2 font-medium w-16">{t.intercept.enabled}</th>
                    <th className="py-2 px-2 font-medium w-24"></th>
                  </tr>
                </thead>
                <tbody>
                  {rules.map((rule) => (
                    <tr key={rule.id} className={`border-b last:border-0 ${editingId === rule.id ? 'bg-accent/30' : ''}`}>
                      <td className="py-2 px-2 font-medium">{rule.name}</td>
                      <td className="py-2 px-2 font-mono text-xs max-w-xs truncate" title={rule.expression}>
                        {rule.expression}
                      </td>
                      <td className="py-2 px-2">
                        <Badge variant={actionColors[rule.action] || 'outline'} className="text-xs">
                          {actionLabel(rule.action)}
                        </Badge>
                      </td>
                      <td className="py-2 px-2">
                        <span className={`inline-block h-2 w-2 rounded-full ${rule.enabled ? 'bg-green-500' : 'bg-gray-400'}`} />
                      </td>
                      <td className="py-2 px-2">
                        <div className="flex items-center gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7"
                            onClick={() => handleEdit(rule)}
                            disabled={editing}
                            title={t.intercept.edit}
                          >
                            <Shield className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7 text-destructive hover:text-destructive"
                            onClick={() => handleDelete(rule.id)}
                            disabled={editing}
                            title={t.intercept.delete}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Inline edit/create form */}
          {editing && (
            <div className="mt-4 border rounded-md p-4 bg-muted/20 space-y-3">
              <div className="flex items-center gap-3">
                <Input
                  placeholder={t.intercept.ruleName}
                  value={form.name}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                  className="h-9 w-48"
                />
                <Input
                  placeholder={t.intercept.expressionPlaceholder}
                  value={form.expression}
                  onChange={(e) => setForm((f) => ({ ...f, expression: e.target.value }))}
                  className="h-9 flex-1 font-mono text-xs"
                />
                <Select value={form.action} onValueChange={(v) => setForm((f) => ({ ...f, action: v }))}>
                  <SelectTrigger className="h-9 w-28">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="block">{t.intercept.block}</SelectItem>
                    <SelectItem value="drop">{t.intercept.drop}</SelectItem>
                    <SelectItem value="allow">{t.intercept.allow}</SelectItem>
                  </SelectContent>
                </Select>
                <label className="flex items-center gap-2 text-sm cursor-pointer">
                  <Switch checked={form.enabled} onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))} />
                  <span className="text-xs text-muted-foreground">{t.intercept.enabled}</span>
                </label>
                <div className="flex items-center gap-1">
                  <Button size="sm" onClick={handleSave} disabled={saving || !form.name.trim() || !form.expression.trim()}>
                    {saving ? t.intercept.saving : t.intercept.save}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={resetForm}>
                    <X className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <p className="text-xs text-muted-foreground">
                {form.action === 'block' && t.intercept.actionBlockDesc}
                {form.action === 'drop' && t.intercept.actionDropDesc}
                {form.action === 'allow' && t.intercept.actionAllowDesc}
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Deploy modal */}
      {deployOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setDeployOpen(false)}>
          <div
            className="bg-background rounded-lg shadow-xl w-full max-w-lg mx-4 p-6 space-y-4 max-h-[80vh] overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">{t.intercept.deployTitle}</h3>
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setDeployOpen(false)}>
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* Node selection */}
            <div>
              <p className="text-sm font-medium mb-2">{t.intercept.selectNodes}</p>
              <div className="max-h-44 overflow-y-auto space-y-1 border rounded-md p-3">
                {nodes.filter((n) => n.online).length === 0 ? (
                  <p className="text-sm text-muted-foreground">{t.common.empty}</p>
                ) : (
                  nodes.filter((n) => n.online).map((node) => (
                    <label key={node.node_id} className="flex items-center gap-2 py-1 cursor-pointer text-sm">
                      <input
                        type="checkbox"
                        checked={selectedNodes.has(node.node_id)}
                        onChange={() => toggleNode(node.node_id)}
                        className="rounded"
                      />
                      <span>{node.node_id}</span>
                      {node.version && <span className="text-xs text-muted-foreground">v{node.version}</span>}
                    </label>
                  ))
                )}
              </div>
              {selectedNodes.size === 0 && (
                <p className="text-xs text-muted-foreground mt-1 italic">{t.intercept.allNodes}</p>
              )}
            </div>

            {/* Rule selection */}
            <div>
              <p className="text-sm font-medium mb-2">{t.intercept.selectRules}</p>
              <div className="max-h-44 overflow-y-auto space-y-1 border rounded-md p-3">
                {enabledRules.length === 0 ? (
                  <p className="text-sm text-muted-foreground">{t.intercept.noRules}</p>
                ) : (
                  enabledRules.map((rule) => (
                    <label key={rule.id} className="flex items-center gap-2 py-1 cursor-pointer text-sm">
                      <input
                        type="checkbox"
                        checked={selectedRules.has(rule.id)}
                        onChange={() => toggleRule(rule.id)}
                        className="rounded"
                      />
                      <span className="truncate">{rule.name}</span>
                      <Badge variant={actionColors[rule.action] || 'outline'} className="text-xs ml-auto shrink-0">
                        {actionLabel(rule.action)}
                      </Badge>
                    </label>
                  ))
                )}
              </div>
              {selectedRules.size === 0 && (
                <p className="text-xs text-muted-foreground mt-1 italic">{t.intercept.allRules}</p>
              )}
            </div>

            {deployResult && (
              <div className={`rounded-md p-2 text-sm ${deployResult.includes('fail') || deployResult.includes('Error') ? 'bg-red-50 text-red-700' : 'bg-green-50 text-green-700'}`}>
                {deployResult}
              </div>
            )}
            <div className="flex justify-end gap-2 pt-2 border-t">
              <Button variant="outline" size="sm" onClick={() => setDeployOpen(false)}>{t.common.close}</Button>
              <Button size="sm" onClick={handleDeploy} disabled={deploying}>
                <Send className="h-3 w-3 mr-1" /> {deploying ? t.intercept.deploying : t.intercept.deploy}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
