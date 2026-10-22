import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { Combobox, FormModal, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import type { DNSMapping, Network, Route } from '../../types';
import { isIPv4, isIPv6 } from '../../utils/validation';
import { isValidRecordName, recordFQDN } from './dnsNames';

interface RecordForm {
  routeId: string;
  name: string;
  ip: string;
  ipV6: string;
}

type FieldErrors = Partial<Record<keyof RecordForm, string>>;

function formFrom(record: DNSMapping | null): RecordForm {
  return {
    routeId: record?.route_id ?? '',
    name: record?.name ?? '',
    ip: record?.ip_address ?? '',
    ipV6: record?.ip_address_v6 ?? '',
  };
}

/** Creates or edits a DNS record of a network: its route, name and addresses. */
export default function DNSRecordModal({
  isOpen,
  onClose,
  record,
  network,
  routes,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The record to edit, or null to create one. */
  record: DNSMapping | null;
  network: Network;
  /** The routes of the network. */
  routes: Route[];
}) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<RecordForm>(() => formFrom(record));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    setForm(formFrom(record));
    setErrors({});
    setSubmitError(null);
  }, [isOpen, record]);

  const set = <K extends keyof RecordForm>(key: K, value: RecordForm[K]) => setForm((f) => ({ ...f, [key]: value }));
  const route = routes.find((r) => r.id === form.routeId);
  const name = form.name.trim();
  const ip = form.ip.trim();
  const ipV6 = form.ipV6.trim();
  const fqdn = recordFQDN(name, network.name, network.domain_suffix);

  const validate = () => {
    const e: FieldErrors = {};
    if (!form.routeId) e.routeId = 'Choose the route the addresses are behind.';
    if (!name) e.name = 'Give the record a name.';
    else if (!isValidRecordName(name)) e.name = 'Use letters, digits and hyphens (not at the start or end), or * for a wildcard.';
    if (!ip && !ipV6) e.ip = 'Enter an IPv4 or an IPv6 address, or both.';
    else if (ip && !isIPv4(ip)) e.ip = 'Enter an IPv4 address, e.g. 10.0.0.50.';
    else if (ip && route && !route.destination_cidr) e.ip = `Route “${route.name}” has no IPv4 destination.`;
    if (ipV6 && !isIPv6(ipV6)) e.ipV6 = 'Enter an IPv6 address, e.g. fd00::50.';
    else if (ipV6 && route && !route.destination_cidr_v6) e.ipV6 = `Route “${route.name}” has no IPv6 destination.`;
    setErrors(e);
    return !Object.values(e).some(Boolean);
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    if (!validate()) return;
    setSubmitting(true);
    try {
      // An empty address in an update removes it.
      if (record) await api.updateDNSMapping(network.id, record.route_id, record.id, { name, ip_address: ip, ip_address_v6: ipV6 });
      else await api.createDNSMapping(network.id, form.routeId, { name, ip_address: ip || undefined, ip_address_v6: ipV6 || undefined });
    } catch (err) {
      setSubmitError(apiErrorMessage(err));
      setSubmitting(false);
      return;
    }
    toast.success(`DNS record “${fqdn}” ${record ? 'saved' : 'created'}`);
    void queryClient.invalidateQueries({ queryKey: ['dns'] });
    setSubmitting(false);
    onClose();
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={record ? 'Edit DNS record' : 'New DNS record'}
      description={
        record
          ? `In network ${network.name}. Changes apply when you save.`
          : `A name the peers of network ${network.name} resolve to an address behind a route.`
      }
      onSubmit={handleSubmit}
      submitLabel={record ? 'Save changes' : 'Create record'}
      submitting={submitting}
      error={submitError}
    >
      <Combobox
        label="Route"
        required
        value={form.routeId}
        onChange={(id) => set('routeId', id)}
        options={routes.map((r) => ({
          value: r.id,
          label: r.name,
          description: [r.destination_cidr, r.destination_cidr_v6].filter(Boolean).join(' · '),
        }))}
        placeholder="Select a route…"
        disabled={!!record}
        error={errors.routeId}
        hint={record ? 'A record stays on its route.' : 'The addresses must be in its destinations.'}
      />
      <TextInput
        label="Name"
        required
        value={form.name}
        onChange={(e) => set('name', e.target.value)}
        placeholder="nas"
        error={errors.name}
        autoFocus={!record}
        hint={
          name ? (
            <>
              Resolves as <span className="font-mono text-ink-secondary">{fqdn}</span>
            </>
          ) : (
            'A hostname label, or * for a wildcard.'
          )
        }
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <TextInput
          label="IPv4 address"
          mono
          value={form.ip}
          onChange={(e) => set('ip', e.target.value)}
          placeholder="10.0.0.50"
          error={errors.ip}
          hint={route?.destination_cidr ? `In ${route.destination_cidr}` : 'At least one address, IPv4 or IPv6.'}
        />
        <TextInput
          label="IPv6 address"
          mono
          value={form.ipV6}
          onChange={(e) => set('ipV6', e.target.value)}
          placeholder="fd00::50"
          error={errors.ipV6}
          hint={route?.destination_cidr_v6 ? `In ${route.destination_cidr_v6}` : undefined}
        />
      </div>
    </FormModal>
  );
}
