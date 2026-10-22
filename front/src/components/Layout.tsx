import { Link, useLocation } from 'react-router-dom';
import { useState } from 'react';
import type { ComponentType, SVGProps } from 'react';
import { Dialog, DialogBackdrop, DialogPanel } from '@headlessui/react';
import {
  ArrowsRightLeftIcon,
  Bars3Icon,
  ComputerDesktopIcon,
  DevicePhoneMobileIcon,
  GlobeAltIcon,
  HashtagIcon,
  MoonIcon,
  ServerStackIcon,
  ShieldCheckIcon,
  Squares2X2Icon,
  SunIcon,
  UserGroupIcon,
  UsersIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { useTheme } from '../contexts/ThemeContext';
import { useAuth } from '../contexts/AuthContext';
import ProfileModal from './ProfileModal';
import CaptivePortalAlertModal from './CaptivePortalAlertModal';
import { useCaptivePortalCheck } from '../hooks/useCaptivePortalCheck';
import { cx } from './ui';

type Icon = ComponentType<SVGProps<SVGSVGElement>>;

const navigationSections: { title?: string; items: { name: string; href: string; icon: Icon; adminOnly?: boolean }[] }[] = [
  {
    items: [
      { name: 'Dashboard', href: '/dashboard', icon: Squares2X2Icon },
      { name: 'My devices', href: '/devices', icon: DevicePhoneMobileIcon },
    ],
  },
  {
    title: 'Network',
    items: [
      { name: 'Networks', href: '/networks', icon: ServerStackIcon },
      { name: 'Peers', href: '/peers', icon: ComputerDesktopIcon },
      { name: 'IPAM', href: '/ipam', icon: HashtagIcon },
    ],
  },
  {
    title: 'Access control',
    items: [
      { name: 'Groups', href: '/groups', icon: UserGroupIcon, adminOnly: true },
      { name: 'Policies', href: '/policies', icon: ShieldCheckIcon, adminOnly: true },
      { name: 'Routes', href: '/routes', icon: ArrowsRightLeftIcon, adminOnly: true },
      { name: 'DNS records', href: '/dns', icon: GlobeAltIcon, adminOnly: true },
    ],
  },
  { title: 'Administration', items: [{ name: 'Users', href: '/users', icon: UsersIcon, adminOnly: true }] },
];

function initials(name: string | undefined) {
  return (name ?? '?')
    .split(/\s+/)
    .map((w) => w[0])
    .join('')
    .slice(0, 2)
    .toUpperCase();
}

function Sidebar({ onNavigate, onOpenProfile }: { onNavigate?: () => void; onOpenProfile: () => void }) {
  const location = useLocation();
  const { theme, setTheme } = useTheme();
  const { user } = useAuth();
  const isAdmin = user?.role === 'administrator';
  // Devices waiting for their owner: shown next to "My devices".
  const attention = useCaptivePortalCheck().reduce((n, net) => n + net.affectedPeers.length, 0);

  const sections = navigationSections
    .map((s) => ({ ...s, items: s.items.filter((i) => !i.adminOnly || isAdmin) }))
    .filter((s) => s.items.length > 0);

  const themes = [
    { value: 'light' as const, label: 'Light theme', icon: SunIcon },
    { value: 'dark' as const, label: 'Dark theme', icon: MoonIcon },
    { value: 'system' as const, label: 'System theme', icon: ComputerDesktopIcon },
  ];

  return (
    <div className="flex h-full flex-col bg-nav text-nav-ink">
      <div className="flex h-14 shrink-0 items-center gap-2.5 px-4">
        <img src="/logo.svg" alt="" className="size-7 rounded-md" />
        <span className="text-[15px] font-semibold tracking-tight text-nav-ink-active">Wirety</span>
      </div>

      <nav aria-label="Main" className="flex-1 space-y-5 overflow-y-auto px-3 py-3">
        {sections.map((section, i) => (
          <div key={section.title ?? i}>
            {section.title && (
              <p className="mb-1.5 px-2 text-[11px] font-medium tracking-wider text-nav-ink/60 uppercase">{section.title}</p>
            )}
            <ul className="space-y-0.5">
              {section.items.map((item) => {
                const active = location.pathname.startsWith(item.href);
                return (
                  <li key={item.href}>
                    <Link
                      to={item.href}
                      onClick={onNavigate}
                      aria-current={active ? 'page' : undefined}
                      className={cx(
                        'group relative flex h-8 items-center gap-2.5 rounded-md px-2 text-[13px] font-medium transition-colors',
                        'focus-visible:outline-2 focus-visible:outline-primary-500',
                        active ? 'bg-white/[0.08] text-nav-ink-active' : 'hover:bg-white/[0.04] hover:text-nav-ink-active',
                      )}
                    >
                      {active && <span className="absolute inset-y-1.5 -left-3 w-[3px] rounded-r bg-primary-500" aria-hidden="true" />}
                      <item.icon
                        className={cx('size-[18px] shrink-0', active ? 'text-primary-400' : 'text-nav-ink/70 group-hover:text-nav-ink-active')}
                        aria-hidden="true"
                      />
                      {item.name}
                      {item.href === '/devices' && attention > 0 && (
                        <>
                          <span className="sr-only">
                            , {attention} {attention === 1 ? 'device needs' : 'devices need'} your attention
                          </span>
                          <span
                            aria-hidden="true"
                            className="ml-auto rounded-full bg-amber-400/15 px-1.5 text-[11px] font-semibold text-amber-300 tabular-nums"
                          >
                            {attention}
                          </span>
                        </>
                      )}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>

      <div className="space-y-3 border-t border-nav-line p-3">
        {user && (
          <button
            type="button"
            onClick={onOpenProfile}
            className="flex w-full items-center gap-2.5 rounded-md p-1.5 text-left transition-colors hover:bg-white/[0.04] focus-visible:outline-2 focus-visible:outline-primary-500"
          >
            <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary-500/15 text-xs font-semibold text-primary-300">
              {initials(user.name)}
            </span>
            <span className="min-w-0">
              <span className="block truncate text-[13px] font-medium text-nav-ink-active">{user.name}</span>
              <span className="block truncate text-xs text-nav-ink/70">{user.email}</span>
            </span>
          </button>
        )}
        <div className="flex items-center justify-between gap-2 px-1.5">
          <div role="group" aria-label="Theme" className="flex rounded-md bg-white/[0.04] p-0.5">
            {themes.map((t) => (
              <button
                key={t.value}
                type="button"
                onClick={() => setTheme(t.value)}
                aria-label={t.label}
                aria-pressed={theme === t.value}
                title={t.label}
                className={cx(
                  'flex size-7 items-center justify-center rounded transition-colors focus-visible:outline-2 focus-visible:outline-primary-500',
                  theme === t.value ? 'bg-white/[0.1] text-nav-ink-active' : 'text-nav-ink/70 hover:text-nav-ink-active',
                )}
              >
                <t.icon className="size-4" aria-hidden="true" />
              </button>
            ))}
          </div>
          <p className="text-[11px] text-nav-ink/60">
            by{' '}
            <a href="https://pewty.fr" target="_blank" rel="noopener noreferrer" className="hover:text-nav-ink-active">
              Pewty
            </a>
            {' · '}
            <a href="https://github.com/pewty-fr/wirety" target="_blank" rel="noopener noreferrer" className="hover:text-nav-ink-active">
              GitHub
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}

export default function Layout({ children }: { children: React.ReactNode }) {
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  return (
    <div className="min-h-screen bg-canvas">
      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 border-r border-nav-line lg:block">
        <Sidebar onOpenProfile={() => setIsProfileModalOpen(true)} />
      </aside>

      {/* Mobile top bar and drawer */}
      <div className="sticky top-0 z-20 flex h-14 items-center gap-3 border-b border-line bg-surface/90 px-4 backdrop-blur lg:hidden">
        <button
          type="button"
          onClick={() => setIsSidebarOpen(true)}
          aria-label="Open menu"
          className="-ml-1.5 rounded-md p-1.5 text-ink-secondary hover:bg-surface-hover"
        >
          <Bars3Icon className="size-6" />
        </button>
        <img src="/logo.svg" alt="" className="size-6 rounded" />
        <span className="font-semibold text-ink">Wirety</span>
      </div>
      <Dialog open={isSidebarOpen} onClose={setIsSidebarOpen} className="relative z-40 lg:hidden">
        <DialogBackdrop transition className="fixed inset-0 bg-gray-950/50 transition duration-200 data-closed:opacity-0" />
        <DialogPanel
          transition
          className="fixed inset-y-0 left-0 w-64 transition duration-200 ease-out data-closed:-translate-x-full"
        >
          <button
            type="button"
            onClick={() => setIsSidebarOpen(false)}
            aria-label="Close menu"
            className="absolute top-3 -right-11 rounded-md p-1.5 text-white"
          >
            <XMarkIcon className="size-6" />
          </button>
          <Sidebar
            onNavigate={() => setIsSidebarOpen(false)}
            onOpenProfile={() => {
              setIsSidebarOpen(false);
              setIsProfileModalOpen(true);
            }}
          />
        </DialogPanel>
      </Dialog>

      <main className="min-h-screen lg:pl-60">{children}</main>

      <ProfileModal isOpen={isProfileModalOpen} onClose={() => setIsProfileModalOpen(false)} />

      {/* Captive Portal Alert — auto-shows when any of the user's devices need
          to authenticate.  See CaptivePortalAlertModal for detection logic. */}
      <CaptivePortalAlertModal />
    </div>
  );
}
