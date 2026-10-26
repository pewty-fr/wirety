import { act, screen, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import type { CaptivePortalEvent, PeerConnectivityStatus } from '../types';
import { server } from './setup';

/** The open dialog with the given title. */
export async function findDialog(title: string | RegExp) {
  return screen.findByRole('dialog', { name: title });
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/**
 * Picks `option` in the combobox labelled `label` (inside `container`). A
 * string option matches the start of the option's name (options show a
 * description after their label).
 */
export async function pick(user: UserEvent, container: HTMLElement, label: string | RegExp, option: string | RegExp) {
  const input = within(container).getByRole('combobox', { name: label });
  await user.click(input);
  const name = typeof option === 'string' ? new RegExp(`^${escape(option)}`) : option;
  await user.click(await screen.findByRole('option', { name }));
  // Headless UI gives the focus back to the input on the next frame: let it
  // happen, or it reopens the options over the next action.
  await act(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));
}

/** Opens the tab named `name` (a name prefix: tabs show counts). */
export async function openTab(user: UserEvent, container: HTMLElement, name: string) {
  await user.click(within(container).getByRole('tab', { name: new RegExp(`^${name}`) }));
}

/** The connectivity status of each peer (by id); the others are idle. */
export function peerSessions(byPeer: Record<string, Partial<PeerConnectivityStatus>>) {
  server.use(
    http.get('*/api/v1/networks/:id/peers/:peerId/session', ({ params }) =>
      HttpResponse.json({
        peer_id: params.peerId,
        has_active_agent: false,
        last_checked: '2026-10-01T10:00:00Z',
        captive_portal_state: '',
        ...byPeer[String(params.peerId)],
      }),
    ),
  );
}

/** The captive-portal history (newest first) of each peer (by id); the others have none. */
export function peerEvents(byPeer: Record<string, CaptivePortalEvent[]>) {
  server.use(
    http.get('*/api/v1/networks/:id/peers/:peerId/captive-portal-events', ({ params }) =>
      HttpResponse.json(byPeer[String(params.peerId)] ?? []),
    ),
  );
}

/** An ISO timestamp `minutes` ago. */
export const minutesAgo = (minutes: number) => new Date(Date.now() - minutes * 60 * 1000).toISOString();
