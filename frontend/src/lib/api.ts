const BASE = `${import.meta.env.BASE_URL}api`;

function getToken(): string | null {
  try {
    return localStorage.getItem('gtopng-token');
  } catch {
    return null;
  }
}

function authHeaders(): Record<string, string> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

async function authFetch(url: string, options?: RequestInit): Promise<Response> {
  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  if (options?.headers) {
    Object.assign(headers, options.headers as Record<string, string>);
  }
  const res = await fetch(url, { ...options, headers });
  if (res.status === 401) {
    try { localStorage.removeItem('gtopng-token'); } catch { /* ignore */ }
  }
  return res;
}

function handleAuthError(status: number) {
  if (status === 401) {
    try { localStorage.removeItem('gtopng-token'); } catch { /* ignore */ }
  }
}

export async function checkSetup(): Promise<{ setup_required: boolean }> {
  const res = await fetch(`${BASE}/auth/status`);
  if (!res.ok) throw new Error(`Setup check failed: ${res.status}`);
  return res.json();
}

export async function setup(username: string, password: string): Promise<void> {
  const res = await fetch(`${BASE}/auth/setup`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Setup failed' }));
    throw new Error(err.error || 'Setup failed');
  }
}

export async function login(username: string, password: string): Promise<{ token: string; username: string }> {
  const res = await authFetch(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Invalid credentials' }));
    throw new Error(err.error || 'Invalid credentials');
  }
  return res.json();
}

export async function fetchSummary(): Promise<import('@/types').Summary> {
  const res = await authFetch(`${BASE}/summary`);
  if (!res.ok) throw new Error(`Failed to fetch summary: ${res.status}`);
  return res.json();
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
}

export interface FetchHostsParams {
  nodeId?: string;
  iface?: string;
  search?: string;
  country?: string;
  asn?: string;
  sort?: string;
  limit?: number;
  offset?: number;
}

export async function fetchHosts(params?: FetchHostsParams): Promise<PaginatedResponse<import('@/types').Host>> {
  const q = new URLSearchParams();
  if (params?.nodeId) q.set('node_id', params.nodeId);
  if (params?.iface) q.set('interface', params.iface);
  if (params?.search) q.set('search', params.search);
  if (params?.country) q.set('country', params.country);
  if (params?.asn) q.set('asn', params.asn);
  if (params?.sort) q.set('sort', params.sort);
  q.set('limit', String(params?.limit ?? 50));
  q.set('offset', String(params?.offset ?? 0));
  const res = await authFetch(`${BASE}/hosts?${q}`);
  if (!res.ok) throw new Error(`Failed to fetch hosts: ${res.status}`);
  return res.json();
}

export interface FetchFlowsParams {
  nodeId?: string;
  iface?: string;
  search?: string;
  protocol?: string;
  app?: string;
  sort?: string;
  limit?: number;
  offset?: number;
}

export async function fetchFlows(params?: FetchFlowsParams): Promise<PaginatedResponse<import('@/types').Flow>> {
  const q = new URLSearchParams();
  if (params?.nodeId) q.set('node_id', params.nodeId);
  if (params?.iface) q.set('interface', params.iface);
  if (params?.search) q.set('search', params.search);
  if (params?.protocol) q.set('protocol', params.protocol);
  if (params?.app) q.set('app', params.app);
  if (params?.sort) q.set('sort', params.sort);
  q.set('limit', String(params?.limit ?? 50));
  q.set('offset', String(params?.offset ?? 0));
  const res = await authFetch(`${BASE}/flows?${q}`);
  if (!res.ok) throw new Error(`Failed to fetch flows: ${res.status}`);
  return res.json();
}

export async function fetchProtocols(nodeId?: string, iface?: string, limit?: number, offset?: number): Promise<PaginatedResponse<import('@/types').ProtocolStat>> {
  const q = new URLSearchParams();
  if (nodeId) q.set('node_id', nodeId);
  if (iface) q.set('interface', iface);
  q.set('limit', String(limit ?? 50));
  q.set('offset', String(offset ?? 0));
  const res = await authFetch(`${BASE}/protocols?${q}`);
  if (!res.ok) throw new Error(`Failed to fetch protocols: ${res.status}`);
  return res.json();
}

