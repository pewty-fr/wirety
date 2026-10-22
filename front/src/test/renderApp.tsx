import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { AppProviders, AppRoutes } from '../App';
import { ThemeProvider } from '../contexts/ThemeContext';

/**
 * Renders the whole application at `path`, logged in as the fake API's
 * current user (db.me), with a fresh query cache. The captive-portal alert is
 * dismissed unless `captivePortalAlert` is set.
 */
export function renderApp(path = '/dashboard', { captivePortalAlert = false } = {}) {
  // The captive-portal alert would pop over every page otherwise.
  if (!captivePortalAlert) window.sessionStorage.setItem('wirety.captivePortalAlert.dismissed', '1');
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: Infinity }, mutations: { retry: false } },
  });
  const user = userEvent.setup();
  const utils = render(
    <ThemeProvider>
      <AppProviders queryClient={queryClient}>
        <MemoryRouter initialEntries={[path]}>
          <AppRoutes />
        </MemoryRouter>
      </AppProviders>
    </ThemeProvider>,
  );
  return { user, queryClient, ...utils };
}
