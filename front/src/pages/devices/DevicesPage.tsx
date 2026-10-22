import { useEffect, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ArrowPathIcon, ArrowTopRightOnSquareIcon, ComputerDesktopIcon, PlusIcon, ServerIcon } from '@heroicons/react/20/solid';
import { DevicePhoneMobileIcon } from '@heroicons/react/24/outline';
import api from '../../api/client';
import {
  Badge,
  Button,
  Card,
  CardHeader,
  EmptyState,
  Page,
  PageHeader,
  Spinner,
  apiErrorMessage,
  buttonClass,
  cx,
  useConfirm,
  useToast,
} from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { useNetworks, usePeerCaptivePortalEvents, usePeers } from '../../hooks/useQueries';
import type { Network, Peer } from '../../types';
import { describeCaptivePortalEvent, lastSignOut } from '../../utils/captivePortalEvents';
import { timeAgo } from '../../utils/dates';
import { captivePortalURL, isSuspicious, needsAttention } from '../../utils/devices';
import PeerDetailModal from '../peers/PeerDetailModal';
import { peerStatus, refreshPeers } from '../peers/peerStatus';
import RegularPeerModal from '../peers/RegularPeerModal';

interface NetworkDevices {
  network: Network;
  portalURL: string | null;
  devices: Peer[];
}

/** The sign-in of the device: since when, or why and when it ended. */
function SignInHistory({ device }: { device: Peer }) {
  const { data: events } = usePeerCaptivePortalEvents(device.network_id ?? '', device.id);
  const latest = events?.[0];
  if (!latest) return null;
  if (device.session_status?.captive_portal_state === 'authenticated' && latest.event === 'authenticated') {
    return <p className="text-xs text-ink-muted">Signed in {timeAgo(latest.created_at)}</p>;
  }
  const signOut = lastSignOut(events);
  if (!signOut) return null;
  const { label, explanation } = describeCaptivePortalEvent(signOut);
  return (
    <p className="text-xs text-ink-muted">
      <span className="font-medium text-ink-secondary">{label}</span> {timeAgo(signOut.created_at)} — {explanation}
    </p>
  );
}

function DeviceRow({
  device,
  now,
  canSignIn,
  resetting,
  onReset,
  onOpen,
}: {
  device: Peer;
  now: number;
  canSignIn: boolean;
  resetting: boolean;
  onReset: () => void;
  onOpen: () => void;
}) {
  const status = peerStatus(device, now);
  const suspicious = isSuspicious(device, now);
  const quarantined = device.session_status?.captive_portal_state === 'quarantined';
  const session = device.session_status?.current_session;
  const Icon = device.use_agent ? ServerIcon : ComputerDesktopIcon;

  return (
    <li aria-label={device.name} className={cx('flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3.5', suspicious && 'bg-red-50/60 dark:bg-red-500/5')}>
      <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-surface-muted text-ink-muted">
        <Icon className="size-4" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="font-medium text-ink">{device.name}</span>
          <Badge tone={status.tone} dot className={cx(status.pending && '[&>span]:animate-pulse')}>
            {status.label}
          </Badge>
          {suspicious && (
            <Badge tone="danger" dot>
              Sign-in requested from an unknown source
            </Badge>
          )}
        </div>
        <p className="text-xs text-ink-muted">
          <span className="font-mono text-ink-secondary">{device.address || device.address_v6}</span>
          {session?.hostname && <> · {session.hostname}</>}
          {timeAgo(session?.last_seen) && <> · seen {timeAgo(session?.last_seen)}</>}
        </p>
        {quarantined ? (
          <p className="text-xs text-red-700 dark:text-red-400">Blocked after too many failed sign-ins: ask an administrator to reset it.</p>
        ) : (
          <SignInHistory device={device} />
        )}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {suspicious ? (
          <Button variant="danger" size="sm" icon={<ArrowPathIcon className="size-4" />} loading={resetting} onClick={onReset}>
            Reset sign-in
          </Button>
        ) : (
          canSignIn &&
          needsAttention(device, now) &&
          !quarantined && <span className="text-xs font-medium text-amber-700 dark:text-amber-400">Sign-in required</span>
        )}
        <Button variant="secondary" size="sm" onClick={onOpen}>
          Details
        </Button>
      </div>
    </li>
  );
}