export async function fetchAlerts(nodeId?: string, limit = 100): Promise<import('@/types').Alert[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (nodeId) params.set('node_id', nodeId);
  const res = await authFetch(`${BASE}/alerts?${params}`);
  if (!res.ok) throw new Error(`Failed to fetch alerts: ${res.status}`);
  return res.json();
}

export async function acknowledgeAlert(id: string): Promise<void> {
  const res = await authFetch(`${BASE}/alerts/${encodeURIComponent(id)}/ack`, { method: 'POST' });
  if (!res.ok) throw new Error(`Failed to ack alert: ${res.status}`);
}

export async function fetchTrafficHistory(nodeId?: string, from?: string, to?: string, granularity?: string): Promise<import('@/types').TrafficSnapshot[]> {
  const params = new URLSearchParams({ interval: '10' });
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (granularity && granularity !== 'raw') params.set('granularity', granularity);
  const res = await authFetch(`${BASE}/traffic/history?${params}`);
  if (!res.ok) throw new Error(`Failed to fetch history: ${res.status}`);
  return res.json();
}

export async function fetchTrafficMatrix(nodeId?: string, limit = 20): Promise<import('@/types').TrafficMatrixCell[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (nodeId) params.set('node_id', nodeId);
  const res = await authFetch(`${BASE}/traffic-matrix?${params}`);
  if (!res.ok) throw new Error(`Failed to fetch traffic matrix: ${res.status}`);
  return res.json();
}

export async function fetchHostTrafficHistory(ip: string, nodeId?: string, from?: string, to?: string, granularity?: string): Promise<import('@/types').HostSnapshot[]> {
  const params = new URLSearchParams();
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (granularity && granularity !== 'raw') params.set('granularity', granularity);
  const res = await authFetch(`${BASE}/hosts/${encodeURIComponent(ip)}/traffic?${params}`);
  if (!res.ok) throw new Error(`Failed to fetch host traffic: ${res.status}`);
  return res.json();
}

export async function fetchNodes(): Promise<import('@/types').NodeInfo[]> {
  const res = await authFetch(`${BASE}/nodes`);
  if (!res.ok) throw new Error(`Failed to fetch nodes: ${res.status}`);
  return res.json();
}

export async function fetchHostProtocols(ip: string, nodeId?: string): Promise<import('@/types').ProtocolStat[]> {
  const url = nodeId ? `${BASE}/hosts/${encodeURIComponent(ip)}/protocols?node_id=${encodeURIComponent(nodeId)}` : `${BASE}/hosts/${encodeURIComponent(ip)}/protocols`;
  const res = await authFetch(url);
  if (!res.ok) throw new Error(`Failed to fetch host protocols: ${res.status}`);
  return res.json();
}

export interface HostPeer {
  peer_ip: string;
  bytes: number;
  packets: number;
  flow_count: number;
}

export async function fetchHostPeers(ip: string, nodeId?: string): Promise<HostPeer[]> {
  const url = nodeId ? `${BASE}/hosts/${encodeURIComponent(ip)}/peers?node_id=${encodeURIComponent(nodeId)}` : `${BASE}/hosts/${encodeURIComponent(ip)}/peers`;
  const res = await authFetch(url);
  if (!res.ok) throw new Error(`Failed to fetch host peers: ${res.status}`);
  return res.json();
}

export interface ServerConfig {
  bandwidth_threshold_bps: number;
  alert_thresholds?: import('@/types').AlertThresholds;
  bpf_filter?: string;
}

export async function fetchConfig(): Promise<ServerConfig> {
  const res = await authFetch(`${BASE}/config`);
  if (!res.ok) throw new Error(`Failed to fetch config: ${res.status}`);
  return res.json();
}

export async function updateConfig(update: Partial<ServerConfig>): Promise<void> {
  const res = await authFetch(`${BASE}/config`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(update),
  });
  if (!res.ok) throw new Error(`Failed to update config: ${res.status}`);
}

export async function testWebhook(): Promise<void> {
  const res = await authFetch(`${BASE}/webhook/test`, { method: 'POST' });
  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(data.error || `Webhook test failed: ${res.status}`);
  }
}

