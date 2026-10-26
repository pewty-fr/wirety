import type { ReactNode } from 'react';
import { PencilSquareIcon, TrashIcon } from '@heroicons/react/20/solid';
import { Button, Modal, ModalBody, ModalFooter, cx } from '../../components/ui';
import { useNetwork } from '../../hooks/useQueries';
import type { Network } from '../../types';
import { getNetworkDomain } from '../../types';
import { computeCapacityFromCIDR, formatCapacity } from '../../utils/networkCapacity';

function Detail({ label, mono, className, children }: { label: string; mono?: boolean; className?: string; children: ReactNode }) {
  return (
    <div className={className}>
      <dt className="text-xs font-medium text-ink-muted">{label}</dt>
      <dd className={cx('mt-1 text-sm text-ink', mono && 'font-mono text-[13px] break-all')}>{children}</dd>
    </div>
  );
}

function Stat({ label, value, className }: { label: string; value: ReactNode; className?: string }) {
  return (
    <div>
      <dt className="text-xs text-ink-muted">{label}</dt>
      <dd className={cx('mt-0.5 text-lg font-semibold text-ink tabular-nums', className)}>{value}</dd>
    </div>
  );
}

const formatDate = (value: string) => new Date(value).toLocaleString();

/**
 * A network's details and IPv4 capacity. Administrators can edit or delete it
 * from here. The network is read again from the query cache, so it stays
 * fresh after an edit.
 */
export default function NetworkDetailModal({
  isOpen,
  onClose,
  network: listed,
  canManage,
  onEdit,
  onDelete,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The network as listed (shown until the fresh copy is loaded). */
  network: Network | null;
  /** Shows the Edit and Delete actions (administrators). */
  canManage: boolean;
  onEdit: (network: Network) => void;
  onDelete: (network: Network) => void;
}) {
  const { data: fetched } = useNetwork(listed?.id ?? '', isOpen);
  if (!listed) return null;

  // The single-network endpoint may not count the peers: keep the list's count.
  const network: Network =
    fetched && fetched.id === listed.id ? { ...listed, ...fetched, peer_count: fetched.peer_count ?? listed.peer_count } : listed;

  const capacity = computeCapacityFromCIDR(network.cidr ?? '');
  const used = network.peer_count ?? 0;
  const free = capacity === null ? null : Math.max(0, capacity - used);
  const utilization = capacity ? Math.min(100, Math.round((used / capacity) * 100)) : 0;
  const barColor = utilization >= 95 ? 'bg-red-500' : utilization >= 80 ? 'bg-amber-500' : 'bg-primary-500';

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={network.name} description="Network details" size="lg">
      <ModalBody>
        <dl className="grid gap-x-6 gap-y-4 sm:grid-cols-2">
          <Detail label="IPv4 CIDR" mono>
            {network.cidr || '—'}
          </Detail>
          <Detail label="IPv6 CIDR" mono>
            {network.cidr_v6 || '—'}
          </Detail>
          <Detail label="Domain" mono>
            {getNetworkDomain({ ...network, domain_suffix: network.domain_suffix || 'internal' })}
          </Detail>
          <Detail label="Domain suffix" mono>
            {network.domain_suffix || 'internal'}
          </Detail>
          <Detail label="DNS servers" mono className="sm:col-span-2">
            {network.dns?.length ? network.dns.join(', ') : <span className="font-sans text-ink-muted">None</span>}
          </Detail>
        </dl>

        <section aria-labelledby="network-capacity" className="rounded-control border border-line bg-surface-muted/50 p-4">
          <h3 id="network-capacity" className="text-sm font-medium text-ink">
            IPv4 capacity
          </h3>
          {capacity === null ? (
            <p className="mt-1 text-sm text-ink-muted">
              No IPv4 range: {used} peer{used === 1 ? '' : 's'}, addressed in IPv6 only.
            </p>
          ) : (
            <>
              <dl className="mt-3 grid grid-cols-2 gap-4 sm:grid-cols-4">
                <Stat label="Addresses" value={formatCapacity(capacity)} />
                <Stat label="Used" value={used.toLocaleString()} />
                <Stat label="Free" value={formatCapacity(free)} />
                <Stat label="Utilization" value={`${utilization}%`} />
              </dl>
              <div
                role="progressbar"
                aria-label="IPv4 utilization"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={utilization}
                className="mt-4 h-1.5 overflow-hidden rounded-full bg-line"
              >
                <div className={cx('h-full rounded-full', barColor)} style={{ width: `${utilization}%` }} />
              </div>
            </>
          )}
        </section>

        <dl className="grid gap-x-6 gap-y-4 sm:grid-cols-3">
          <Detail label="ID" mono>
            {network.id}
          </Detail>
          <Detail label="Created">{formatDate(network.created_at)}</Detail>
          <Detail label="Last updated">{formatDate(network.updated_at)}</Detail>
        </dl>
      </ModalBody>
      {canManage && (
        <ModalFooter>
          <div className="mr-auto">
            <Button
              variant="secondary"
              icon={<TrashIcon className="size-4 text-red-600 dark:text-red-400" />}
              onClick={() => onDelete(network)}
            >
              Delete network
            </Button>
          </div>
          <Button icon={<PencilSquareIcon className="size-4" />} onClick={() => onEdit(network)}>
            Edit network
          </Button>
        </ModalFooter>
      )}
    </Modal>
  );
}
