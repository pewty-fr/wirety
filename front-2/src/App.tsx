import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import DashboardPage from './pages/dashboard/DashboardPage';
import NetworksPage from './pages/networks/NetworksPage';
import PeersPage from './pages/peers/PeersPage';
import IPAMPage from './pages/ipam/IPAMPage';
import SecurityPage from './pages/security/SecurityPage';
import UsersPage from './pages/users/UsersPage';

function App() {
  return (
    <BrowserRouter>
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
    </BrowserRouter>
  );
}

export default App;
