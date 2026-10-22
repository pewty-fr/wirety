import { useState } from 'react';
import { MagnifyingGlassIcon } from '@heroicons/react/20/solid';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { TableCellsIcon } from '@heroicons/react/24/outline';
import api from '../../api/client';
import {
  Alert,
  Badge,
  EmptyState,
  Page,
  PageHeader,
  Pagination,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
  TextInput,
  apiErrorMessage,
} from '../../components/ui';
import { useDebounce } from '../../hooks/useDebounce';
import type { IPAMAllocation } from '../../types';

const PAGE_SIZE = 50;

// Older servers (before dual-stack IPAM) don't send the family: infer it.
const familyOf = (a: IPAMAllocation) => a.family || (a.ip.includes(':') ? 'ipv6' : 'ipv4');

/** Every address of every network, allocated to a peer or free. */
export default function IPAMPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const filter = useDebounce(search.trim(), 300);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['ipam', page, PAGE_SIZE, filter],
    queryFn: () => api.getIPAMAllocations(page, PAGE_SIZE, filter || undefined),
    placeholderData: keepPreviousData,
  });
  const allocations = data?.data ?? [];
  const total = data?.total ?? 0;

  return (
    <Page>
      <PageHeader
        title="IP address management"
        description={
          data
            ? `${total.toLocaleString()} address${total === 1 ? '' : 'es'} tracked across your networks.`
            : 'The addresses of your networks and the peers they are allocated to.'
        }
      />

      <TextInput
        label="Search"
        type="search"
        value={search}
        onChange={(e) => {
          setSearch(e.target.value);
          setPage(1);
        }}
        placeholder="Network, IP or peer…"
        leading={<MagnifyingGlassIcon />}
        className="w-full sm:w-72"
        autoComplete="off"
      />

      {isError ? (
        <Alert tone="danger" title="The addresses could not be loaded">
          {apiErrorMessage(error)}
        </Alert>
      ) : isLoading ? (
        <div className="flex justify-center py-16 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading addresses…</span>
        </div>
      ) : allocations.length === 0 ? (
        <EmptyState
          icon={TableCellsIcon}
          title={filter ? `No address matches “${filter}”` : 'No address yet'}
          description={
            filter ? 'Try another network, IP or peer name.' : 'The addresses of your networks appear here once they have peers.'
          }
        />
      ) : (
        <>
          <Table>
            <THead>
              <tr>
                <TH>IP address</TH>
                <TH>Family</TH>
                <TH>Network</TH>
                <TH>Peer</TH>
                <TH>Status</TH>
              </tr>
            </THead>
            <TBody>
              {allocations.map((a, i) => (
                <TR key={`${a.network_id}-${a.ip}-${i}`}>
                  <TD mono>
                    <span className="font-medium text-ink">{a.ip}</span>
                  </TD>
                  <TD>
                    {familyOf(a) === 'ipv6' ? <Badge tone="info">IPv6</Badge> : <Badge tone="neutral">IPv4</Badge>}
                  </TD>
                  <TD>
                    <div className="text-ink">{a.network_name}</div>
                    <div className="font-mono text-xs text-ink-muted">{a.network_cidr}</div>
                  </TD>
                  <TD>{a.peer_name || <span className="text-ink-subtle">—</span>}</TD>
                  <TD>
                    {a.allocated ? (
                      <Badge tone="success" dot>
                        Allocated
                      </Badge>
                    ) : (
                      <Badge tone="neutral">Free</Badge>
                    )}
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
          <Pagination page={page} pageSize={PAGE_SIZE} total={total} onPageChange={setPage} />
        </>
      )}
    </Page>
  );
}
