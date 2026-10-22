import { useEffect, useId, useState } from 'react';
import type { ReactNode } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { CheckIcon, ClipboardDocumentIcon, EyeIcon, EyeSlashIcon, PlusIcon, TrashIcon } from '@heroicons/react/20/solid';
import api from '../../api/client';
import {
  Alert,
  Badge,
  Button,
  Card,
  CardHeader,
  FormModal,
  IconButton,
  Spinner,
  TextInput,
  apiErrorMessage,
  useConfirm,
  useToast,
} from '../../components/ui';
import { resourceKeys, useAPITokens } from '../../hooks/useQueries';
import type { APIToken } from '../../types';
import { formatDate, isPast, timeAgo } from '../../utils/dates';


/** Today as YYYY-MM-DD, in local time (the value format of date inputs). */
function today() {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function CreateTokenModal({
  isOpen,
  onClose,
  onCreated,
}: {
  isOpen: boolean;
  onClose: () => void;
  onCreated: (token: APIToken) => void;
}) {
  const [name, setName] = useState('');
  const [expiresOn, setExpiresOn] = useState('');
  const [errors, setErrors] = useState<Partial<Record<'name' | 'expiresOn', string>>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const minDate = today();

  useEffect(() => {
    if (!isOpen) return;
    setName('');
    setExpiresOn('');
    setErrors({});
    setSubmitError(null);
  }, [isOpen]);

  const handleSubmit = async () => {
    setSubmitError(null);
    const e: typeof errors = {};
    if (!name.trim()) e.name = 'Give the token a name.';
    if (expiresOn && expiresOn < minDate) e.expiresOn = 'Choose today or a later date.';
    setErrors(e);
    if (e.name || e.expiresOn) return;
    setSubmitting(true);
    let token: APIToken;
    try {
      token = await api.createAPIToken({
        name: name.trim(),
        // Valid through the chosen day, in the user's time zone.
        ...(expiresOn ? { expires_at: new Date(`${expiresOn}T23:59:59`).toISOString() } : {}),
      });
    } catch (err) {
      setSubmitError(apiErrorMessage(err));
      setSubmitting(false);
      return;
    }
    setSubmitting(false);
    onCreated(token);
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      size="sm"
      title="New API token"
      description="A token acts with your permissions until it expires or you revoke it."
      onSubmit={handleSubmit}
      submitLabel="Create token"
      submitting={submitting}
      error={submitError}
    >
      <TextInput
        label="Name"
        required
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="CI pipeline, MCP server…"
        autoComplete="off"
        error={errors.name}
        autoFocus
      />
      <TextInput
        label="Expiration"
        type="date"
        min={minDate}
        value={expiresOn}
        onChange={(e) => setExpiresOn(e.target.value)}
        error={errors.expiresOn}
        hint="Optional. Leave empty for a token that never expires."
      />
    </FormModal>
  );
}

/** The secret of a token just created: shown once, with a copy button. */
function TokenCreatedBanner({ token, onDismiss }: { token: APIToken; onDismiss: () => void }) {
  const toast = useToast();
  const [visible, setVisible] = useState(false);
  const [copied, setCopied] = useState(false);
  const secret = token.token ?? '';

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
    } catch {
      toast.error('The token could not be copied. Show it and copy it by hand.');
    }
  };

  return (
    <Alert tone="success" title={`Token “${token.name}” created`}>
      <p>Copy it now: it will not be shown again.</p>
      <div className="mt-2 flex items-center gap-1.5">
        <code
          aria-label="Token"
          className="min-w-0 flex-1 overflow-x-auto rounded-control border border-line bg-surface px-2.5 py-1.5 font-mono text-xs whitespace-nowrap text-ink"
        >
          {visible ? secret : '•'.repeat(40)}
        </code>
        <IconButton label={visible ? 'Hide token' : 'Show token'} onClick={() => setVisible((v) => !v)}>
          {visible ? <EyeSlashIcon className="size-4" /> : <EyeIcon className="size-4" />}
        </IconButton>
        <Button
          variant="secondary"
          size="sm"
          onClick={copy}
          icon={copied ? <CheckIcon className="size-4" /> : <ClipboardDocumentIcon className="size-4" />}
        >
          {copied ? 'Copied' : 'Copy'}
        </Button>
      </div>
      <Button variant="ghost" size="sm" className="mt-2 -ml-3" onClick={onDismiss}>
        I’ve saved it
      </Button>
    </Alert>
  );
}