// Notification channels
export async function fetchNotificationChannels(): Promise<import('@/types').NotificationChannel[]> {
  const res = await authFetch(`${BASE}/notification-channels`);
  if (!res.ok) throw new Error(`Failed to fetch channels: ${res.status}`);
  return res.json();
}

export async function createNotificationChannel(ch: Omit<import('@/types').NotificationChannel, 'id'>): Promise<import('@/types').NotificationChannel> {
  const res = await authFetch(`${BASE}/notification-channels`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(ch),
  });
  if (!res.ok) throw new Error(`Failed to create channel: ${res.status}`);
  return res.json();
}

export async function updateNotificationChannel(id: string, ch: Partial<import('@/types').NotificationChannel>): Promise<void> {
  const res = await authFetch(`${BASE}/notification-channels/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(ch),
  });
  if (!res.ok) throw new Error(`Failed to update channel: ${res.status}`);
}

export async function deleteNotificationChannel(id: string): Promise<void> {
  const res = await authFetch(`${BASE}/notification-channels/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Failed to delete channel: ${res.status}`);
}

export async function testNotificationChannel(id: string): Promise<void> {
  const res = await authFetch(`${BASE}/notification-channels/${encodeURIComponent(id)}/test`, { method: 'POST' });
  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(data.error || `Test failed: ${res.status}`);
  }
}

export function exportCSV(headers: string[], rows: string[][], filename: string): void {
  const csvContent = [headers, ...rows]
    .map((row) => row.map((cell) => `"${String(cell).replace(/"/g, '""')}"`).join(','))
    .join('\n');
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

// Syslog API
export async function fetchSyslog(severity?: string, source?: string, limit = 100, offset = 0): Promise<PaginatedResponse<import('@/types').SyslogRecord>> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (severity) params.set('severity', severity);
  if (source) params.set('source', source);
  const res = await authFetch(`${BASE}/syslog?${params}`);
  if (!res.ok) throw new Error(`Failed to fetch syslog: ${res.status}`);
  return res.json();
}

// Trap API
export async function fetchTraps(limit = 100): Promise<{ items: import('@/types').TrapMessage[]; total: number }> {
  const res = await authFetch(`${BASE}/traps?limit=${limit}`);
  if (!res.ok) throw new Error(`Failed to fetch traps: ${res.status}`);
  return res.json();
}

// VoIP API
export async function fetchVoipSessions(): Promise<{ items: import('@/types').VOIPSession[]; total: number }> {
  const res = await authFetch(`${BASE}/voip-sessions`);
  if (!res.ok) throw new Error(`Failed to fetch VoIP sessions: ${res.status}`);
  return res.json();
}

// Matrix history API
export async function fetchMatrixHistory(from?: string, to?: string): Promise<import('@/types').TrafficMatrixHistoryCell[]> {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await authFetch(`${BASE}/traffic-matrix/history?${params}`);
  if (!res.ok) throw new Error(`Failed to fetch matrix history: ${res.status}`);
  return res.json();
}

// Lua script APIs
export async function fetchLuaScripts(): Promise<{ items: import('@/types').LuaScript[]; total: number }> {
  const res = await authFetch(`${BASE}/lua-scripts`);
  if (!res.ok) throw new Error(`Failed to fetch Lua scripts: ${res.status}`);
  return res.json();
}

