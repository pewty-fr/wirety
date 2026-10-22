import type { ReactNode } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { queryClient as appQueryClient } from './providers/QueryProvider';
import { ConfirmProvider, Spinner, ToastProvider } from './components/ui';
import Layout from './components/Layout';
import DashboardPage from './pages/dashboard/DashboardPage';
import DevicesPage from './pages/devices/DevicesPage';
import NetworksPage from './pages/networks/NetworksPage';
import PeersPage from './pages/peers/PeersPage';
import IPAMPage from './pages/ipam/IPAMPage';
import UsersPage from './pages/users/UsersPage';
import GroupsPage from './pages/groups/GroupsPage';
import PoliciesPage from './pages/policies/PoliciesPage';
import RoutesPage from './pages/routes/RoutesPage';
import DNSRecordsPage from './pages/dns/DNSRecordsPage';
import LoginPage from './pages/auth/LoginPage';
import CaptivePortalPage from './pages/captive-portal/CaptivePortalPage';
import ProfilePage from './pages/profile/ProfilePage';

function ProtectedRoutes() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-canvas text-ink-muted" role="status">
        <Spinner className="size-5" />
        <span className="sr-only">Loading…</span>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <LoginPage />;
  }

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/devices" element={<DevicesPage />} />
        <Route path="/networks" element={<NetworksPage />} />
        <Route path="/peers" element={<PeersPage />} />
        <Route path="/groups" element={<GroupsPage />} />
        <Route path="/policies" element={<PoliciesPage />} />
        <Route path="/routes" element={<RoutesPage />} />
        <Route path="/dns" element={<DNSRecordsPage />} />
        <Route path="/ipam" element={<IPAMPage />} />
        <Route path="/users" element={<UsersPage />} />
        <Route path="/profile" element={<ProfilePage />} />
      </Routes>
    </Layout>
  );
}

export function AppRoutes() {
  return (
    <Routes>
      {/* Captive portal route - accessible without authentication */}
      <Route path="/captive-portal" element={<CaptivePortalPage />} />
      {/* All other routes require authentication */}
      <Route path="/*" element={<ProtectedRoutes />} />
    </Routes>
  );
}

/** Data, feedback and auth providers (the router is added by the caller). */
export function AppProviders({ children, queryClient = appQueryClient }: { children: ReactNode; queryClient?: QueryClient }) {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <ConfirmProvider>
          <AuthProvider>{children}</AuthProvider>
        </ConfirmProvider>
      </ToastProvider>
    </QueryClientProvider>
  );
}

function App() {
  return (
    <AppProviders>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </AppProviders>
  );
}

export default App;
