# NetGazer

<p align="center">
  <em>Distributed Network Traffic Observability Platform</em>
</p>

<p align="center">
  <em>分布式网络流量可观测平台 —— 洞见每一帧，守护每一秒</em>
</p>

<p align="center">
  <a href="#features">Features</a> ·
  <a href="#quick-start">Quick Start</a> ·
  <a href="#deployment">Deployment</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="#development">Development</a> ·
  <a href="#api">API</a>
</p>

---

NetGazer is a self-hosted, distributed network traffic monitoring and observability platform. It combines real-time packet capture, deep packet inspection (nDPI + OpenGFW), geographic traffic analysis, a powerful alerting engine, and traffic interception — all accessible through a modern web dashboard.

Inspired by ntopng, written in Go, and built for today's hybrid networks.

---

## Features

### Real-time Monitoring
- **Live dashboard** with throughput charts, KPI cards, top hosts, and node status — updated every second via WebSocket
- **Active flows table** with protocol and application classification (400+ protocols via nDPI)
- **Host inventory** with traffic stats, search, multi-dimensional filtering (node, interface, country, ASN)
- **Traffic matrix** showing source–destination communication patterns

### Deep Packet Inspection
- **nDPI engine** — 400+ protocol signatures (TLS, HTTP, DNS, SSH, QUIC, DHCP, SSDP, etc.)
- **OpenGFW engine** — application-layer detection for TLS, HTTP, SSH, DNS, QUIC, WireGuard, OpenVPN, SOCKS, Trojan, Fully Encrypted Traffic (FTE)
- **DNS query monitoring** — top queried domains, query types, response codes

### Geographic Traffic Analysis
- **World map** with country-level traffic heatmap (Natural Earth 1:110m projection, 164 countries)
- **Country ranking** — traffic breakdown by country with bar charts
- **ASN ranking** — traffic breakdown by Autonomous System
- **Drill-down** from map/country/ASN to filtered host list
- Requires MaxMind GeoLite2 or GeoLite2-compatible `.mmdb` database

### Alerting Engine (15+ alert types)
| Alert | Description |
|-------|-------------|
| High Bandwidth | Node-level bandwidth exceeds threshold |
| New Device | Previously unseen MAC/IP appears on the network |
| Suspicious Port | Traffic to banned/blacklisted ports |
| Port Scan | Single source probes many destination ports |
| DNS Suspicious Port | DNS traffic on non-standard ports |
| Flow Flood | Excessive concurrent flows from a single source |
| DNS Exfiltration | Large DNS payloads or unusually long query names |
| ICMP Flood | Excessive ICMP traffic |
| SYN Flood | Many tiny TCP flows to many ports (potential DoS) |
| Horizontal Scan | Same port probed across many destination IPs |
| Data Exfiltration | Highly asymmetric outbound/inbound traffic ratio |
| Unexpected Protocol | Protocol not in the expected whitelist |
| ARP Spoofing | Multiple MAC addresses claiming the same IP |
| Long Flow | Flow active for an unusually long duration |

Alerts are deduplicated with configurable cooldown and forwarded to notification channels.

### Notifications
- **Multi-channel** — Slack, DingTalk, Feishu (Lark), Telegram, Email, Generic Webhook
- **Per-channel test** button to verify configuration
- **Alert severity routing** — configure which severity levels trigger which channels

### Traffic Interception
- Create **block / drop / allow rules** with a powerful expression language
- Match on: IP, port, protocol, TLS SNI, HTTP host, DNS query name, SSH software version, etc.
- Built-in functions: `geoip(country_code)`, `cidrMatch(ip, cidr)`
- Deploy rules to specific agent nodes in real time

### Lua Scripting
- Write custom alert logic in Lua with access to `get_hosts()`, `get_flows()`, `get_flow()`, `get_host()`
- Call `alert(severity, type, message)` to emit custom alerts
- Test scripts live before deploying

### Historical Reports & Export
- **Reports** — Summary, Top Talkers, Top Protocols, Alert Breakdown, Traffic Trend
- **Export** — JSON, CSV, NDJSON, ClickHouse TSV formats
- **Multi-tier retention** — raw → hourly (24h), hourly → daily (7d), daily → weekly (30d), weekly removed (365d)

