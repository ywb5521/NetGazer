export interface Translations {
  app: { title: string };
  nav: {
    dashboard: string;
    hosts: string;
    flows: string;
    protocols: string;
    alerts: string;
    nodes: string;
    filterByNode: string;
    allNodes: string;
    filterByInterface: string;
    allInterfaces: string;
    dns: string;
    reports: string;
    syslog: string;
    intercept: string;
    settings: string;
    geo: string;
    interfaces: string;
    pools: string;
    serviceMap: string;
    live: string;
    paused: string;
  };
  dashboard: {
    throughput: string;
    packetsPerSec: string;
    activeHosts: string;
    activeFlows: string;
    nodes: string;
    realtime: string;
    totalTracked: string;
    currentConnections: string;
    topHosts: string;
    topTalkers: string;
    accumulating: string;
    dnsQueries: string;
    noDns: string;
    packetSizeDist: string;
    noPacketData: string;
    trafficHistory: string;
    trafficMatrix: string;
    loadingMatrix: string;
    alertSummary: string;
    noAlerts: string;
    unacked: string;
    live: string;
    loadingHistory: string;
    liveStatus: string;
    ip: string;
    hostname: string;
    rate: string;
    total: string;
    domain: string;
    count: string;
    bytes: string;
    packets: string;
    customize: string;
    statsCards: string;
    trafficChart: string;
    packetSize: string;
    flowDirection: string;
    topologyMap: string;
    nodesOverview: string;
    topApps: string;
    dnsStats: string;
  };
  hosts: {
    title: string;
    hostDetail: string;
    backToList: string;
    searchPlaceholder: string;
    overview: string;
    traffic: string;
    protocols: string;
    peers: string;
    identity: string;
    trafficStats: string;
    bytesSent: string;
    bytesReceived: string;
    packetsSent: string;
    packetsReceived: string;
    timeline: string;
    firstSeen: string;
    lastSeen: string;
    avgRate: string;
    activeFlows: string;
    noHosts: string;
    noFlows: string;
    flat: string;
    bySubnet: string;
    hostname: string;
    name: string;
    mac: string;
    vendor: string;
    category: string;
    country: string;
    os: string;
    asn: string;
    subnet: string;
    hosts: string;
    page: string;
    of: string;
    prev: string;
    next: string;
  };
  flows: {
    title: string;
    searchPlaceholder: string;
    clearSearch: string;
    flowDetail: string;
    source: string;
    destination: string;
    protocol: string;
    app: string;
    duration: string;
    node: string;
    noFlows: string;
    vlan: string;
    web: string;
    dns: string;
    sshRdp: string;
    gt1mb: string;
    clear: string;
    allProto: string;
    allApps: string;
    minBytes: string;
    prev: string;
    next: string;
    page: string;
    of: string;
    flowId: string;
    close: string;
  };
  protocols: {
    title: string;
    chart: string;
    table: string;
    categories: string;
    web: string;
    networkServices: string;
    remoteAccess: string;
    email: string;
    fileTransfer: string;
    streaming: string;
    database: string;
    other: string;
  };
  alerts: {
    title: string;
    all: string;
    critical: string;
    warning: string;
    info: string;
    allTypes: string;
    acked: string;
    ack: string;
    noAlerts: string;
    page: string;
    of: string;
    prev: string;
    next: string;
  };
  nodes: {
    title: string;
    online: string;
    offline: string;
    interface: string;
    interfaces: string;
    interfacesCount: string;
    version: string;
    tags: string;
    lastSeen: string;
    startAgent: string;
    systemHealth: string;
    diskFree: string;
    uptime: string;
    tcpHealth: string;
    flows_: string;
    retx: string;
    rst: string;
    rtt: string;
    packetLoss: string;
    pkts: string;
    zeroWindow: string;
    outOfOrder: string;
    na: string;
    appLatency: string;
    dnsLatency: string;
    tlsLatency: string;
    tcpRtt: string;
    voipQuality: string;
    voipSessions: string;
    voipJitter: string;
    voipMos: string;
    voipLost: string;
  };
  syslog: {
    title: string;
    allSeverities: string;
    filterSource: string;
    records: string;
    time: string;
    severity: string;
    facility: string;
    hostname: string;
    appName: string;
    message: string;
    source: string;
    loading: string;
    noMessages: string;
    of: string;
    prev: string;
    next: string;
  };
  lua: {
    title: string;
    description: string;
    newScript: string;
    noScripts: string;
    scriptName: string;
    enabled: string;
    save: string;
    saving: string;
    test: string;
    delete: string;
    nodeId: string;
    testOk: string;
    testError: string;
    placeholder: string;
  };
  settings: {
    title: string;
    alertThresholds: string;
    thresholdDesc: string;
    bandwidthThreshold: string;
    current: string;
    save: string;
    saved: string;
    systemInfo: string;
    version: string;
    frontend: string;
    backend: string;
    database: string;
    bannedPorts: string;
    bannedPortsPlaceholder: string;
    bannedPortsDesc: string;
    portScanThreshold: string;
    portScanThresholdDesc: string;
    portScanWindow: string;
    portScanWindowDesc: string;
    flowFloodThreshold: string;
    flowFloodThresholdDesc: string;
    alertCooldown: string;
    alertCooldownDesc: string;
    dnsSuspiciousPorts: string;
    dnsSuspiciousPortsPlaceholder: string;
    dnsSuspiciousPortsDesc: string;
    resetToDefaults: string;
    bpfFilter: string;
    bpfFilterDesc: string;
    bpfFilterPlaceholder: string;
    bpfFilterSaved: string;
    bpfFilterSaveFailed: string;
    bpfFilterCleared: string;
    bpfFilterClearFailed: string;
    clear: string;
    notificationChannels: string;
    notificationChannelsDesc: string;
    channelName: string;
    addChannel: string;
    add: string;
    cancel: string;
    test: string;
    ok: string;
    fail: string;
    testing: string;
    noChannels: string;
    geoip: string;
    geoipDesc: string;
    geoipCountryDb: string;
    geoipAsnDb: string;
    geoipUploadFile: string;
    geoipUpload: string;
    geoipDownloadUrl: string;
    geoipDownload: string;
    geoipDownloading: string;
    geoipStatus: string;
    geoipNotLoaded: string;
    geoipReady: string;
    geoipFile: string;
    nodeTokens: string;
    nodeTokensDesc: string;
    generateToken: string;
    tokenDescription: string;
    tokenDescriptionPlaceholder: string;
    tokenCreated: string;
    tokenCreatedWarning: string;
    copyToken: string;
    tokenCopied: string;
    revokeToken: string;
    noTokens: string;
  };
  reports: {
    title: string;
    description: string;
    from: string;
    to: string;
    generating: string;
    generateReport: string;
    export: string;
    totalTraffic: string;
    avgThroughput: string;
    peakThroughput: string;
    uniqueHosts: string;
    alerts: string;
    trafficTrend: string;
    throughput: string;
    topTalkers: string;
    ip: string;
    total: string;
    sent: string;
    received: string;
    topProtocols: string;
    traffic: string;
    alertSummary: string;
    byType: string;
    type: string;
    count: string;
    bySeverity: string;
    severity: string;
    recentAlerts: string;
    reportGenerated: string;
    reportFailed: string;
    exportSuccess: string;
    exportFailed: string;
  };
  intercept: {
    title: string;
    description: string;
    newRule: string;
    noRules: string;
    ruleName: string;
    expression: string;
    action: string;
    block: string;
    drop: string;
    allow: string;
    enabled: string;
    save: string;
    saving: string;
    edit: string;
    delete: string;
    deploy: string;
    deployTitle: string;
    selectNodes: string;
    allNodes: string;
    deploying: string;
    deployed: string;
    nodeRules: string;
    noNodeRules: string;
    deleteConfirm: string;
    expressionPlaceholder: string;
    actionBlockDesc: string;
    actionDropDesc: string;
    actionAllowDesc: string;
    reference: string;
    referenceContent: string;
    selectRules: string;
    allRules: string;
  };
  auth: {
    loginTitle: string;
    loginPrompt: string;
    loginButton: string;
    passwordPlaceholder: string;
    usernamePlaceholder: string;
    invalidPassword: string;
    logout: string;
  };
  geo: {
    title: string;
    map: string;
    countries: string;
    asns: string;
    country: string;
    hosts: string;
    bytes: string;
    percentage: string;
    loading: string;
    empty: string;
  };
  hostPools: {
    title: string;
    description: string;
    newPool: string;
    editPool: string;
    deletePool: string;
    noPools: string;
    name: string;
    cidrs: string;
    cidrsPlaceholder: string;
    hosts: string;
    totalTraffic: string;
    save: string;
    cancel: string;
    deleteConfirm: string;
    addCidr: string;
  };
  interfaces: {
    title: string;
    node: string;
    name: string;
    throughput: string;
    packets: string;
    hosts: string;
    flows: string;
    loading: string;
    empty: string;
  };
  serviceMap: {
    title: string;
    loading: string;
    empty: string;
    services: string;
    edges: string;
  };
  common: {
    language: string;
    switchTo: string;
    connected: string;
    disconnected: string;
    nodesOnline: string;
    loading: string;
    error: string;
    empty: string;
    bytesPerSec: string;
    noData: string;
    accumulatingData: string;
    current: string;
    baseline: string;
    sourceIps: string;
    destinationIps: string;
    trafficFlow: string;
    compareWithPrev: string;
    load: string;
    raw: string;
    hourly: string;
    daily: string;
    weekly: string;
    close: string;
  };
}

