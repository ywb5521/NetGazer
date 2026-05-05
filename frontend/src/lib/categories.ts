export const APP_CATEGORIES: Record<string, string> = {
  HTTP: 'Web',
  HTTPS: 'Web',
  TLS: 'Web',
  QUIC: 'Web',
  HTTP2: 'Web',
  WebSocket: 'Web',

  DNS: 'Network Services',
  DHCP: 'Network Services',
  SNMP: 'Network Services',
  NTP: 'Network Services',
  BGP: 'Network Services',
  OSPF: 'Network Services',
  ICMP: 'Network Services',
  ICMPv6: 'Network Services',
  ARP: 'Network Services',
  STUN: 'Network Services',

  SSH: 'Remote Access',
  RDP: 'Remote Access',
  Telnet: 'Remote Access',
  VNC: 'Remote Access',

  SMTP: 'Email',
  IMAP: 'Email',
  POP3: 'Email',
  SMTPS: 'Email',
  IMAPS: 'Email',

  FTP: 'File Transfer',
  SFTP: 'File Transfer',
  SMB: 'File Transfer',
  NFS: 'File Transfer',
  TFTP: 'File Transfer',

  SIP: 'Streaming',
  RTP: 'Streaming',
  RTSP: 'Streaming',
  H323: 'Streaming',

  MySQL: 'Database',
  PostgreSQL: 'Database',
  MongoDB: 'Database',
  Redis: 'Database',
  LDAP: 'Database',

  TCP: 'Other',
  UDP: 'Other',
};

export const CATEGORY_COLORS: Record<string, string> = {
  Web: 'var(--chart-1)',
  'Network Services': 'var(--chart-2)',
  'Remote Access': 'var(--chart-3)',
  Email: 'var(--chart-4)',
  'File Transfer': 'var(--chart-5)',
  Streaming: '#8b5cf6',
  Database: '#ec4899',
  Other: '#6b7280',
};

export function baseProtocol(appProtocol: string): string {
  if (!appProtocol) return '';
  // Strip enriched suffix like "TLS (github.com)" -> "TLS"
  const m = appProtocol.match(/^(.+?)\s*\(.+?\)$/);
  return (m ? m[1] : appProtocol).toUpperCase();
}

export function getCategory(appProtocol: string): string {
  const base = baseProtocol(appProtocol);
  if (!base) return 'Other';
  for (const [proto, cat] of Object.entries(APP_CATEGORIES)) {
    if (base === proto.toUpperCase()) return cat;
  }
  return 'Other';
}

export function protocolMatches(appProtocol: string, candidates: string[]): boolean {
  const base = baseProtocol(appProtocol);
  return candidates.some((c) => base === c.toUpperCase());
}