### Advanced Protocols
- **VoIP Monitoring** — RTP/RTCP session tracking with MOS quality score, jitter, packet loss
- **NetFlow / sFlow** — Collect flow data from routers and switches
- **SNMP** — Poll network devices (v1/v2c/v3), receive SNMP traps
- **Syslog** — Built-in syslog message collector with severity/source filtering

### Host Pools
- Group hosts by CIDR ranges for aggregate traffic views
- CRUD management via API and UI

### Service Map
- Visualize service-to-service communication (TLS ↔ HTTP ↔ DNS ↔ SSH etc.)
- Clickable nodes and edges with drill-down to filtered flows

### Interface Monitoring
- Per-interface traffic statistics across all nodes
- Expand to see top hosts on each interface

### Internationalization
- Full **Chinese (简体中文)** and **English** UI
- Language persisted in localStorage

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Browser                              │
│              (React 19 + shadcn/ui + recharts)              │
└──────────┬──────────────┬──────────────┬────────────────────┘
           │ REST API     │ WebSocket    │ Static files
           ▼              ▼              ▼
    ┌──────────────────────────────────────────┐
    │          nginx (Docker container)         │
    │     Port 9527: HTTP + WS + Frontend       │
    │     Port 50051: HTTP/2 gRPC proxy         │
    └──────┬────────────────┬───────────────────┘
           │                │
           ▼                ▼
    ┌──────────────────────────────────────────────┐
    │           NetGazer Server (Docker)            │
    │     (with built-in static file serving)       │
    │  ┌─────────┐ ┌──────────┐ ┌───────────────┐  │
    │  │ HTTP    │ │ WebSocket│ │ Alert Engine   │  │
    │  │ API     │ │ Hub      │ │ (15+ checks)   │  │
    │  └────┬────┘ └────┬─────┘ └───────┬───────┘  │
    │       │           │               │          │
    │  ┌────┴───────────┴───────────────┴───────┐  │
    │  │           Aggregator                    │  │
    │  │  (hosts, flows, protocols, GeoIP, ASN) │  │
    │  └────────────────┬───────────────────────┘  │
    │                   │                          │
    │  ┌────────────────┴───────────────────────┐  │
    │  │           SQLite Storage               │  │
    │  │  (snapshots, alerts, config, pools)    │  │
    │  └────────────────────────────────────────┘  │
    │                   │                          │
    │  ┌────────────────┴───────────────────────┐  │
    │  │     gRPC Receiver  (:50051)            │  │
    │  └────────────────┬───────────────────────┘  │
    └───────────────────┼──────────────────────────┘
                        │ gRPC (bidirectional stream)
              ┌─────────┴──────────┐
              │                    │
    ┌─────────┴──────┐  ┌─────────┴──────┐
    │ NetGazer Agent │  │ NetGazer Agent │  ...
    │   (node1)      │  │   (node2)      │
    │ ┌────────────┐ │  │ ┌────────────┐ │
    │ │ libpcap    │ │  │ │ libpcap    │ │
    │ │ nDPI/OGFW  │ │  │ │ nDPI/OGFW  │ │
    │ │ Health     │ │  │ │ Health     │ │
    │ └────────────┘ │  │ └────────────┘ │
    └────────────────┘  └────────────────┘
```

---

## Quick Start

### Server（Docker 部署）

```bash
git clone https://github.com/yourorg/netgazer.git
cd netgazer
docker compose up -d
```

访问 `http://<服务器IP>:9527`，首次使用访问 `/setup` 创建管理员账户。

### Agent（在受监控的主机上）

