import type { CaptivePortalEvent, CaptivePortalEventType } from '../types';

interface EventDescription {
  label: string;
  // What happened, in words the device's user understands.
  explanation: string;
}

const descriptions: Record<CaptivePortalEventType, EventDescription> = {
  authenticated: {
    label: 'Signed in',
    explanation: 'Signed in through the captive portal.',
  },
  expired: {
    label: 'Session expired',
    explanation: 'The captive-portal session duration was reached.',
  },
  tunnel_inactive: {
    label: 'Tunnel disconnected',
    explanation: 'The VPN tunnel was inactive for more than 3 minutes (device asleep, network lost or VPN turned off).',
  },
  endpoint_changed: {
    label: 'Network changed',
    explanation: "The device's public IP changed (another Wi-Fi, mobile data…): signing in again from the new network is required.",
  },
  revoked: {
    label: 'Revoked',
    explanation: 'The authentication was reset from the dashboard.',
  },
};

export function describeCaptivePortalEvent(event: CaptivePortalEvent): EventDescription {
  return descriptions[event.event] ?? { label: event.event, explanation: '' };
}

/** The most recent end of access in a newest-first history, if any. */
export function lastSignOut(events: CaptivePortalEvent[] | undefined): CaptivePortalEvent | undefined {
  return events?.find((e) => e.event !== 'authenticated');
}
