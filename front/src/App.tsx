import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import QueryProvider from './providers/QueryProvider';
import Layout from './components/Layout';
import DashboardPage from './pages/dashboard/DashboardPage';
import NetworksPage from './pages/networks/NetworksPage';
import PeersPage from './pages/peers/PeersPage';
import IPAMPage from './pages/ipam/IPAMPage';
import SecurityPage from './pages/security/SecurityPage';
import UsersPage from './pages/users/UsersPage';
import GroupsPage from './pages/groups/GroupsPage';
import PoliciesPage from './pages/policies/PoliciesPage';
import RoutesPage from './pages/routes/RoutesPage';
import LoginPage from './pages/auth/LoginPage';
import CaptivePortalPage from './pages/captive-portal/CaptivePortalPage';

function ProtectedRoutes() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700">
        <div className="text-gray-500 dark:text-gray-400">Loading...</div>
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
        <Route path="/networks" element={<NetworksPage />} />
        <Route path="/peers" element={<PeersPage />} />
        <Route path="/groups" element={<GroupsPage />} />
        <Route path="/policies" element={<PoliciesPage />} />
        <Route path="/routes" element={<RoutesPage />} />
        <Route path="/ipam" element={<IPAMPage />} />
        <Route path="/security" element={<SecurityPage />} />
        <Route path="/users" element={<UsersPage />} />
      </Routes>
    </Layout>
  );
}

function App() {
  return (
    <QueryProvider>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            {/* Captive portal route - accessible without authentication */}
            <Route path="/captive-portal" element={<CaptivePortalPage />} />
            {/* All other routes require authentication */}
            <Route path="/*" element={<ProtectedRoutes />} />
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryProvider>
  );
}

export default App;