export default function DevicesPage() {
  const { user } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();
  const queryClient = useQueryClient();
  const { data: networks = [], isLoading: networksLoading } = useNetworks();
  // The same query (and 10 s polling) as the captive-portal alert: a sign-in
  // shows up here within seconds.
  const { data: peersData, isLoading: peersLoading } = usePeers(1, 1000, 10000);
  const [creating, setCreating] = useState(false);
  const [detail, setDetail] = useState<{ open: boolean; peer: Peer | null }>({ open: false, peer: null });
  const [resettingId, setResettingId] = useState<string | null>(null);

  // Statuses depend on the age of the last handshake.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 10 * 1000);
    return () => clearInterval(id);
  }, []);

  const byNetwork = useMemo<NetworkDevices[]>(() => {
    const peers = peersData?.peers ?? [];
    return networks
      .map((network) => ({
        network,
        portalURL: captivePortalURL(peers.find((p) => p.network_id === network.id && p.is_jump)),
        devices: peers.filter((p) => p.network_id === network.id && !p.is_jump && !!user && p.owner_id === user.id),
      }))
      .filter((n) => n.devices.length > 0);
  }, [networks, peersData, user]);

  const resetSignIn = async (device: Peer) => {
    const ok = await confirm({
      title: `Reset the sign-in of “${device.name}”?`,
      message: (
        <div className="space-y-2">
          <p>
            Someone asked to sign in as this device while you are not using it: most likely a copy of its WireGuard configuration is in
            use elsewhere.
          </p>
          <p>
            Resetting cancels that sign-in and removes the device from the network’s allowed list: whoever uses the copy loses access. You
            sign in again the next time you use the device.
          </p>
        </div>
      ),
      confirmLabel: 'Reset sign-in',
      destructive: true,
    });
    if (!ok) return;
    setResettingId(device.id);
    try {
      await api.revokePeerAuthentication(device.network_id!, device.id);
      toast.success(`Sign-in of “${device.name}” reset`);
      refreshPeers(queryClient);
      void queryClient.invalidateQueries({ queryKey: ['peerCaptivePortalEvents'] });
    } catch (err) {
      toast.error(apiErrorMessage(err));
    } finally {
      setResettingId(null);
    }
  };

  const loading = networksLoading || peersLoading;
  const newDevice = (
    <Button icon={<PlusIcon className="size-4" />} disabled={networks.length === 0} onClick={() => setCreating(true)}>
      New device
    </Button>
  );

  return (
    <Page>
      <PageHeader
        title="My devices"
        description="Your devices on each network, and the sign-in each network asks for before letting them through."
        actions={!loading && byNetwork.length > 0 ? newDevice : undefined}
      />

      {loading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading your devices…</span>
        </div>
      ) : byNetwork.length === 0 ? (
        <EmptyState
          icon={DevicePhoneMobileIcon}
          title="No device yet"
          description={
            networks.length === 0
              ? 'You have access to no network yet. Ask an administrator to give you access to one.'
              : 'Add the computers and phones you connect with: each one gets its own WireGuard configuration.'
          }
          action={networks.length > 0 && newDevice}
        />
      ) : (
        byNetwork.map(({ network, portalURL, devices }) => {
          const signInNeeded = devices.some((d) => needsAttention(d, now) && !isSuspicious(d, now));
          return (
            <section key={network.id} aria-label={network.name}>
              <Card>
                <CardHeader
                  title={network.name}
                  description={<span className="font-mono text-xs">{[network.cidr, network.cidr_v6].filter(Boolean).join(' · ')}</span>}
                  actions={
                    portalURL && (
                      <a
                        href={portalURL}
                        target="_blank"
                        rel="noreferrer noopener"
                        className={buttonClass({ size: 'sm', variant: signInNeeded ? 'primary' : 'secondary' })}
                      >
                        <ArrowTopRightOnSquareIcon className="size-4" aria-hidden="true" />
                        Sign in
                      </a>
                    )
                  }
                />
                <ul className="divide-y divide-line">
                  {devices.map((d) => (
                    <DeviceRow
                      key={d.id}
                      device={d}
                      now={now}
                      canSignIn={Boolean(portalURL)}
                      resetting={resettingId === d.id}
                      onReset={() => void resetSignIn(d)}
                      onOpen={() => setDetail({ open: true, peer: d })}
                    />
                  ))}
                </ul>
                <p className="border-t border-line px-5 py-3 text-xs leading-relaxed text-ink-muted">
                  {portalURL ? (
                    <>
                      <span className="font-medium text-ink-secondary">Sign in</span> opens the network’s captive portal,{' '}
                      <span className="font-mono">{portalURL}</span>, in a new tab. It only answers through the “{network.name}” WireGuard tunnel:
                      connect it on the device first, then sign in from that device.
                    </>
                  ) : (
                    'This network has no jump peer yet, so no captive portal to sign in to.'
                  )}
                </p>
              </Card>
            </section>
          );
        })
      )}

      <RegularPeerModal isOpen={creating} onClose={() => setCreating(false)} peer={null} networks={networks} />
      <PeerDetailModal
        isOpen={detail.open}
        onClose={() => setDetail((d) => ({ ...d, open: false }))}
        peer={detail.peer}
        networks={networks}
      />
    </Page>
  );
}