export async function createLuaScript(script: import('@/types').LuaScript): Promise<void> {
  const res = await authFetch(`${BASE}/lua-scripts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(script),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `Failed to save script: ${res.status}` }));
    throw new Error(err.error || 'Failed to save script');
  }
}

export async function deleteLuaScript(name: string): Promise<void> {
  const res = await authFetch(`${BASE}/lua-scripts/${encodeURIComponent(name)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Failed to delete script: ${res.status}`);
}

export async function testLuaScript(content: string, nodeId?: string): Promise<{ status: string; error?: string }> {
  const res = await authFetch(`${BASE}/lua-scripts/test`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, node_id: nodeId || '' }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `Test failed: ${res.status}` }));
    throw new Error(err.error || 'Test failed');
  }
  return res.json();
}

// Report APIs
export async function fetchReportSummary(nodeId?: string, from?: string, to?: string): Promise<import('@/types').SummaryReport> {
  const params = new URLSearchParams();
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await authFetch(`${BASE}/reports/summary?${params}`);
  if (!res.ok) throw new Error(`Report failed: ${res.status}`);
  return res.json();
}

export async function fetchReportTopTalkers(nodeId?: string, from?: string, to?: string, limit = 20): Promise<import('@/types').TopTalker[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await authFetch(`${BASE}/reports/top-talkers?${params}`);
  if (!res.ok) throw new Error(`Report failed: ${res.status}`);
  return res.json();
}

export async function fetchReportTopProtocols(nodeId?: string, from?: string, to?: string, limit = 10): Promise<import('@/types').TopProtocol[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await authFetch(`${BASE}/reports/top-protocols?${params}`);
  if (!res.ok) throw new Error(`Report failed: ${res.status}`);
  return res.json();
}

export async function fetchReportAlerts(nodeId?: string, from?: string, to?: string): Promise<import('@/types').AlertSummary> {
  const params = new URLSearchParams();
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await authFetch(`${BASE}/reports/alerts?${params}`);
  if (!res.ok) throw new Error(`Report failed: ${res.status}`);
  return res.json();
}

export async function fetchReportTrend(nodeId?: string, from?: string, to?: string): Promise<import('@/types').TrendPoint[]> {
  const params = new URLSearchParams();
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await authFetch(`${BASE}/reports/trend?${params}`);
  if (!res.ok) throw new Error(`Report failed: ${res.status}`);
  return res.json();
}

// Export APIs — trigger download
export function getExportUrl(type: 'snapshots' | 'hosts' | 'alerts', format: string, nodeId?: string, from?: string, to?: string): string {
  const params = new URLSearchParams({ format });
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const token = getToken();
  const sep = `${BASE}/export/${type}?${params}`;
  // We cannot add auth headers for direct download links, so we construct a URL and use a programmatic fetch
  return sep;
}

export async function downloadExport(type: 'snapshots' | 'hosts' | 'alerts', format: string, nodeId?: string, from?: string, to?: string): Promise<void> {
  const params = new URLSearchParams({ format });
  if (nodeId) params.set('node_id', nodeId);
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const url = `${BASE}/export/${type}?${params}`;
  const res = await authFetch(url);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `Export failed: ${res.status}` }));
    throw new Error(err.error || 'Export failed');
  }
  const blob = await res.blob();
  const ext = format === 'csv' ? 'csv' : format === 'ndjson' ? 'ndjson' : format === 'clickhouse' ? 'tsv' : 'json';
  const downloadUrl = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = downloadUrl;
  a.download = `gtopng-${type}.${ext}`;
  a.click();
  URL.revokeObjectURL(downloadUrl);
}

// ── Intercept Rules ──

export async function listInterceptRules(): Promise<{ rules: import('../types').InterceptRule[] }> {
  const res = await authFetch(`${BASE}/intercept/rules`);
  return res.json();
}

export async function createInterceptRule(rule: Partial<import('../types').InterceptRule>): Promise<{ status: string; id: string }> {
  const res = await authFetch(`${BASE}/intercept/rules`, {
    method: 'POST',
    body: JSON.stringify(rule),
  });
  return res.json();
}

export async function updateInterceptRule(id: string, rule: Partial<import('../types').InterceptRule>): Promise<{ status: string }> {
  const res = await authFetch(`${BASE}/intercept/rules/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(rule),
  });
  return res.json();
}