function TokenRow({ token, onRevoke, revoking }: { token: APIToken; onRevoke: () => void; revoking: boolean }) {
  const expired = isPast(token.expires_at);
  const expires = formatDate(token.expires_at);
  const lastUsed = timeAgo(token.last_used_at);
  const meta = [
    `Created ${formatDate(token.created_at) ?? '—'}`,
    expires ? `${expired ? 'Expired' : 'Expires'} ${expires}` : 'Never expires',
    lastUsed ? `Last used ${lastUsed}` : 'Never used',
  ];
  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-medium text-ink">{token.name}</span>
          {expired && <Badge tone="danger">Expired</Badge>}
        </div>
        <p className="truncate text-xs text-ink-muted">{meta.join(' · ')}</p>
      </div>
      <IconButton label={`Revoke ${token.name}`} tone="danger" onClick={onRevoke} disabled={revoking}>
        {revoking ? <Spinner /> : <TrashIcon className="size-4" />}
      </IconButton>
    </li>
  );
}

const description: ReactNode = (
  <>
    Tokens act with your permissions. Send one in the{' '}
    <code className="rounded bg-surface-muted px-1 font-mono text-xs">Authorization: Bearer wirety_…</code> header.
  </>
);

/**
 * The signed-in user's API tokens: list, creation (the secret is shown once)
 * and revocation. Used by the profile page (as a card) and the profile
 * dialog (`compact`).
 */
export function ApiTokens({ compact = false }: { compact?: boolean }) {
  const titleId = useId();
  const queryClient = useQueryClient();
  const toast = useToast();
  const confirm = useConfirm();
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<APIToken | null>(null);
  const { data: tokens = [], isLoading, isError, error } = useAPITokens();

  const revoke = useMutation({
    mutationFn: (t: APIToken) => api.deleteAPIToken(t.id),
    onSuccess: (_, t) => {
      toast.success(`Token “${t.name}” revoked`);
      setCreated((c) => (c?.id === t.id ? null : c));
    },
    onError: (err) => toast.error(apiErrorMessage(err)),
    onSettled: () => queryClient.invalidateQueries({ queryKey: resourceKeys.apiTokens }),
  });

  const handleRevoke = async (t: APIToken) => {
    const ok = await confirm({
      title: `Revoke token “${t.name}”?`,
      message: 'The scripts and tools using it lose access immediately. This cannot be undone.',
      confirmLabel: 'Revoke token',
      destructive: true,
    });
    if (ok) revoke.mutate(t);
  };

  const handleCreated = (token: APIToken) => {
    setCreating(false);
    setCreated(token);
    void queryClient.invalidateQueries({ queryKey: resourceKeys.apiTokens });
  };

  const newButton = (
    <Button size="sm" icon={<PlusIcon className="size-4" />} onClick={() => setCreating(true)}>
      New token
    </Button>
  );

  const content = (
    <>
      {created && <TokenCreatedBanner token={created} onDismiss={() => setCreated(null)} />}
      {isLoading ? (
        <div className="flex justify-center py-6 text-ink-muted" role="status">
          <Spinner className="size-5" />
          <span className="sr-only">Loading tokens…</span>
        </div>
      ) : isError ? (
        <Alert tone="danger">The tokens could not be loaded: {apiErrorMessage(error)}</Alert>
      ) : tokens.length === 0 ? (
        <p className="rounded-control border border-dashed border-line-strong px-3 py-5 text-center text-sm text-ink-muted">
          No API token yet.
        </p>
      ) : (
        <ul className="divide-y divide-line rounded-control border border-line" aria-labelledby={titleId}>
          {tokens.map((t) => (
            <TokenRow
              key={t.id}
              token={t}
              onRevoke={() => handleRevoke(t)}
              revoking={revoke.isPending && revoke.variables?.id === t.id}
            />
          ))}
        </ul>
      )}
      <CreateTokenModal isOpen={creating} onClose={() => setCreating(false)} onCreated={handleCreated} />
    </>
  );

  if (compact) {
    return (
      <section aria-labelledby={titleId} className="space-y-3">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h3 id={titleId} className="text-sm font-semibold text-ink">
              API tokens
            </h3>
            <p className="mt-0.5 text-sm text-ink-muted">{description}</p>
          </div>
          {newButton}
        </div>
        {content}
      </section>
    );
  }

  return (
    <Card>
      <CardHeader title={<span id={titleId}>API tokens</span>} description={description} actions={newButton} />
      <div className="space-y-4 px-5 py-4">{content}</div>
    </Card>
  );
}
