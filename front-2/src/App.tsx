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
import LoginPage from './pages/auth/LoginPage';

function ProtectedRoutes() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
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
          <ProtectedRoutes />
        </BrowserRouter>
      </AuthProvider>
    </QueryProvider>
  );
}

export default App;
