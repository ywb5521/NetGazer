import { lazy, Suspense, useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { Toaster } from 'sonner';
import { AppLayout } from '@/components/layout/AppLayout';
import { AppContextProvider } from '@/context/AppContext';
import { AuthContextProvider, useAuth } from '@/context/AuthContext';
import { I18nProvider } from '@/i18n/I18nContext';
import { checkSetup } from '@/lib/api';

const DashboardPage = lazy(() => import('@/pages/DashboardPage'));
const HostsPage = lazy(() => import('@/pages/HostsPage'));
const HostDetailPage = lazy(() => import('@/pages/HostDetailPage'));
const FlowsPage = lazy(() => import('@/pages/FlowsPage'));
const ProtocolsPage = lazy(() => import('@/pages/ProtocolsPage'));
const AlertsPage = lazy(() => import('@/pages/AlertsPage'));
const NodesPage = lazy(() => import('@/pages/NodesPage'));
const DnsPage = lazy(() => import('@/pages/DnsPage'));
const ReportsPage = lazy(() => import('@/pages/ReportsPage'));
const SyslogPage = lazy(() => import('@/pages/SyslogPage'));
const InterceptPage = lazy(() => import('@/pages/InterceptPage'));
const SettingsPage = lazy(() => import('@/pages/SettingsPage'));
const GeoPage = lazy(() => import('@/pages/GeoPage'));
const InterfacesPage = lazy(() => import('@/pages/InterfacesPage'));
const HostPoolsPage = lazy(() => import('@/pages/HostPoolsPage'));
const ServiceMapPage = lazy(() => import('@/pages/ServiceMapPage'));
const LoginPage = lazy(() => import('@/pages/LoginPage'));
const SetupPage = lazy(() => import('@/pages/SetupPage'));

const BASENAME = import.meta.env.BASE_URL.replace(/\/$/, '');

function PageLoader() {
  return (
    <div className="flex items-center justify-center py-16">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
    </div>
  );
}

function SetupGuard() {
  const { isAuthenticated } = useAuth();
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);

  useEffect(() => {
    checkSetup().then(s => setNeedsSetup(s.setup_required)).catch(() => setNeedsSetup(false));
  }, []);

  if (needsSetup === null) return <PageLoader />;
  if (needsSetup) return <Navigate to="/setup" replace />;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <Outlet />;
}

export default function App() {
  return (
    <I18nProvider>
    <AuthContextProvider>
    <AppContextProvider>
      <BrowserRouter basename={BASENAME}>
      <Toaster richColors position="bottom-right" closeButton visibleToasts={3} />
        <Suspense fallback={<PageLoader />}>
          <Routes>
            <Route path="/setup" element={<SetupPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route element={<SetupGuard />}>
              <Route element={<AppLayout />}>
                <Route path="/" element={<DashboardPage />} />
                <Route path="/hosts" element={<HostsPage />} />
                <Route path="/hosts/:ip" element={<HostDetailPage />} />
                <Route path="/flows" element={<FlowsPage />} />
                <Route path="/protocols" element={<ProtocolsPage />} />
                <Route path="/alerts" element={<AlertsPage />} />
                <Route path="/nodes" element={<NodesPage />} />
                <Route path="/dns" element={<DnsPage />} />
                <Route path="/reports" element={<ReportsPage />} />
                <Route path="/syslog" element={<SyslogPage />} />
                <Route path="/intercept" element={<InterceptPage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="/geo" element={<GeoPage />} />
                <Route path="/interfaces" element={<InterfacesPage />} />
                <Route path="/pools" element={<HostPoolsPage />} />
                <Route path="/service-map" element={<ServiceMapPage />} />
              </Route>
            </Route>
          </Routes>
        </Suspense>
      </BrowserRouter>
    </AppContextProvider>
    </AuthContextProvider>
    </I18nProvider>
  );
}
