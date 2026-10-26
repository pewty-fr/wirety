import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it, vi } from 'vitest';
import { server } from '../../test/setup';
import { db, requestsTo, seedNetwork } from '../../test/fakeApi';
import { findDialog } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

/** Replaces the clipboard (after renderApp: user-event installs its own). */
function stubClipboard() {
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(window.navigator, 'clipboard', { value: { writeText }, configurable: true });
  return writeText;
}

/** Counts the sign-outs (the fake API does not record them). */
function watchLogout() {
  const calls = { count: 0 };
  server.use(
    http.post('*/api/v1/auth/logout', () => {
      calls.count += 1;
      return HttpResponse.json({});
    }),
  );
  return calls;
}

function seedToken(name: string, over: Partial<(typeof db.tokens)[number]> = {}) {
  const t = { id: `tok-${name.toLowerCase().replace(/\W+/g, '-')}`, name, created_at: '2026-10-01T10:00:00Z', ...over };
  db.tokens.push(t);
  return t;
}

describe('Profile page', () => {
  it('shows the account read-only', async () => {
    const corp = seedNetwork({ name: 'corp' });
    seedNetwork({ name: 'lab' });
    db.me = { ...db.me, id: 'user-bob', name: 'Bob', email: 'bob@example.com', role: 'user', authorized_networks: [corp.id] };
    renderApp('/profile');

    expect(await screen.findByRole('heading', { name: 'Profile' })).toBeInTheDocument();
    const account = screen.getByText('Name').closest('dl') as HTMLElement;
    expect(within(account).getByText('Bob')).toBeInTheDocument();
    expect(within(account).getByText('bob@example.com')).toBeInTheDocument();
    expect(within(account).getByText('User')).toBeInTheDocument();
    expect(within(account).getByText('Password')).toBeInTheDocument();
    const networks = within(account).getByRole('list', { name: 'Authorized networks' });
    expect(await within(networks).findByText('corp')).toBeInTheDocument();
    expect(within(networks).queryByText('lab')).not.toBeInTheDocument();
    expect(within(account).queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('tells administrators they use every network', async () => {
    renderApp('/profile');
    expect(await screen.findByText('All networks, as an administrator.')).toBeInTheDocument();
    const role = screen.getByText('Role', { selector: 'dt' }).parentElement as HTMLElement;
    expect(within(role).getByText('Administrator')).toBeInTheDocument();
  });

  it('creates an API token and shows its secret once, with a copy button', async () => {
    const { user } = renderApp('/profile');
    const writeText = stubClipboard();

    await user.click(await screen.findByRole('button', { name: 'New token' }));
    const dialog = await findDialog('New API token');
    await user.type(within(dialog).getByLabelText(/^Name/), 'CI pipeline');
    fireEvent.change(within(dialog).getByLabelText('Expiration'), { target: { value: '2099-12-31' } });
    await user.click(within(dialog).getByRole('button', { name: 'Create token' }));

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New API token' })).not.toBeInTheDocument());
    expect(requestsTo('POST', /\/users\/me\/tokens$/)[0].body).toEqual({
      name: 'CI pipeline',
      // Valid through the chosen day, in the browser's time zone.
      expires_at: new Date('2099-12-31T23:59:59').toISOString(),
    });
    const banner = screen.getByText('Token “CI pipeline” created').closest('[role="status"]') as HTMLElement;
    expect(within(banner).getByText('Copy it now: it will not be shown again.')).toBeInTheDocument();
    // Hidden until asked for.
    expect(within(banner).queryByText('wirety_tok_secret')).not.toBeInTheDocument();
    await user.click(within(banner).getByRole('button', { name: 'Show token' }));
    expect(within(banner).getByText('wirety_tok_secret')).toBeInTheDocument();

    await user.click(within(banner).getByRole('button', { name: 'Copy' }));
    expect(writeText).toHaveBeenCalledWith('wirety_tok_secret');
    expect(await within(banner).findByRole('button', { name: 'Copied' })).toBeInTheDocument();

    const list = await screen.findByRole('list', { name: 'API tokens' });
    expect(within(list).getByText('CI pipeline')).toBeInTheDocument();
    expect(within(list).getByText(/Never used/)).toBeInTheDocument();

    await user.click(within(banner).getByRole('button', { name: 'I’ve saved it' }));
    expect(screen.queryByText('Token “CI pipeline” created')).not.toBeInTheDocument();
    expect(screen.queryByText('wirety_tok_secret')).not.toBeInTheDocument();
  });

  it('creates a token that never expires', async () => {
    const { user } = renderApp('/profile');
    await user.click(await screen.findByRole('button', { name: 'New token' }));
    const dialog = await findDialog('New API token');
    await user.type(within(dialog).getByLabelText(/^Name/), 'MCP server');
    await user.click(within(dialog).getByRole('button', { name: 'Create token' }));

    expect(await screen.findByText('Token “MCP server” created')).toBeInTheDocument();
    expect(requestsTo('POST', /\/users\/me\/tokens$/)[0].body).toEqual({ name: 'MCP server' });
    expect(within(await screen.findByRole('list', { name: 'API tokens' })).getByText(/Never expires/)).toBeInTheDocument();
  });

  it('checks the token form before sending anything', async () => {
    const { user } = renderApp('/profile');
    await user.click(await screen.findByRole('button', { name: 'New token' }));
    const dialog = await findDialog('New API token');
    fireEvent.change(within(dialog).getByLabelText('Expiration'), { target: { value: '2000-01-01' } });

    await user.click(within(dialog).getByRole('button', { name: 'Create token' }));

    expect(within(dialog).getByText('Give the token a name.')).toBeInTheDocument();
    expect(within(dialog).getByText('Choose today or a later date.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');
    expect(within(dialog).getByLabelText('Expiration')).toHaveAttribute('aria-invalid', 'true');
    expect(requestsTo('POST', /\/users\/me\/tokens$/)).toHaveLength(0);
  });

  it('shows the server error in the token dialog, and starts empty after a cancel', async () => {
    server.use(http.post('*/api/v1/users/me/tokens', () => HttpResponse.json({ error: 'token name already used' }, { status: 409 })));
    const { user } = renderApp('/profile');
    await user.click(await screen.findByRole('button', { name: 'New token' }));
    let dialog = await findDialog('New API token');
    await user.type(within(dialog).getByLabelText(/^Name/), 'CI pipeline');
    await user.click(within(dialog).getByRole('button', { name: 'Create token' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('token name already used');
    expect(screen.getByRole('dialog', { name: 'New API token' })).toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await user.click(screen.getByRole('button', { name: 'New token' }));
    dialog = await findDialog('New API token');
    expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('');
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
  });

  it('lists the tokens with their expiry and last use', async () => {
    seedToken('old', { expires_at: '2020-01-01T00:00:00Z' });
    seedToken('deploy', { last_used_at: new Date().toISOString() });
    renderApp('/profile');
    const list = await screen.findByRole('list', { name: 'API tokens' });
    const old = within(list).getByText('old').closest('li') as HTMLElement;
    expect(within(old).getByText('Expired')).toBeInTheDocument();
    expect(within(old).getByText(/Expired Jan 1, 2020/)).toBeInTheDocument();
    const deploy = within(list).getByText('deploy').closest('li') as HTMLElement;
    expect(within(deploy).getByText(/Last used just now/)).toBeInTheDocument();
  });

  it('revokes a token after confirmation', async () => {
    const token = seedToken('CI pipeline');
    seedToken('MCP server');
    const { user } = renderApp('/profile');

    await user.click(await screen.findByRole('button', { name: 'Revoke CI pipeline' }));
    let confirm = await findDialog('Revoke token “CI pipeline”?');
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    expect(requestsTo('DELETE', /\/users\/me\/tokens\//)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: 'Revoke CI pipeline' }));
    confirm = await findDialog('Revoke token “CI pipeline”?');
    await user.click(within(confirm).getByRole('button', { name: 'Revoke token' }));

    expect(await screen.findByText('Token “CI pipeline” revoked')).toBeInTheDocument();
    expect(requestsTo('DELETE', /\/users\/me\/tokens\//)[0].path).toBe(`/users/me/tokens/${token.id}`);
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Revoke CI pipeline' })).not.toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Revoke MCP server' })).toBeInTheDocument();
  });

  it('shows a failed revocation in a toast', async () => {
    seedToken('CI pipeline');
    server.use(http.delete('*/api/v1/users/me/tokens/:id', () => HttpResponse.json({ error: 'token not found' }, { status: 404 })));
    const { user } = renderApp('/profile');
    await user.click(await screen.findByRole('button', { name: 'Revoke CI pipeline' }));
    const confirm = await findDialog('Revoke token “CI pipeline”?');
    await user.click(within(confirm).getByRole('button', { name: 'Revoke token' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('token not found');
  });

  it('signs out in password mode too', async () => {
    const logout = watchLogout();
    const { user } = renderApp('/profile');
    await user.click(await screen.findByRole('button', { name: 'Sign out' }));

    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument();
    expect(logout.count).toBe(1);
  });
});

describe('Profile dialog', () => {
  const openProfile = async (user: ReturnType<typeof renderApp>['user']) => {
    await user.click(await screen.findByRole('button', { name: /admin@wirety\.local/ }));
    return findDialog('Profile');
  };

  it('opens from the sidebar with the account and the API tokens', async () => {
    seedToken('CI pipeline');
    const { user } = renderApp('/dashboard');
    const dialog = await openProfile(user);

    expect(within(dialog).getByText('admin@wirety.local')).toBeInTheDocument();
    expect(within(dialog).getByText('All networks, as an administrator.')).toBeInTheDocument();
    expect(await within(dialog).findByRole('list', { name: 'API tokens' })).toHaveTextContent('CI pipeline');

    await user.click(within(dialog).getByRole('button', { name: 'Done' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Profile' })).not.toBeInTheDocument());
  });

  it('creates and revokes tokens like the profile page', async () => {
    const { user } = renderApp('/dashboard');
    const writeText = stubClipboard();
    const profile = await openProfile(user);

    await user.click(within(profile).getByRole('button', { name: 'New token' }));
    const create = await findDialog('New API token');
    await user.type(within(create).getByLabelText(/^Name/), 'laptop');
    await user.click(within(create).getByRole('button', { name: 'Create token' }));

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New API token' })).not.toBeInTheDocument());
    // The profile dialog stays open, with the new secret.
    expect(screen.getByRole('dialog', { name: 'Profile' })).toBeInTheDocument();
    expect(requestsTo('POST', /\/users\/me\/tokens$/)[0].body).toEqual({ name: 'laptop' });
    expect(within(profile).getByText('Token “laptop” created')).toBeInTheDocument();
    await user.click(within(profile).getByRole('button', { name: 'Copy' }));
    expect(writeText).toHaveBeenCalledWith('wirety_tok_secret');

    await user.click(await within(profile).findByRole('button', { name: 'Revoke laptop' }));
    const confirm = await findDialog('Revoke token “laptop”?');
    await user.click(within(confirm).getByRole('button', { name: 'Revoke token' }));

    expect(await screen.findByText('Token “laptop” revoked')).toBeInTheDocument();
    expect(requestsTo('DELETE', /\/users\/me\/tokens\//)).toHaveLength(1);
    expect(screen.getByRole('dialog', { name: 'Profile' })).toBeInTheDocument();
    // The secret of a revoked token is no use any more.
    expect(within(profile).queryByText('Token “laptop” created')).not.toBeInTheDocument();
  });

  it('signs out', async () => {
    const logout = watchLogout();
    const { user } = renderApp('/dashboard');
    const dialog = await openProfile(user);
    await user.click(within(dialog).getByRole('button', { name: 'Sign out' }));

    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument();
    expect(logout.count).toBe(1);
  });
});
