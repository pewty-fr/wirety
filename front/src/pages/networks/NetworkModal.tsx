import { useEffect, useState } from 'react';
import type { KeyboardEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { SparklesIcon, XMarkIcon } from '@heroicons/react/20/solid';
import api from '../../api/client';
import { Button, FormModal, IconButton, TextInput, apiErrorMessage, useToast } from '../../components/ui';
import { queryKeys } from '../../hooks/useQueries';
import type { Network } from '../../types';
import { getCIDRError, isIPAddress, suggestIPv6ULACIDRs } from '../../utils/validation';

interface NetworkForm {
  name: string;
  cidr: string;
  cidrV6: string;
  dns: string[];
  domainSuffix: string;
}

type FieldName = 'name' | 'cidr' | 'cidrV6' | 'dns' | 'domainSuffix' | 'maxPeers';
type FieldErrors = Partial<Record<FieldName, string>>;

interface CIDRSuggestion {
  prefix: number;
  hosts: number;
  cidrs: string[];
}

function formFrom(network: Network | null): NetworkForm {
  return {
    name: network?.name ?? '',
    cidr: network?.cidr ?? '',
    cidrV6: network?.cidr_v6 ?? '',
    dns: network?.dns ?? [],
    domainSuffix: network?.domain_suffix || 'internal',
  };
}

// A DNS hostname, as the server checks network names and domain suffixes:
// dot-separated labels of lowercase letters, digits and inner hyphens.
const LABEL = '[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?';
const HOSTNAME = new RegExp(`^${LABEL}(\\.${LABEL})*$`);
const isHostname = (value: string) => value.length <= 253 && HOSTNAME.test(value);

/** The error of a CIDR typed in the field of the given family, or null. */
function cidrError(value: string, family: 4 | 6): string | null {
  const cidr = value.trim();
  if (!cidr) return null;
  const isV6 = cidr.includes(':');
  if (family === 4 && isV6) return 'This is an IPv6 range: enter it in the IPv6 CIDR field.';
  if (family === 6 && !isV6 && /^\d+\.\d+/.test(cidr)) return 'This is an IPv4 range: enter it in the IPv4 CIDR field.';
  return getCIDRError(cidr);
}

function dnsServerError(value: string, current: string[]): string | null {
  if (!isIPAddress(value)) return 'Enter an IPv4 or IPv6 address, e.g. 1.1.1.1.';
  if (current.includes(value)) return 'This server is already in the list.';
  return null;
}

/**
 * Creates or edits a network: its name, address ranges (with free IPv4
 * ranges and IPv6 ULA prefixes to pick from), DNS servers and domain suffix.
 */
export default function NetworkModal({
  isOpen,
  onClose,
  network,
}: {
  isOpen: boolean;
  onClose: () => void;
  /** The network to edit, or null to create one. */
  network: Network | null;
}) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<NetworkForm>(() => formFrom(network));
  const [dnsInput, setDnsInput] = useState('');
  const [errors, setErrors] = useState<FieldErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [maxPeers, setMaxPeers] = useState('100');
  const [suggestion, setSuggestion] = useState<CIDRSuggestion | null>(null);
  const [suggesting, setSuggesting] = useState(false);
  const [ulaPrefixes, setUlaPrefixes] = useState<string[]>([]);

  useEffect(() => {
    if (!isOpen) return;
    setForm(formFrom(network));
    setDnsInput('');
    setErrors({});
    setSubmitError(null);
    setMaxPeers('100');
    setSuggestion(null);
    setUlaPrefixes([]);
  }, [isOpen, network]);

  const isEdit = network !== null;
  const setError = (field: FieldName, message: string | null) => setErrors((e) => ({ ...e, [field]: message ?? undefined }));

  const setCidr = (value: string) => {
    setForm((f) => ({ ...f, cidr: value }));
    setError('cidr', cidrError(value, 4));
  };
  const setCidrV6 = (value: string) => {
    setForm((f) => ({ ...f, cidrV6: value }));
    setError('cidrV6', cidrError(value, 6));
  };

  const suggestRanges = async () => {
    const count = Number(maxPeers);
    if (!Number.isInteger(count) || count < 1) {
      setError('maxPeers', 'Enter a whole number of peers, at least 1.');
      return;
    }
    setError('maxPeers', null);
    setSuggesting(true);
    try {
      const res = await api.getSuggestedCIDRs(count, 10);
      setSuggestion({ prefix: res.suggested_prefix, hosts: res.usable_hosts, cidrs: res.cidrs ?? [] });
    } catch (err) {
      setSuggestion(null);
      setError('maxPeers', apiErrorMessage(err));
    } finally {
      setSuggesting(false);
    }
  };

  const addDns = () => {
    const server = dnsInput.trim();
    if (!server) return;
    const error = dnsServerError(server, form.dns);
    setError('dns', error);
    if (error) return;
    setForm((f) => ({ ...f, dns: [...f.dns, server] }));
    setDnsInput('');
  };

  const onDnsKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return;
    // Enter adds the server instead of submitting the form.
    e.preventDefault();
    addDns();
  };

  /** Checks the form; returns the DNS servers to send, or null when invalid. */
  const validate = (): string[] | null => {
    const e: FieldErrors = {};
    const name = form.name.trim();
    if (!name) e.name = 'Give the network a name.';
    else if (!isHostname(name)) e.name = 'Use lowercase letters, digits, hyphens and dots, e.g. prod or prod.eu.';

    const cidr = form.cidr.trim();
    const cidrV6 = form.cidrV6.trim();
    if (!cidr && !cidrV6) {
      e.cidr = 'Enter an IPv4 CIDR, an IPv6 CIDR, or both.';
    } else {
      e.cidr = cidrError(cidr, 4) ?? undefined;
      e.cidrV6 = cidrError(cidrV6, 6) ?? undefined;
    }
    // The server keeps a range when it receives none: it can change, not go.
    if (network?.cidr && !cidr) e.cidr = 'The IPv4 range of a network can be changed, not removed.';
    if (network?.cidr_v6 && !cidrV6) e.cidrV6 = 'The IPv6 range of a network can be changed, not removed.';

    // A server typed but not added yet is added with the rest.
    let dns = form.dns;
    const pending = dnsInput.trim();
    if (pending) {
      const error = dnsServerError(pending, dns);
      if (error) e.dns = error;
      else dns = [...dns, pending];
    }

    const suffix = form.domainSuffix.trim();
    if (!suffix) e.domainSuffix = 'Enter a domain suffix, e.g. internal.';
    else if (!isHostname(suffix)) e.domainSuffix = 'Use lowercase letters, digits, hyphens and dots, e.g. internal or corp.example.com.';

    setErrors(e);
    if (Object.values(e).some(Boolean)) return null;
    if (pending) {
      setForm((f) => ({ ...f, dns }));
      setDnsInput('');
    }
    return dns;
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    const dns = validate();
    if (!dns) return;
    setSubmitting(true);
    const name = form.name.trim();
    const cidr = form.cidr.trim();
    const cidrV6 = form.cidrV6.trim();
    const domainSuffix = form.domainSuffix.trim();
    const rangesChanged = isEdit && (cidr !== network.cidr || cidrV6 !== (network.cidr_v6 ?? ''));
    try {
      if (isEdit) {
        const saved = await api.updateNetwork(network.id, {
          name,
          cidr: cidr || undefined,
          cidr_v6: cidrV6 || undefined,
          dns,
          domain_suffix: domainSuffix,
          default_group_ids: network.default_group_ids ?? [],
        });
        if (saved?.id) queryClient.setQueryData(queryKeys.network(network.id), saved);
      } else {
        await api.createNetwork({
          name,
          cidr: cidr || undefined,
          cidr_v6: cidrV6 || undefined,
          dns: dns.length > 0 ? dns : undefined,
          domain_suffix: domainSuffix,
        });
      }
    } catch (err) {
      setSubmitError(apiErrorMessage(err));
      setSubmitting(false);
      return;
    }
    toast.success(isEdit ? `Network “${name}” saved` : `Network “${name}” created`);
    setSubmitting(false);
    void queryClient.invalidateQueries({ queryKey: ['networks'] });
    void queryClient.invalidateQueries({ queryKey: ['network'] });
    if (rangesChanged) {
      // Peers got new addresses.
      void queryClient.invalidateQueries({ queryKey: ['peers'] });
      void queryClient.invalidateQueries({ queryKey: ['networkPeers'] });
      void queryClient.invalidateQueries({ queryKey: ['ipam'] });
    }
    onClose();
  };

  const cidrChanged = isEdit && !!network.cidr && form.cidr.trim() !== network.cidr;
  const cidrV6Changed = isEdit && !!network.cidr_v6 && form.cidrV6.trim() !== network.cidr_v6;
  const readdressHint = 'Saving gives every peer a new address in this range.';

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={isEdit ? 'Edit network' : 'New network'}
      description={
        isEdit
          ? `Changes to “${network.name}” apply when you save.`
          : 'A WireGuard network: its peers get their addresses from its ranges.'
      }
      onSubmit={handleSubmit}
      submitLabel={isEdit ? 'Save changes' : 'Create network'}
      submitting={submitting}
      error={submitError}
    >
      <TextInput
        label="Name"
        required
        value={form.name}
        onChange={(e) => {
          setForm((f) => ({ ...f, name: e.target.value }));
          setError('name', null);
        }}
        placeholder="prod"
        error={errors.name}
        hint="Lowercase letters, digits, hyphens and dots, e.g. prod or prod.eu."
        autoFocus={!isEdit}
        autoComplete="off"
      />

      <div className="space-y-3">
        <TextInput
          label="IPv4 CIDR"
          mono
          value={form.cidr}
          onChange={(e) => setCidr(e.target.value)}
          placeholder="10.0.0.0/16"
          error={errors.cidr}
          hint={cidrChanged ? readdressHint : 'Optional if the network has an IPv6 range.'}
          autoComplete="off"
        />
        {!isEdit && (
          <div className="space-y-3 rounded-control border border-line bg-surface-muted/50 p-3">
            <div className="grid grid-cols-[1fr_auto] items-start gap-2">
              <TextInput
                label="Expected max peers"
                type="number"
                inputMode="numeric"
                min={1}
                value={maxPeers}
                onChange={(e) => {
                  setMaxPeers(e.target.value);
                  setError('maxPeers', null);
                }}
                error={errors.maxPeers}
                hint="Suggests free IPv4 ranges of the right size."
              />
              <Button variant="secondary" className="mt-6.5" loading={suggesting} onClick={suggestRanges}>
                Suggest
              </Button>
            </div>
            {suggestion &&
              (suggestion.cidrs.length > 0 ? (
                <div className="space-y-2">
                  <p className="text-xs text-ink-muted">
                    Free /{suggestion.prefix} ranges ({suggestion.hosts.toLocaleString()} addresses each). Pick one:
                  </p>
                  <div role="group" aria-label="Suggested IPv4 ranges" className="flex flex-wrap gap-2">
                    {suggestion.cidrs.map((cidr) => (
                      <Button
                        key={cidr}
                        variant="secondary"
                        size="sm"
                        className="font-mono"
                        onClick={() => {
                          setCidr(cidr);
                          setSuggestion(null);
                        }}
                      >
                        {cidr}
                      </Button>
                    ))}
                  </div>
                </div>
              ) : (
                <p className="text-xs text-ink-muted">No free range of this size is left.</p>
              ))}
          </div>
        )}
      </div>

      <div className="space-y-2">
        <TextInput
          label="IPv6 CIDR"
          mono
          value={form.cidrV6}
          onChange={(e) => setCidrV6(e.target.value)}
          placeholder="fd00::/64"
          error={errors.cidrV6}
          hint={
            cidrV6Changed
              ? readdressHint
              : 'Optional, for dual stack. Use a unique local address (fd00::/8), never a public prefix.'
          }
          autoComplete="off"
        />
        <Button
          variant="ghost"
          size="sm"
          icon={<SparklesIcon className="size-4" />}
          onClick={() => setUlaPrefixes(suggestIPv6ULACIDRs(5))}
          title="Random unique local address prefixes (RFC 4193), the IPv6 equivalent of private ranges"
        >
          Suggest ULA prefixes
        </Button>
        {ulaPrefixes.length > 0 && (
          <div className="space-y-2 rounded-control border border-line bg-surface-muted/50 p-3">
            <p className="text-xs text-ink-muted">
              Each prefix has its own random 40-bit global ID (RFC 4193): private, never routed on the internet. Pick one:
            </p>
            <div role="group" aria-label="Suggested IPv6 prefixes" className="flex flex-wrap gap-2">
              {ulaPrefixes.map((cidr) => (
                <Button
                  key={cidr}
                  variant="secondary"
                  size="sm"
                  className="font-mono"
                  onClick={() => {
                    setCidrV6(cidr);
                    setUlaPrefixes([]);
                  }}
                >
                  {cidr}
                </Button>
              ))}
            </div>
          </div>
        )}
      </div>

      <div className="space-y-2">
        <div className="grid grid-cols-[1fr_auto] items-start gap-2">
          <TextInput
            label="DNS servers"
            mono
            value={dnsInput}
            onChange={(e) => {
              setDnsInput(e.target.value);
              setError('dns', null);
            }}
            onKeyDown={onDnsKeyDown}
            placeholder="1.1.1.1"
            error={errors.dns}
            hint="Optional. Given to the peers in their configuration; press Enter to add."
            autoComplete="off"
          />
          <Button variant="secondary" className="mt-6.5" onClick={addDns}>
            Add
          </Button>
        </div>
        {form.dns.length > 0 && (
          <ul aria-label="Added DNS servers" className="divide-y divide-line rounded-control border border-line">
            {form.dns.map((server) => (
              <li key={server} className="flex items-center justify-between gap-3 py-1 pr-1 pl-3">
                <span className="font-mono text-[13px] text-ink">{server}</span>
                <IconButton
                  label={`Remove ${server}`}
                  tone="danger"
                  onClick={() => setForm((f) => ({ ...f, dns: f.dns.filter((s) => s !== server) }))}
                >
                  <XMarkIcon className="size-4" />
                </IconButton>
              </li>
            ))}
          </ul>
        )}
      </div>

      <TextInput
        label="Domain suffix"
        required
        mono
        value={form.domainSuffix}
        onChange={(e) => {
          setForm((f) => ({ ...f, domainSuffix: e.target.value }));
          setError('domainSuffix', null);
        }}
        placeholder="internal"
        error={errors.domainSuffix}
        hint={
          form.name.trim() && form.domainSuffix.trim()
            ? `DNS names in this network end in .${form.name.trim()}.${form.domainSuffix.trim()}`
            : 'Ends the DNS names of the network, after its name, e.g. internal or corp.example.com.'
        }
        autoComplete="off"
      />
    </FormModal>
  );
}