export const zh: Translations = {
  app: { title: 'gtopng - 网络流量监控' },
  nav: {
    dashboard: '仪表盘',
    hosts: '主机',
    flows: '流量',
    protocols: '协议',
    alerts: '告警',
    nodes: '节点',
    filterByNode: '按节点筛选',
    allNodes: '全部节点',
    filterByInterface: '按网卡筛选',
    allInterfaces: '全部网卡',
    dns: 'DNS 查询',
    reports: '报表',
    syslog: '系统日志',
    intercept: '流量拦截',
    settings: '设置',
    geo: '地理',
    interfaces: '网卡',
    pools: '主机池',
    serviceMap: '服务地图',
    live: '实时',
    paused: '已暂停',
  },
  dashboard: {
    throughput: '吞吐量',
    packetsPerSec: '包/秒',
    activeHosts: '活跃主机',
    activeFlows: '活跃流',
    nodes: '个节点',
    realtime: '实时',
    totalTracked: '已追踪',
    currentConnections: '当前连接',
    topHosts: 'Top 5 主机',
    topTalkers: 'Top Talkers (速率)',
    accumulating: '数据收集中...',
    dnsQueries: 'DNS 查询',
    noDns: '未检测到 DNS 查询',
    packetSizeDist: '数据包大小分布',
    noPacketData: '暂无数据包数据',
    trafficHistory: '流量历史',
    trafficMatrix: '流量矩阵',
    loadingMatrix: '加载流量矩阵中...',
    alertSummary: '告警概览',
    noAlerts: '暂无告警',
    unacked: '未确认',
    live: '实时',
    loadingHistory: '加载历史...',
    liveStatus: '实时',
    ip: 'IP',
    hostname: '主机名',
    rate: '速率',
    total: '总计',
    domain: '域名',
    count: '次数',
    bytes: '字节',
    packets: '数据包',
    customize: '自定义',
    statsCards: '统计卡片',
    trafficChart: '流量图表',
    packetSize: '数据包大小',
    flowDirection: '流量方向',
    topologyMap: '拓扑图',
    nodesOverview: '节点概览',
    topApps: '热门应用',
    dnsStats: 'DNS 统计',
  },
  hosts: {
    title: '主机',
    hostDetail: '主机详情',
    backToList: '返回列表',
    searchPlaceholder: '搜索 IP、主机名、MAC...',
    overview: '概览',
    traffic: '流量',
    protocols: '协议',
    peers: '通信对端',
    identity: '设备信息',
    trafficStats: '流量统计',
    bytesSent: '已发送',
    bytesReceived: '已接收',
    packetsSent: '已发送包',
    packetsReceived: '已接收包',
    timeline: '活动时间',
    firstSeen: '首次发现',
    lastSeen: '最近活跃',
    avgRate: '平均速率',
    activeFlows: '活跃流',
    noHosts: '未检测到主机',
    noFlows: '无相关流量',
    flat: '平铺',
    bySubnet: '按子网',
    hostname: '主机名',
    name: '名称',
    mac: 'MAC',
    vendor: '厂商',
    category: '分类',
    country: '国家',
    os: '操作系统',
    asn: 'ASN',
    subnet: '子网',
    hosts: '主机',
    page: '页',
    of: '/',
    prev: '上一页',
    next: '下一页',
  },
  flows: {
    title: '流量',
    searchPlaceholder: '按 IP、端口或协议搜索...',
    clearSearch: '清除搜索',
    flowDetail: '流量详情',
    source: '源地址',
    destination: '目标地址',
    protocol: '协议',
    app: '应用',
    duration: '持续时间',
    node: '节点',
    noFlows: '暂无流量数据',
    vlan: 'VLAN',
    web: 'Web',
    dns: 'DNS',
    sshRdp: 'SSH/RDP',
    gt1mb: '>1MB',
    clear: '清除',
    allProto: '全部协议',
    allApps: '全部应用',
    minBytes: '最小字节',
    prev: '上一页',
    next: '下一页',
    page: '页',
    of: '/',
    flowId: '流 ID',
    close: '关闭',
  },
  protocols: {
    title: '协议',
    chart: '图表',
    table: '表格',
    categories: '分类',
    web: 'Web',
    networkServices: '网络服务',
    remoteAccess: '远程访问',
    email: '邮件',
    fileTransfer: '文件传输',
    streaming: '流媒体',
    database: '数据库',
    other: '其他',
  },
  alerts: {
    title: '告警',
    all: '全部',
    critical: '严重',
    warning: '警告',
    info: '提示',
    allTypes: '全部类型',
    acked: '已确认',
    ack: '确认',
    noAlerts: '暂无告警',
    page: '页',
    of: '/',
    prev: '上一页',
    next: '下一页',
  },
  nodes: {
    title: '节点',
    online: '在线',
    offline: '离线',
    interface: '网卡',
    interfaces: '网卡列表',
    interfacesCount: '{count} 张网卡',
    version: '版本',
    tags: '标签',
    lastSeen: '最近活跃',
    startAgent: '启动 gtopng-agent 以开始监控',
    systemHealth: '系统健康',
    diskFree: '磁盘可用',
    uptime: '运行时间',
    tcpHealth: 'TCP 健康',
    flows_: '流',
    retx: '重传',
    rst: 'RST',
    rtt: 'RTT',
    packetLoss: '丢包率',
    pkts: '包',
    zeroWindow: '零窗口',
    outOfOrder: '乱序',
    na: '无',
    appLatency: '应用延迟',
    dnsLatency: 'DNS',
    tlsLatency: 'TLS',
    tcpRtt: 'TCP RTT',
    voipQuality: 'VoIP 质量',
    voipSessions: '会话',
    voipJitter: '抖动',
    voipMos: 'MOS',
    voipLost: '丢包',
  },
  syslog: {
    title: '系统日志',
    allSeverities: '全部级别',
    filterSource: '筛选来源...',
    records: ' 条记录',
    time: '时间',
    severity: '级别',
    facility: '设施',
    hostname: '主机名',
    appName: '应用',
    message: '消息',
    source: '来源',
    loading: '加载中...',
    noMessages: '暂无系统日志',
    of: '/',
    prev: '上一页',
    next: '下一页',
  },
  lua: {
    title: 'Lua 脚本',
    description: '编写在每次告警检查周期中运行的 Lua 脚本。定义 on_check(node_id) 函数。可用函数: alert(severity, type, message), get_hosts(node_id), get_flows(node_id), get_host(ip), get_flow(id)。',
    newScript: '+ 新建脚本',
    noScripts: '暂无脚本',
    scriptName: '脚本名称',
    enabled: '已启用',
    save: '保存',
    saving: '保存中...',
    test: '测试',
    delete: '删除',
    nodeId: '节点 ID (可选)',
    testOk: 'OK - 脚本运行成功',
    testError: '错误',
    placeholder: '-- Lua 脚本\nfunction on_check(node_id)\n  local hosts = get_hosts(node_id)\n  for _, h in ipairs(hosts) do\n    if h.bytes_sent > 1000000000 then\n      alert("warning", "traffic", "High traffic from " .. h.ip)\n    end\n  end\nend',
  },
  settings: {
    title: '设置',
    alertThresholds: '告警阈值',
    thresholdDesc: '微调告警触发条件和检测灵敏度。修改后立即生效。',
    bandwidthThreshold: '带宽阈值 (Mbps)',
    current: '当前',
    save: '保存',
    saved: '已保存',
    systemInfo: '系统信息',
    version: '版本',
    frontend: '前端',
    backend: '后端',
    database: '数据库',
    bannedPorts: '禁用的端口 (逗号分隔)',
    bannedPortsPlaceholder: '23, 3389, 445',
    bannedPortsDesc: '当流量目标为这些端口时触发告警。',
    portScanThreshold: '端口扫描阈值',
    portScanThresholdDesc: '触发告警前扫描的唯一端口数 (1-1000)',
    portScanWindow: '端口扫描窗口 (秒)',
    portScanWindowDesc: '端口扫描检测的时间窗口 (10-600s)',
    flowFloodThreshold: '流量洪泛阈值',
    flowFloodThresholdDesc: '触发告警前每个 IP 的并发流数 (1-10000)',
    alertCooldown: '告警冷却时间 (分钟)',
    alertCooldownDesc: '重复告警之间的最小间隔 (1-1440)',
    dnsSuspiciousPorts: 'DNS 可疑端口 (逗号分隔，空 = 所有非 53 端口)',
    dnsSuspiciousPortsPlaceholder: '空 = 标记所有非 53 端口',
    dnsSuspiciousPortsDesc: '当 DNS 流量出现在这些端口时被标记。留空则标记所有非 53 端口。',
    resetToDefaults: '恢复默认值',
    bpfFilter: 'BPF 捕获过滤器',
    bpfFilterDesc: '应用于所有 Agent 的 Berkeley Packet Filter 表达式。只捕获匹配的数据包。留空则捕获所有流量。常见示例: tcp port 80, not host 10.0.0.1, net 192.168.1.0/24。',
    bpfFilterPlaceholder: '例如: tcp port 80 或 not host 10.0.0.1',
    bpfFilterSaved: 'BPF 过滤器已保存 — Agent 将在下次连接时收到更新',
    bpfFilterSaveFailed: '保存 BPF 过滤器失败',
    bpfFilterCleared: 'BPF 过滤器已清除',
    bpfFilterClearFailed: '清除 BPF 过滤器失败',
    clear: '清除',
    notificationChannels: '通知渠道',
    notificationChannelsDesc: '将告警转发到外部服务。可以启用一个或多个渠道。',
    channelName: '渠道名称',
    addChannel: '添加渠道',
    add: '添加',
    cancel: '取消',
    test: '测试',
    ok: 'OK',
    fail: '失败',
    testing: '...',
    noChannels: '未配置通知渠道。',
    geoip: 'GeoIP 数据库',
    geoipDesc: '上传 MaxMind GeoLite2 .mmdb 文件，或输入下载 URL 自动获取 IP 地理位置数据库。支持 Country 和 ASN 两类数据库。',
    geoipCountryDb: '国家数据库 (Country)',
    geoipAsnDb: 'ASN 数据库',
    geoipUploadFile: '上传文件',
    geoipUpload: '上传',
    geoipDownloadUrl: '下载 URL',
    geoipDownload: '下载',
    geoipDownloading: '下载中...',
    geoipStatus: '当前状态',
    geoipNotLoaded: '未加载',
    geoipReady: '已就绪',
    geoipFile: '文件',
    nodeTokens: '节点认证',
    nodeTokensDesc: '管理 agent 节点的访问令牌。启用节点认证后，只有持有有效令牌的 agent 才能连接并上报数据。',
    generateToken: '生成令牌',
    tokenDescription: '描述',
    tokenDescriptionPlaceholder: '例如：办公室路由器、数据中心节点1',
    tokenCreated: '令牌已生成',
    tokenCreatedWarning: '请安全保存此令牌，它将不会再次显示。',
    copyToken: '复制',
    tokenCopied: '已复制！',
    revokeToken: '撤销',
    noTokens: '暂无令牌。生成一个令牌以启用节点认证。',
  },
  reports: {
    title: '历史报表',
    description: '生成任意时间范围的报表。将数据导出到外部系统。',
    from: '从',
    to: '至',
    generating: '生成中...',
    generateReport: '生成报表',
    export: '导出:',
    totalTraffic: '总流量',
    avgThroughput: '平均吞吐量',
    peakThroughput: '峰值吞吐量',
    uniqueHosts: '唯一主机',
    alerts: '告警',
    trafficTrend: '流量趋势',
    throughput: '吞吐量',
    topTalkers: 'Top Talkers',
    ip: 'IP',
    total: '总计',
    sent: '已发送',
    received: '已接收',
    topProtocols: '热门协议',
    traffic: '流量',
    alertSummary: '告警概览',
    byType: '按类型',
    type: '类型',
    count: '数量',
    bySeverity: '按级别',
    severity: '级别',
    recentAlerts: '最近告警',
    reportGenerated: '报表已生成',
    reportFailed: '生成报表失败',
    exportSuccess: '已导出',
    exportFailed: '导出失败',
  },
  intercept: {
    title: '流量拦截',
    description: '管理基于协议分析结果的流量拦截规则。规则通过 expr 表达式匹配流量特征，支持阻断、丢弃和放行动作。',
    newRule: '+ 新建规则',
    noRules: '暂无拦截规则',
    ruleName: '规则名称',
    expression: '表达式',
    action: '动作',
    block: '阻断',
    drop: '丢弃',
    allow: '放行',
    enabled: '启用',
    save: '保存',
    saving: '保存中...',
    edit: '编辑',
    delete: '删除',
    deploy: '下发到节点',
    deployTitle: '下发规则到节点',
    selectNodes: '选择目标节点（空 = 全部节点）',
    allNodes: '全部节点',
    deploying: '下发中...',
    deployed: '已下发到 {count} 个节点',
    nodeRules: '节点规则状态',
    noNodeRules: '暂无节点规则信息',
    deleteConfirm: '确定要删除此规则吗？',
    expressionPlaceholder: '例如: tls.req.sni endsWith "bad.example.com"',
    actionBlockDesc: '阻断整条 TCP 连接',
    actionDropDesc: '丢弃当前包',
    actionAllowDesc: '明确放行',
    reference: '表达式参考',
    referenceContent: '表达式使用 expr-lang/expr 语法，可引用各协议分析器提取的属性。分析器属性以协议名为根，包含 req（请求）和 resp（响应）两个子对象。\n\n支持的分析器属性:\n• tls.req.sni — TLS SNI (字符串)\n• tls.req.versions — TLS 版本列表\n• tls.req.ciphers — 密码套件列表\n• tls.resp.version — 服务端选择的 TLS 版本\n• http.req.method — HTTP 方法\n• http.req.host — HTTP Host 头\n• http.req.path — HTTP 路径\n• http.req.user_agent — User-Agent\n• ssh.req.version — SSH 协议版本\n• ssh.req.software — SSH 客户端软件\n• socks.req.target_addr — SOCKS 目标地址\n• dns.req.questions — DNS 查询名列表\n• dns.resp.answers — DNS 应答列表\n• quic.req.sni — QUIC SNI\n• wireguard.message_type — WireGuard 消息类型 (1=HS, 2=Data, 3=Cookie)\n• openvpn.opcode — OpenVPN 操作码\n• trojan.yes — Trojan 检测为真\n• fet.yes — 全加密流量 (可能的 Shadowsocks/VMess)\n• ip.src / ip.dst — 源/目标 IP\n• tcp.src_port / tcp.dst_port — TCP 端口\n\n可用内置函数:\n• geoip(ip, code) — 判断 IP 归属国家/地区, code 为 ISO 3166-1 代码 (如 CN、US、RU、GB 等)\n• cidrMatch(ip, cidr) — IP 是否在 CIDR 网段内\n• lookup(host, server) — DNS 查询\n\n表达式示例:\n• tls.req.sni endsWith \".example.com\"\n• \"badhost\" in dns.req.questions\n• fet.yes and tcp.dst_port > 1024\n• http.req.method == \"POST\" and http.req.path contains \"/api\"\n• geoip(ip.src, \"RU\") — 阻断来自俄罗斯的源 IP\n• geoip(ip.dst, \"CN\") and tcp.dst_port == 443 — 阻断发往中国的 HTTPS 流量\n• fet.yes and not (geoip(ip.src, \"US\") or geoip(ip.src, \"GB\")) — 阻断来自非美英国家的加密流量',
    selectRules: '选择规则（空 = 全部已启用规则）',
    allRules: '全部规则',
  },
  auth: {
    loginTitle: 'gtopng',
    loginPrompt: '请输入管理员密码',
    loginButton: '登录',
    passwordPlaceholder: '密码',
    usernamePlaceholder: '用户名',
    invalidPassword: '密码错误',
    logout: '退出登录',
  },
  geo: {
    title: '地理流量',
    map: '世界地图',
    countries: '国家排名',
    asns: 'AS 自治系统',
    country: '国家',
    hosts: '主机数',
    bytes: '流量',
    percentage: '占比',
    loading: '加载中...',
    empty: '暂无地理数据',
  },
  hostPools: {
    title: '主机池',
    description: '通过 CIDR 网段将主机分组管理，查看池内流量汇总。',
    newPool: '新建主机池',
    editPool: '编辑主机池',
    deletePool: '删除',
    noPools: '暂无主机池',
    name: '名称',
    cidrs: 'CIDR 网段',
    cidrsPlaceholder: '例如: 10.0.0.0/8, 192.168.1.0/24',
    hosts: '主机数',
    totalTraffic: '总流量',
    save: '保存',
    cancel: '取消',
    deleteConfirm: '确认删除此主机池？',
    addCidr: '添加 CIDR',
  },
  interfaces: {
    title: '网卡详情',
    node: '节点',
    name: '网卡名',
    throughput: '吞吐量',
    packets: '包速率',
    hosts: '主机数',
    flows: '流数',
    loading: '加载中...',
    empty: '暂无网卡数据',
  },
  serviceMap: {
    title: '服务地图',
    loading: '加载中...',
    empty: '暂无服务流量数据',
    services: '服务',
    edges: '通信',
  },
  common: {
    language: 'Language',
    switchTo: '切换到',
    connected: '连接',
    disconnected: '已断开',
    nodesOnline: '{online}/{total} 节点在线',
    loading: '加载中...',
    error: '错误',
    empty: '暂无数据',
    bytesPerSec: 'B/s',
    noData: '暂无数据',
    accumulatingData: '数据收集中...',
    current: '当前',
    baseline: '基线',
    sourceIps: '源 IP',
    destinationIps: '目标 IP',
    trafficFlow: '流量',
    compareWithPrev: '对比上一时段',
    load: '加载',
    raw: '原始 (10s)',
    hourly: '每小时',
    daily: '每日',
    weekly: '每周',
    close: '关闭',
  },
};
