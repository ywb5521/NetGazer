export interface Host {
  ip: string;
  mac: string;
  hostname: string;
  bytes_sent: number;
  bytes_received: number;
  packets_sent: number;
  packets_received: number;
  first_seen: number;
  last_seen: number;
  vendor: string;
  active_flows: number;
  node_id: string;
  interface?: string;
  country?: string;
  category?: string;
  asn?: string;
  os_info?: string;
  vlan_id?: number;
}

export interface TrafficMatrixCell {
  source: string;
  destination: string;
  bytes: number;
}

export interface TrafficMatrixHistoryCell {
  timestamp: number;
  source: string;
  dest: string;
  bytes: number;
}

export interface Flow {
  id: string;
  src_ip: string;
  dst_ip: string;
  src_port: number;
  dst_port: number;
  protocol: string;
  app_protocol: string;
  bytes: number;
  packets: number;
  first_seen: number;
  last_seen: number;
  node_id: string;
  interface?: string;
  vlan_id?: number;
}

export interface ProtocolStat {
  protocol: string;
  bytes: number;
  packets: number;
  percentage: number;
  node_id: string;
  interface?: string;
}

export interface Alert {
  id: string;
  type: string;
  severity: string;
  message: string;
  source_ip?: string;
  node_id: string;
  timestamp: number;
  acknowledged: boolean;
}

export interface TrafficSnapshot {
  timestamp: string;
  bytes_per_sec: number;
  packets_per_sec: number;
  flows_count: number;
  node_id: string;
}

export interface HostSnapshot {
  timestamp: string;
  node_id: string;
  host_ip: string;
  bytes_sent: number;
  bytes_received: number;
  packets_sent: number;
  packets_received: number;
}

export interface TCPMetrics {
  active_tcp_flows: number;
  total_retransmits: number;
  total_rsts: number;
  total_zero_windows: number;
  total_out_of_order: number;
  rtt_avg_ms: number;
  rtt_min_ms: number;
  rtt_max_ms: number;
  rtt_samples: number;
  total_expected_pkts: number;
  total_lost_pkts: number;
  packet_loss_pct: number;
}

export interface LatencyStats {
  samples: number;
  avg_ms: number;
  min_ms: number;
  max_ms: number;
}

export interface VOIPStats {
  active_sessions: number;
  total_sessions: number;
  total_packets: number;
  total_bytes: number;
  total_lost: number;
  avg_jitter_ms: number;
  min_mos: number;
  avg_mos: number;
}

export interface VOIPSession {
  ssrc: number;
  src_ip: string;
  dst_ip: string;
  src_port: number;
  dst_port: number;
  packets: number;
  bytes: number;
  lost_packets: number;
  loss_pct: number;
  jitter_ms: number;
  max_jitter_ms: number;
  mos: number;
  codec: string;
  first_seen: number;
  last_seen: number;
  active: boolean;
}

export interface SystemHealth {
  cpu_percent: number;
  mem_percent: number;
  mem_used_bytes: number;
  mem_total_bytes: number;
  disk_free_bytes: number;
  disk_total_bytes: number;
  uptime_seconds: number;
}

export interface InterfaceInfo {
  name: string;
  bytes_per_sec: number;
  packets_per_sec: number;
  hosts_count: number;
  flows_count: number;
}

export interface NodeInfo {
  node_id: string;
  interface: string;
  interfaces: string[];
  interface_info: InterfaceInfo[];
  tags: string[];
  online: boolean;
  bytes_per_sec: number;
  packets_per_sec: number;
  hosts_count: number;
  flows_count: number;
  last_seen: number;
  version: string;
  system_health?: SystemHealth;
  tcp_metrics?: TCPMetrics;
  dns_latency?: LatencyStats;
  tls_latency?: LatencyStats;
  tcp_latency?: LatencyStats;
  voip_stats?: VOIPStats;
}

export interface Summary {
  hosts_count: number;
  active_flows: number;
  total_bytes: number;
  total_packets: number;
  uptime: string;
  nodes_online: number;
  nodes_total: number;
}

export interface DNSQueryStat {
  query_name: string;
  count: number;
  bytes: number;
}

export interface PacketSizeDist {
  size_64: number;
  size_128: number;
  size_256: number;
  size_512: number;
  size_1024: number;
  size_1500: number;
  size_gt1500: number;
}

export interface GlobalSnapshot {
  nodes: NodeInfo[];
  hosts: Host[];
  flows: Flow[];
  protocols: ProtocolStat[];
  traffic: TrafficSnapshot;
  alerts: Alert[];
  dns_queries: DNSQueryStat[];
  packet_size_dist?: PacketSizeDist;
}

export type NotificationChannelType = 'generic_webhook' | 'slack' | 'dingtalk' | 'feishu' | 'email' | 'telegram';

export interface NotificationChannel {
  id: string;
  name: string;
  type: NotificationChannelType;
  enabled: boolean;
  config: Record<string, unknown>;
}

export interface AlertThresholds {
  banned_ports: number[];
  port_scan_threshold: number;
  port_scan_window_sec: number;
  flow_flood_threshold: number;
  alert_cooldown_min: number;
  dns_suspicious_ports: number[] | null;
  suppressed_alert_types?: string[] | null;
  dns_exfil_query_min_len?: number;
  dns_exfil_min_bytes?: number;
  icmp_flood_threshold?: number;
  syn_flood_ratio?: number;
  horizontal_scan_threshold?: number;
  data_exfil_ratio?: number;
  unexpected_protocols?: string[] | null;
  arp_spoof_threshold?: number;
  long_flow_seconds?: number;
}

export type WSMessage =
  | { type: 'snapshot'; data: GlobalSnapshot }
  | { type: 'new_alert'; data: Alert }
  | { type: 'nodes_update'; data: NodeInfo[] };

// Report types
export interface SummaryReport {
  from: string;
  to: string;
  total_bytes: number;
  avg_bps: number;
  peak_bps: number;
  unique_hosts: number;
  total_flows: number;
  alert_count: number;
}

export interface TopTalker {
  ip: string;
  hostname: string;
  total_bytes: number;
  bytes_sent: number;
  bytes_received: number;
  flow_count: number;
}

export interface TopProtocol {
  name: string;
  total_bytes: number;
  pct_bytes: number;
  flow_count: number;
}

export interface AlertSummary {
  total: number;
  by_type: Record<string, number>;
  by_severity: Record<string, number>;
  recent: Alert[];
}

export interface TrendPoint {
  timestamp: string;
  bytes_per_sec: number;
  packets_per_sec: number;
  flows_count: number;
}

export interface SyslogRecord {
  id: string;
  timestamp: number;
  facility: string;
  severity: string;
  hostname: string;
  app_name: string;
  message: string;
  source: string;
}

export interface TrapMessage {
  version: string;
  community: string;
  enterprise_oid: string;
  agent_addr: string;
  generic_trap: number;
  specific_trap: number;
  timestamp: number;
  variables: Record<string, string>;
}

export interface LuaScript {
  name: string;
  content: string;
  enabled: boolean;
}

export interface InterceptRule {
  id: string;
  name: string;
  expression: string;
  action: string;
  enabled: boolean;
}
