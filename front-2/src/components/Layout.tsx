import { Link, useLocation } from 'react-router-dom';
import { useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { 
  faChartLine, 
  faNetworkWired, 
  faServer, 
  faMapMarkerAlt, 
  faShieldAlt, 
  faUsers,
  faSun,
  faMoon,
  faDesktop,
  faUserCircle
} from '@fortawesome/free-solid-svg-icons';
import { useTheme } from '../contexts/ThemeContext';
import { useAuth } from '../contexts/AuthContext';
import ProfileModal from './ProfileModal';
import type { IconDefinition } from '@fortawesome/fontawesome-svg-core';

const navigation: { name: string; href: string; icon: IconDefinition }[] = [
  { name: 'Dashboard', href: '/dashboard', icon: faChartLine },
  { name: 'Networks', href: '/networks', icon: faNetworkWired },
  { name: 'Peers', href: '/peers', icon: faServer },
  { name: 'IPAM', href: '/ipam', icon: faMapMarkerAlt },
  { name: 'Security', href: '/security', icon: faShieldAlt },
  { name: 'Users', href: '/users', icon: faUsers },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { theme, setTheme } = useTheme();
  const { user } = useAuth();
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false);

  const isAdmin = user?.role === 'administrator';

  // Filter navigation items based on user role
  const visibleNavigation = navigation.filter(item => {
    if (item.href === '/users' && !isAdmin) {
      return false;
    }
    return true;
  });

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      {/* Sidebar */}
      <div className="fixed inset-y-0 left-0 w-64 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700">
        <div className="flex flex-col h-full">
          {/* Logo */}
          <div className="flex items-center h-16 px-6 border-b border-gray-200 dark:border-gray-700">
            <div className="flex items-center gap-3">
              <img src="/logo.svg" alt="Wirety Logo" className="w-10 h-10 rounded-lg" />
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Wirety</h1>
            </div>
          </div>

          {/* Navigation */}
          <nav className="flex-1 px-3 py-4 space-y-1">
            {visibleNavigation.map((item) => {
              const isActive = location.pathname.startsWith(item.href);
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  className={`flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
                    isActive
                      ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-700 dark:text-primary-400'
                      : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                  }`}
                >
                  <FontAwesomeIcon icon={item.icon} className="mr-3 text-lg" />
                  {item.name}
                </Link>
              );
            })}
          </nav>

          {/* Footer */}
          <div className="p-4 border-t border-gray-200 dark:border-gray-700 space-y-3">
            {/* User Profile Button */}
            {user && (
              <button
                onClick={() => setIsProfileModalOpen(true)}
                className="w-full flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-colors text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
              >
                <FontAwesomeIcon icon={faUserCircle} className="mr-3 text-lg" />
                <div className="flex-1 min-w-0 text-left">
                  <div className="text-sm font-medium truncate">{user.name}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400 truncate">{user.email}</div>
                </div>
              </button>
            )}

            {/* Theme Segmented Button */}
            <div className="flex flex-col gap-2">
              <div className="flex bg-gray-100 dark:bg-gray-700 rounded-lg p-1">
                <button
                  onClick={() => setTheme('light')}
                  className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    theme === 'light'
                      ? 'bg-white dark:bg-gray-800 text-primary-600 dark:text-primary-400 shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
                  }`}
                  title="Light mode"
                >
                  <FontAwesomeIcon icon={faSun} />
                </button>
                <button
                  onClick={() => setTheme('dark')}
                  className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    theme === 'dark'
                      ? 'bg-white dark:bg-gray-800 text-primary-600 dark:text-primary-400 shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
                  }`}
                  title="Dark mode"
                >
                  <FontAwesomeIcon icon={faMoon} />
                </button>
                <button
                  onClick={() => setTheme('system')}
                  className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    theme === 'system'
                      ? 'bg-white dark:bg-gray-800 text-primary-600 dark:text-primary-400 shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
                  }`}
                  title="System preference"
                >
                  <FontAwesomeIcon icon={faDesktop} />
                </button>
              </div>
            </div>
            <div className="text-xs text-gray-500 dark:text-gray-400">
              WireGuard Network Management
            </div>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="pl-64">
        <main className="min-h-screen">
          {children}
        </main>
      </div>

      {/* Profile Modal */}
      <ProfileModal isOpen={isProfileModalOpen} onClose={() => setIsProfileModalOpen(false)} />
    </div>
  );
}
