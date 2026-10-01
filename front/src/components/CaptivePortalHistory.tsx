import { usePeerCaptivePortalEvents } from '../hooks/useQueries';
import { describeCaptivePortalEvent, lastSignOut } from '../utils/captivePortalEvents';

interface CaptivePortalHistoryProps {
  networkId: string;
  peerId: string;
  enabled: boolean;
}

/**
 * Why the peer last had to go back through the captive portal, and its recent
 * sign-ins / ends of access.
 */
export default function CaptivePortalHistory({ networkId, peerId, enabled }: CaptivePortalHistoryProps) {
  const { data: events } = usePeerCaptivePortalEvents(networkId, peerId, enabled);
  if (!events || events.length === 0) {
    return null;
  }
  const signOut = lastSignOut(events);

  return (
    <div className="space-y-2">
      {signOut && (
        <div className="flex items-start justify-between gap-4">
          <span className="shrink-0 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">Last sign-out</span>
          <div className="text-right">
            <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
              {describeCaptivePortalEvent(signOut).label}
              <span className="font-normal text-gray-500 dark:text-gray-400"> · {new Date(signOut.created_at).toLocaleString()}</span>
            </div>
            <div className="text-xs text-gray-500 dark:text-gray-400">
              {describeCaptivePortalEvent(signOut).explanation}
              {signOut.detail && <> ({signOut.detail})</>}
            </div>
          </div>
        </div>
      )}
      <details className="text-sm">
        <summary className="cursor-pointer text-gray-600 dark:text-gray-300">Portal history</summary>
        <ul className="mt-2 space-y-1">
          {events.map((e) => (
            <li key={`${e.created_at}-${e.event}`} className="flex justify-between gap-4 text-xs">
              <span className="text-gray-500 dark:text-gray-400 whitespace-nowrap">{new Date(e.created_at).toLocaleString()}</span>
              <span className="text-right text-gray-900 dark:text-gray-100">
                {describeCaptivePortalEvent(e).label}
                {e.detail && <span className="text-gray-500 dark:text-gray-400"> — {e.detail}</span>}
              </span>
            </li>
          ))}
        </ul>
      </details>
    </div>
  );
}
