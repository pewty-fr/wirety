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
  faUsersGear,
  faRoute,
  faSun,
  faMoon,
  faDesktop,
  faUserCircle,
  faBars,
  faTimes
} from '@fortawesome/free-solid-svg-icons';
import { useTheme } from '../contexts/ThemeContext';
import { useAuth } from '../contexts/AuthContext';
import ProfileModal from './ProfileModal';
import FooterBanner from './FooterBanner';
import type { IconDefinition } from '@fortawesome/fontawesome-svg-core';

const navigationSections: { 
  title?: string; 
  items: { name: string; href: string; icon: IconDefinition; adminOnly?: boolean }[] 
}[] = [
  {
    items: [
      { name: 'Dashboard', href: '/dashboard', icon: faChartLine },
    ]
  },
  {
    title: 'Network',
    items: [
      { name: 'Networks', href: '/networks', icon: faNetworkWired },
      { name: 'Peers', href: '/peers', icon: faServer },
      { name: 'IPAM', href: '/ipam', icon: faMapMarkerAlt },
    ]
  },
  {
    title: 'Access Control',
    items: [
      { name: 'Groups', href: '/groups', icon: faUsersGear, adminOnly: true },
      { name: 'Policies', href: '/policies', icon: faShieldAlt, adminOnly: true },
      { name: 'Routes', href: '/routes', icon: faRoute, adminOnly: true },
    ]
  },
  {
    title: 'Administration',
    items: [
      { name: 'Security', href: '/security', icon: faShieldAlt },
      { name: 'Users', href: '/users', icon: faUsers, adminOnly: true },
    ]
  },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { theme, setTheme } = useTheme();
  const { user } = useAuth();
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  const isAdmin = user?.role === 'administrator';

  // Filter navigation sections based on user role
  const visibleNavigationSections = navigationSections.map(section => ({
    ...section,
    items: section.items.filter(item => {
      if (item.adminOnly && !isAdmin) {
        return false;
      }
      return true;
    })
  })).filter(section => section.items.length > 0);

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-dark">
      {/* Mobile menu button */}
      <button
        onClick={() => setIsSidebarOpen(!isSidebarOpen)}
        className="fixed top-4 left-4 z-50 lg:hidden p-2 rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 shadow-lg text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        aria-label="Toggle menu"
      >
        <FontAwesomeIcon icon={isSidebarOpen ? faTimes : faBars} className="text-xl" />
      </button>

      {/* Overlay for mobile */}
      {isSidebarOpen && (
        <div
          className="fixed inset-0 backdrop-blur-sm bg-gradient-to-br from-primary-500/10 to-accent-blue/10 dark:from-black/50 dark:to-primary-900/50 z-30 lg:hidden transition-all"
          onClick={() => setIsSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div className={`fixed inset-y-0 left-0 w-64 bg-gradient-to-br from-white to-gray-100 dark:from-dark dark:to-gray-800 border-r border-gray-200 dark:border-gray-700 z-40 transform transition-transform duration-300 ease-in-out ${
        isSidebarOpen ? 'translate-x-0' : '-translate-x-full'
      } lg:translate-x-0`}>
        <div className="flex flex-col h-full">
          {/* Logo */}
          <div className="flex items-center h-16 px-6 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-primary-500 to-accent-blue dark:from-dark dark:to-primary-700">
            <div className="flex items-center gap-3">
              <img src="/logo.svg" alt="Wirety Logo" className="w-10 h-10 rounded-lg" />
              <h1 className="text-2xl font-bold text-white dark:text-white">Wirety</h1>
            </div>
          </div>

          {/* Navigation */}
          <nav className="flex-1 px-3 py-4 space-y-4 overflow-y-auto">
            {visibleNavigationSections.map((section, sectionIndex) => (
              <div key={sectionIndex}>
                {section.title && (
                  <div className="px-3 mb-2">
                    <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                      {section.title}
                    </h3>
                  </div>
                )}
                <div className="space-y-1">
                  {section.items.map((item) => {
                    const isActive = location.pathname.startsWith(item.href);
                    return (
                      <Link
                        key={item.name}
                        to={item.href}
                        onClick={() => setIsSidebarOpen(false)}
                        className={`flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
                          isActive
                            ? 'bg-gradient-to-r from-primary-500 to-accent-blue text-white dark:from-primary-600 dark:to-accent-blue dark:text-white'
                            : 'text-gray-700 dark:text-gray-300 hover:bg-gradient-to-r hover:from-primary-100 hover:to-accent-blue/10 dark:hover:from-gray-700 dark:hover:to-primary-900'
                        }`}
                      >
                        <FontAwesomeIcon icon={item.icon} className="mr-3 text-lg" />
                        {item.name}
                      </Link>
                    );
                  })}
                </div>
              </div>
            ))}
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
      <div className="lg:pl-64">
        <main className="min-h-screen pb-16">
          {children}
        </main>
      </div>

      {/* Footer Banner */}
      <FooterBanner />

      {/* Profile Modal */}
      <ProfileModal isOpen={isProfileModalOpen} onClose={() => setIsProfileModalOpen(false)} />
    </div>
  );
}