export async function deleteInterceptRule(id: string): Promise<{ status: string }> {
  const res = await authFetch(`${BASE}/intercept/rules/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
  return res.json();
}

export async function applyInterceptRules(nodeIds: string[], ruleIds?: string[]): Promise<{ status: string; sent_to: string[]; rules_count: number }> {
  const res = await authFetch(`${BASE}/intercept/apply`, {
    method: 'POST',
    body: JSON.stringify({ node_ids: nodeIds, rule_ids: ruleIds || [] }),
  });
  return res.json();
}

export async function getInterceptNodeRules(nodeId: string): Promise<{ node: string; rules: import('../types').InterceptRule[] }> {
  const res = await authFetch(`${BASE}/intercept/node-rules/${encodeURIComponent(nodeId)}`);
  return res.json();
}

// ── GeoIP ──

export interface GeoIPStatus {
  country_db: string;
  country_info: string;
  asn_db: string;
  asn_info: string;
  ready: boolean;
}

export async function fetchGeoIPStatus(): Promise<GeoIPStatus> {
  const res = await authFetch(`${BASE}/geoip/status`);
  return res.json();
}

export async function uploadGeoIPDB(file: File, type: 'country' | 'asn'): Promise<{ status: string; path: string; type: string; size: number }> {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('type', type);
  const res = await authFetch(`${BASE}/geoip/upload`, {
    method: 'POST',
    body: formData,
  });
  return res.json();
}

export async function downloadGeoIPDB(url: string, type: 'country' | 'asn'): Promise<{ status: string; path: string; type: string; size: number }> {
  const res = await authFetch(`${BASE}/geoip/download`, {
    method: 'POST',
    body: JSON.stringify({ url, type }),
  });
  return res.json();
}

export interface NodeToken {
  id: string;
  token: string;
  description: string;
  enabled: boolean;
  created_at: number;
  last_used_at: number | null;
}

export async function fetchNodeTokens(): Promise<NodeToken[]> {
  const res = await authFetch(`${BASE}/node-tokens`);
  return res.json();
}

export async function createNodeToken(description: string): Promise<{ id: string; token: string; description: string; warning: string }> {
  const res = await authFetch(`${BASE}/node-tokens`, {
    method: 'POST',
    body: JSON.stringify({ description }),
  });
  return res.json();
}

export async function deleteNodeToken(id: string): Promise<{ status: string }> {
  const res = await authFetch(`${BASE}/node-tokens/${id}`, { method: 'DELETE' });
  return res.json();
}

// ── Geo / Country / ASN ──

export interface CountryStat {
  country: string;
  iso_code: string;
  bytes: number;
  packets: number;
  hosts: number;
  percentage: number;
}

export async function fetchCountryStats(): Promise<CountryStat[]> {
  const res = await authFetch(`${BASE}/geo/countries`);
  return res.json();
}

export interface ASNStat {
  asn: string;
  as_number: number;
  org: string;
  bytes: number;
  packets: number;
  hosts: number;
  percentage: number;
}

export async function fetchASNStats(): Promise<ASNStat[]> {
  const res = await authFetch(`${BASE}/geo/asns`);
  return res.json();
}

// ── Service Map ──

export interface ServiceNode {
  name: string;
  bytes: number;
  hosts: number;
}

export interface ServiceEdge {
  src: string;
  dst: string;
  bytes: number;
}

export interface ServiceMapData {
  services: ServiceNode[];
  edges: ServiceEdge[];
}

export async function fetchServiceMap(): Promise<ServiceMapData> {
  const res = await authFetch(`${BASE}/service-map`);
  return res.json();
}

// ── Interfaces ──

export interface InterfaceSummary {
  node_id: string;
  name: string;
  bytes_per_sec: number;
  packets_per_sec: number;
  hosts_count: number;
  flows_count: number;
}

export async function fetchAllInterfaces(): Promise<InterfaceSummary[]> {
  const res = await authFetch(`${BASE}/interfaces`);
  return res.json();
}

// ── Host Pools ──

export interface HostPool {
  id: string;
  name: string;
  description: string;
  cidrs: string[];
  created_at: number;
}

export async function fetchHostPools(): Promise<HostPool[]> {
  const res = await authFetch(`${BASE}/host-pools`);
  return res.json();
}

export async function createHostPool(data: { name: string; description: string; cidrs: string[] }): Promise<{ id: string; status: string }> {
  const res = await authFetch(`${BASE}/host-pools`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function updateHostPool(id: string, data: { name: string; description: string; cidrs: string[] }): Promise<{ status: string }> {
  const res = await authFetch(`${BASE}/host-pools/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function deleteHostPool(id: string): Promise<{ status: string }> {
  const res = await authFetch(`${BASE}/host-pools/${encodeURIComponent(id)}`, { method: 'DELETE' });
  return res.json();
}

export interface HostPoolStats {
  pool: HostPool;
  hosts: { ip: string; hostname: string; bytes_in: number; bytes_out: number; country: string }[];
  hosts_count: number;
  total_bytes: number;
}

export async function fetchHostPoolStats(id: string): Promise<HostPoolStats> {
  const res = await authFetch(`${BASE}/host-pools/${encodeURIComponent(id)}/stats`);
  return res.json();
}
