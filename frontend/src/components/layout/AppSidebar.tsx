import { NavLink } from 'react-router-dom';
import { LayoutDashboard, Monitor, ArrowLeftRight, PieChart, Bell, Server, Globe, Settings, Search, FileText, Pause, Play, LogOut, FileWarning, Shield, Map, Cable, Layers, GitBranch } from 'lucide-react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { useAppContext } from '@/context/AppContext';
import { useAuth } from '@/context/AuthContext';
import { useI18n } from '@/i18n/I18nContext';

export function AppSidebar() {
  const { snapshot, connected, selectedNode, setSelectedNode, selectedInterface, setSelectedInterface, autoRefresh, toggleAutoRefresh } = useAppContext();
  const { logout } = useAuth();
  const { t, toggleLang, lang } = useI18n();
  const online = snapshot?.nodes?.filter((n) => n.online).length ?? 0;
  const total = snapshot?.nodes?.length ?? 0;

  const navItems = [
    { title: t.nav.dashboard, to: '/', icon: LayoutDashboard },
    { title: t.nav.hosts, to: '/hosts', icon: Monitor },
    { title: t.nav.flows, to: '/flows', icon: ArrowLeftRight },
    { title: t.nav.protocols, to: '/protocols', icon: PieChart },
    { title: t.nav.alerts, to: '/alerts', icon: Bell },
    { title: t.nav.nodes, to: '/nodes', icon: Server },
    { title: t.nav.dns, to: '/dns', icon: Search },
    { title: t.nav.reports, to: '/reports', icon: FileText },
    { title: t.nav.syslog, to: '/syslog', icon: FileWarning },
    { title: t.nav.intercept, to: '/intercept', icon: Shield },
    { title: t.nav.geo, to: '/geo', icon: Map },
    { title: t.nav.interfaces, to: '/interfaces', icon: Cable },
    { title: t.nav.pools, to: '/pools', icon: Layers },
    { title: t.nav.serviceMap, to: '/service-map', icon: GitBranch },
    { title: t.nav.settings, to: '/settings', icon: Settings },
  ];

  const connectedText = connected
    ? t.common.nodesOnline.replace('{online}', String(online)).replace('{total}', String(total))
    : t.common.disconnected;

  return (
    <aside className="fixed left-0 top-0 z-40 h-screen w-56 border-r border-border bg-card flex flex-col">
      <div className="flex h-14 items-center gap-2 border-b border-border px-4">
        <div className="flex items-center gap-2 flex-1">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary">
            <span className="text-xs font-bold text-primary-foreground">G</span>
          </div>
          <span className="font-semibold text-sm">netgazer</span>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          title={lang === 'zh' ? 'Switch to English' : '切换到中文'}
          onClick={toggleLang}
        >
          <Globe className="h-3.5 w-3.5" />
        </Button>
      </div>

      <nav className="flex-1 overflow-auto py-4 px-3">
        <ul className="flex flex-col gap-1">
          {navItems.map((item) => (
            <li key={item.to}>
              <NavLink
                to={item.to}
                end={item.to === '/'}
                className={({ isActive }) =>
                  cn(
                    'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-accent text-accent-foreground'
                      : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                  )
                }
              >
                <item.icon className="h-4 w-4" />
                {item.title}
              </NavLink>
            </li>
          ))}
        </ul>

        {snapshot && snapshot.nodes.length > 0 && (
          <div className="mt-4 px-1 space-y-2">
            <div>
              <p className="text-xs text-muted-foreground mb-1.5 px-2">{t.nav.filterByNode}</p>
              <Select
                value={selectedNode || 'all'}
                onValueChange={(v) => {
                  setSelectedNode(v === 'all' ? '' : v);
                  setSelectedInterface('');
                }}
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder={t.nav.allNodes} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.nav.allNodes}</SelectItem>
                  {snapshot.nodes.map((n) => (
                    <SelectItem key={n.node_id} value={n.node_id}>{n.node_id}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {selectedNode && (() => {
              const node = snapshot.nodes.find((n) => n.node_id === selectedNode);
              const ifaces = node?.interfaces;
              if (ifaces && ifaces.length > 0) {
                return (
                  <div>
                    <p className="text-xs text-muted-foreground mb-1.5 px-2">{t.nav.filterByInterface}</p>
                    <Select
                      value={selectedInterface || 'all'}
                      onValueChange={(v) => setSelectedInterface(v === 'all' ? '' : v)}
                    >
                      <SelectTrigger className="h-8 text-xs">
                        <SelectValue placeholder={t.nav.allInterfaces} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">{t.nav.allInterfaces}</SelectItem>
                        {ifaces.map((iface) => (
                          <SelectItem key={iface} value={iface}>{iface}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                );
              }
              return null;
            })()}
          </div>
        )}
      </nav>

      <div className="border-t border-border p-4 space-y-2">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span className={cn(
            'inline-block h-2 w-2 rounded-full',
            connected ? 'bg-green-500' : 'bg-red-500'
          )} />
          {connectedText}
        </div>
        <Button
          variant={autoRefresh ? 'ghost' : 'secondary'}
          size="sm"
          className="h-7 w-full text-xs justify-start"
          onClick={toggleAutoRefresh}
        >
          {autoRefresh ? (
            <><Pause className="mr-1 h-3 w-3" /> {t.nav.live}</>
          ) : (
            <><Play className="mr-1 h-3 w-3" /> {t.nav.paused}</>
          )}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-full text-xs justify-start text-muted-foreground"
          onClick={logout}
        >
          <LogOut className="mr-1 h-3 w-3" /> {t.auth.logout}
        </Button>
      </div>
    </aside>
  );
}