从 [GitHub Releases](https://github.com/yourorg/netgazer/releases) 下载对应架构的 agent 包：

```bash
tar xzf netgazer-agent-*-linux-amd64.tar.gz
sudo setcap cap_net_raw,cap_net_admin=eip ./netgazer-agent

./netgazer-agent \
  --server-addr <服务器IP>:50051 \
  --interfaces eth0 \
  --node-id datacenter-1 \
  --tags production,rack-a
```

### 端口说明

| 端口 | 协议 | 用途 | 外部访问 |
|------|------|------|----------|
| `9527` | HTTP/1.1 | Dashboard + API + WebSocket | 浏览器访问 `http://<IP>:9527` |
| `50051` | HTTP/2 (gRPC) | Agent 连接 | Agent 通过 `--server-addr <IP>:50051` 连接 |

如果外面还有一层反向代理（如 1panel / 宝塔）：

- **9527 端口**：普通 HTTP 反代即可访问页面
- **50051 端口**：gRPC 走 HTTP/2，外层反代需要用 **L4 TCP 代理**模式，不能用 HTTP 反向代理

### 开发模式

```bash
# Terminal 1: Start backend
cd backend && go run ./cmd/server

# Terminal 2: Start frontend dev server
cd frontend && npm run dev
# Opens http://localhost:5173, proxies API & WS to :8080
```

---

## Configuration

### Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--http-port` | `8080` | HTTP & WebSocket listen port |
| `--grpc-port` | `50051` | gRPC listen port (agent communication) |
| `--db` | `./netgazer.db` | SQLite database path |
| `--retention` | `24h` | Raw data retention duration |
| `--bandwidth-threshold` | `100000000` | Default bandwidth alert threshold (bps) |
| `--tls-cert` | — | TLS certificate path for gRPC + HTTP |
| `--tls-key` | — | TLS key path |
| `--tls-ca` | — | CA certificate for mTLS |
| `--netflow-port` | `2055` | NetFlow collector port (0 to disable) |
| `--sflow-port` | `6343` | sFlow collector port (0 to disable) |
| `--syslog-port` | `0` | Syslog collector port (0 to disable) |
| `--trap-port` | `0` | SNMP trap receiver port (0 to disable) |
| `--snmp-config` | — | SNMP polling config JSON file path |
| `--discovery-subnets` | — | Subnets for ARP/ICMP discovery (comma-separated CIDRs) |
| `--geoip-country-db` | — | MaxMind GeoLite2 Country .mmdb path |
| `--geoip-asn-db` | — | MaxMind GeoLite2 ASN .mmdb path |
| `--node-auth` | `false` | Require authentication tokens for agent registration |
| `--web-dir` | — | Path to frontend dist/ for built-in static file serving |
| `--webhook-url` | — | Default webhook URL for notifications |

### Agent Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--server-addr` | `localhost:50051` | gRPC server address |
| `--interfaces` | `eth0` | Network interfaces to capture (comma-separated) |
| `--node-id` | hostname | Unique node identifier |
| `--bpf-filter` | — | BPF capture filter expression |
| `--tags` | — | Comma-separated node tags |
| `--proto-engine` | `ndpi` | Protocol detection: `ndpi`, `opengfw`, or `both` |
| `--intercept` | `false` | Enable traffic interception on this agent |
| `--tls-cert` / `--tls-key` / `--tls-ca` | — | TLS/mTLS for gRPC |
| `--auth-token` | — | Node authentication token |

### Runtime settings

Additional settings can be configured through the web UI under **Settings**:

- **Alert thresholds** — bandwidth, port scan window, flow flood limit, ICMP threshold, alert cooldown
- **Banned ports** — comma-separated list of ports that trigger suspicious-port alerts
- **DNS monitoring** — suspicious DNS ports and exfiltration detection parameters
- **Notification channels** — Slack, DingTalk, Feishu, Telegram, Email, Generic Webhook
- **GeoIP databases** — upload or download MaxMind .mmdb files
- **Lua scripts** — custom alert logic
- **BPF filter** — global packet capture filter applied to all agents
- **Node tokens** — agent authentication tokens

---

## Deployment

### Server（Docker）

```bash
git clone https://github.com/yourorg/netgazer.git
cd netgazer
docker compose up -d
```

启动后访问 `http://<服务器IP>:9527`，首次访问 `/setup` 创建管理员账户。

**加载 GeoIP 数据库（可选）：**

```bash
docker cp Geolite2-Country.mmdb netgazer-server:/var/lib/netgazer/geoip/country.mmdb
docker cp Geolite2-ASN.mmdb netgazer-server:/var/lib/netgazer/geoip/asn.mmdb
```

或者挂载本地目录到 compose 的 `netgazer_data` volume。

**外层反代（如 1panel / 宝塔）：**

Docker 内部已经做了 nginx 反代，外部如果需要绑定域名：
- **HTTP（9527）**：普通反代即可，目标是 `http://127.0.0.1:9527`
- **gRPC（50051）**：Agent 连接的端口，外层反代需要使用 **L4 TCP 代理**模式（不能是 HTTP 反代），目标 `127.0.0.1:50051`

### Agent（在受监控的机器上）

从 [GitHub Releases](https://github.com/yourorg/netgazer/releases) 下载 agent 包，解压运行：

```bash
tar xzf netgazer-agent-*-linux-amd64.tar.gz
sudo setcap cap_net_raw,cap_net_admin=eip ./netgazer-agent
./netgazer-agent --server-addr <服务器IP>:50051 --interfaces eth0 --node-id my-node
```

systemd 服务（推荐）：

```ini
# /etc/systemd/system/netgazer-agent.service
[Unit]
Description=NetGazer Agent
After=network.target

[Service]
Type=simple
ExecStart=/opt/netgazer/bin/netgazer-agent \
  --server-addr mgmt-server:50051 \
  --interfaces eth0 \
  --node-id %H
Restart=always
RestartSec=10
AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN

[Install]
WantedBy=multi-user.target
```

### GeoIP database

NetGazer supports MaxMind GeoLite2 `.mmdb` format for country and ASN lookups.

**Option 1: Upload via Web UI**
Settings → GeoIP → Upload → select file → choose type (Country / ASN)

**Option 2: Download via API**
```bash
curl -X POST http://localhost:8080/api/geoip/download \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/GeoLite2-Country.mmdb", "type": "country"}'
```

**Option 3: CLI flag**
```bash
./netgazer-server --geoip-country-db /path/to/GeoLite2-Country.mmdb \
                  --geoip-asn-db /path/to/GeoLite2-ASN.mmdb
```

> Note: Loyalsoldier GeoIP databases (commonly used in China for routing) are compatible but classify CDN IPs into non-geographic categories (CLOUDFRONT, CLOUDFLARE). For accurate geographic maps, use standard MaxMind GeoLite2 databases.

---

## API

### Overview

| Category | Endpoints | Description |
|----------|-----------|-------------|
| Auth | 3 | Login, setup, status check (public) |
| Real-time | 16 | Nodes, hosts, flows, protocols, traffic matrix, history |
| Alerts | 2 | List, acknowledge |
| Geo | 2 | Country stats, ASN stats |
| Notifications | 5 | CRUD + test for notification channels |
| Intercept | 5 | Traffic rule CRUD + deploy |
| Lua Scripts | 3 | CRUD + test |
| Host Pools | 5 | CRUD + stats |
| Reports | 5 | Summary, top talkers, top protocols, alerts, trend |
| Export | 3 | Snapshots, hosts, alerts in JSON/CSV/NDJSON/ClickHouse |
| GeoIP | 2 | Upload/download .mmdb databases |
| Config | 2 | Get/update server configuration |
| Misc | 6 | Service map, interfaces, syslog, SNMP traps, VoIP, node tokens |
| **Total** | **52** | |

Authentication: JWT Bearer token (24h validity). Include in `Authorization: Bearer <token>` header.

### Key endpoints

```
GET    /api/summary               — Global traffic summary
GET    /api/nodes                 — Agent node list with health
GET    /api/hosts?country=US      — Host list with filters
GET    /api/hosts/:ip             — Single host detail
GET    /api/flows?protocol=TCP    — Active flow list
GET    /api/protocols             — Protocol distribution
GET    /api/traffic/history       — Time-series traffic data
GET    /api/traffic-matrix        — Src–dst traffic matrix
GET    /api/geo/countries         — Traffic by country
GET    /api/geo/asns              — Traffic by ASN
GET    /api/service-map           — Service communication graph
GET    /api/interfaces            — All interfaces with stats
GET    /api/alerts                — Alert list
POST   /api/alerts/:id/ack        — Acknowledge alert
GET    /api/reports/summary       — Historical report
GET    /api/export/snapshots      — Export data
GET    /ws                        — Real-time WebSocket stream
```

---

## Project Structure

```
netgazer/
├── backend/
│   ├── cmd/
│   │   ├── server/main.go        # Server entry point (~670 lines)
│   │   └── agent/main.go         # Agent entry point (~240 lines)
│   ├── internal/
│   │   ├── aggregator/           # Central data aggregation
│   │   ├── alerting/             # 15+ alert checks
│   │   ├── api/                  # HTTP handlers, router, WebSocket hub
│   │   ├── auth/                 # JWT + password hashing
│   │   ├── capture/              # libpcap + nDPI + OpenGFW
│   │   ├── collector/            # NetFlow/sFlow/syslog/trap receivers
│   │   ├── config/               # CLI flag parsing
│   │   ├── discovery/            # ARP/ICMP network discovery
│   │   ├── geoip/                # MaxMind GeoIP2 lookup
│   │   ├── intercept/            # Traffic interception engine
│   │   ├── lua/                  # Lua scripting engine
│   │   ├── models/               # Shared data types
│   │   ├── receiver/             # gRPC server (agent receiver)
│   │   ├── report/               # Report generation + export
│   │   ├── reporter/             # gRPC client (agent side)
│   │   ├── snmp/                 # SNMP poller
│   │   ├── storage/              # SQLite persistence layer
│   │   ├── tracker/              # Per-agent state tracking
│   │   └── webhook/              # Notification dispatcher
│   └── gen/                      # Generated protobuf code
├── frontend/
│   ├── src/
│   │   ├── pages/                # 18 page components
│   │   ├── components/
│   │   │   ├── ui/               # 17 shadcn/ui primitives
│   │   │   ├── layout/           # AppShell, Sidebar
│   │   │   ├── dashboard/        # 12 dashboard widgets
│   │   │   ├── hosts/            # Host detail tabs
│   │   │   ├── geo/              # WorldMap + SVG paths
│   │   │   └── settings/         # 7 settings panels
│   │   ├── context/              # Auth + App state
│   │   ├── hooks/                # WebSocket hook
│   │   ├── lib/                  # API client, utils, i18n
│   │   └── i18n/                 # EN + ZH translations
│   └── vite.config.ts
├── proto/
│   └── netgazer/v1/agent.proto     # gRPC service definition
├── deploy/
│   └── nginx/
│       ├── netgazer.conf             # 路径前缀模式 nginx 配置
│       └── netgazer-docker.conf      # Docker 部署 nginx 配置
└── start.sh                      # Server start script
```

---

## FAQ

**Q: Why is my world map showing CLOUDFRONT/CLOUDFLARE instead of real countries?**

You are using a Loyalsoldier GeoIP database, which classifies CDN IP ranges into separate categories for routing purposes. Use a standard MaxMind GeoLite2 Country database for accurate geographic mapping.

**Q: How do I reduce alerts from internal traffic?**

Add internal subnets to the whitelist or increase the bandwidth/flood thresholds in Settings → Alert Thresholds. You can also use a BPF filter to exclude internal traffic from capture entirely.

**Q: Can I run the agent on a different machine from the server?**

Yes. The agent connects to the server via gRPC (`--server-addr`). Make sure the gRPC port (default 50051) is reachable from the agent machine. For production, enable TLS with `--tls-cert` and `--tls-key`.

**Q: How much data does the SQLite database grow per day?**

Approximately 50–100 MB/day for a busy network with ~50 hosts. Multi-tier aggregation compresses historical data: hourly after 24h, daily after 7d, weekly after 30d. A 1 GB database can typically hold 3–6 months of data for a small network.

**Q: What's the resource overhead on the agent?**

~50 MB RSS with nDPI protocol detection on a typical host. Packet capture uses libpcap with kernel bypass where available. CPU usage scales with traffic volume — typically 2–5% on a 100 Mbps link.

---

## License

MIT

---

## 中文说明

### NetGazer（网络守望者）

NetGazer 是一个自托管的分布式网络流量监控与可观测平台。它将实时抓包、深度包检测（nDPI + OpenGFW）、地理流量分析、强大的告警引擎和流量拦截功能整合在一个现代化的 Web 控制台中。

受 ntopng 启发，使用 Go 语言编写，专为当今混合网络环境打造。

### 核心功能

**实时监控** — 实时仪表盘（每秒 WebSocket 推送）、活跃流表（400+ 协议分类）、主机清单（支持国家/ASN/接口多维筛选）、流量矩阵

**深度包检测** — nDPI 引擎（400+ 协议）、OpenGFW 引擎（TLS/HTTP/SSH/DNS/QUIC/WireGuard/OpenVPN/SOCKS/Trojan 应用层检测）、DNS 查询监控

**地理流量分析** — SVG 世界地图热力图（Natural Earth 1:110m，164 个国家）、国家流量排名、AS 自治系统排名、点击下钻到主机列表

**告警引擎** — 15+ 种告警类型：高带宽、新设备、可疑端口、端口扫描、DNS 可疑端口、流量洪水、DNS 数据泄露、ICMP 洪水、SYN 洪水、横向扫描、数据泄露、异常协议、ARP 欺骗、长连接。告警去重 + 冷却期 + 多渠道通知

**通知渠道** — Slack、钉钉、飞书、Telegram、邮件、通用 Webhook，支持按严重级别路由

**流量拦截** — 基于表达式的拦截规则（IP/端口/协议/TLS SNI/HTTP Host/DNS 查询名），支持 `geoip()` 和 `cidrMatch()` 内置函数，可部署到指定节点

**Lua 脚本** — 自定义告警逻辑，访问 `get_hosts()`/`get_flows()` API，在线测试后部署

**历史报表与导出** — 概览/Top 主机/Top 协议/告警统计/趋势报表；JSON/CSV/NDJSON/ClickHouse 格式导出；多级数据聚合（原始→小时→天→周）

**其他** — VoIP 会话监控（MOS 质量评分）、NetFlow/sFlow 采集、SNMP 轮询/Trap、Syslog 收集、主机池（CIDR 分组）、服务地图、接口详情、中英文双语界面

### 快速开始

**Server（Docker 部署）**：

```bash
git clone https://github.com/yourorg/netgazer.git
cd netgazer
docker compose up -d
```

访问 `http://<服务器IP>:9527`，首次访问 `/setup` 创建管理员账户。

**Agent（在受监控主机上）**：

从 [GitHub Releases](https://github.com/yourorg/netgazer/releases) 下载 agent 包：

```bash
tar xzf netgazer-agent-*-linux-amd64.tar.gz
sudo setcap cap_net_raw,cap_net_admin=eip ./netgazer-agent
./netgazer-agent --server-addr <服务器IP>:50051 --interfaces eth0 --node-id my-node
```

### 端口说明

| 端口 | 协议 | 用途 | 外部访问 |
|------|------|------|----------|
| `9527` | HTTP | Dashboard + API + WebSocket | 普通 HTTP 反代 |
| `50051` | HTTP/2 (gRPC) | Agent 连接 | **L4 TCP 代理**（不是 HTTP 反代） |

### GeoIP 数据库

支持 MaxMind GeoLite2 `.mmdb` 格式。可通过 Web UI 上传、API 下载或命令行参数加载。

> 注意：Loyalsoldier 路由数据库兼容 MaxMind 格式，但会将 CDN IP 分类为非地理类别（如 CLOUDFRONT、CLOUDFLARE），导致地图数据不准确。如需精确地理地图，请使用标准 MaxMind GeoLite2 数据库。

### 部署

Server 仅支持 Docker 部署：`docker compose up -d` 一键启动，Dashboard → `http://localhost:9527`。Docker 内部已包含 nginx 反代，外部只需映射 9527（HTTP）和 50051（gRPC）两个端口。
