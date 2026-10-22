import { screen, waitFor, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { server } from '../../test/setup';
import { db, requestsTo, seedNetwork, seedUser } from '../../test/fakeApi';
import { findDialog } from '../../test/helpers';
import { renderApp } from '../../test/renderApp';

/** Single sign-on mode: users are created on their first sign-in and have no password. */
function useSingleSignOn() {
  server.use(
    http.get('*/api/v1/auth/config', () =>
      HttpResponse.json({
        enabled: true,
        issuer_url: 'https://sso.example.com',
        client_id: 'wirety',
        simple_auth: false,
        authorization_endpoint: 'https://sso.example.com/auth',
        end_session_endpoint: '',
        scope: 'openid',
        authorization_extra_params: '',
      }),
    ),
  );
}

function seedTeam() {
  const corp = seedNetwork({ name: 'corp', cidr: '10.42.0.0/24' });
  const lab = seedNetwork({ name: 'lab', cidr: '10.43.0.0/24' });
  const bob = seedUser({ name: 'Bob', email: 'bob@example.com', authorized_networks: [corp.id] });
  const carol = seedUser({ name: 'Carol', email: 'carol@example.com', role: 'administrator' });
  return { corp, lab, bob, carol };
}

const row = (name: RegExp) => screen.getByRole('row', { name });

describe('Users', () => {
  it('lists the users with their role, networks and last sign-in', async () => {
    const { lab } = seedTeam();
    seedUser({ name: 'Dave', email: 'dave@example.com', authorized_networks: [] });
    seedUser({ name: 'Erin', email: 'erin@example.com', authorized_networks: [lab.id], last_login_at: '0001-01-01T00:00:00Z' });
    renderApp('/users');

    await screen.findByRole('table');
    expect(within(row(/Bob/)).getByText('bob@example.com')).toBeInTheDocument();
    expect(within(row(/Bob/)).getByText('User')).toBeInTheDocument();
    expect(within(row(/Bob/)).getByText('corp')).toBeInTheDocument();
    expect(within(row(/Carol/)).getByText('Administrator')).toBeInTheDocument();
    expect(within(row(/Carol/)).getByText('All networks')).toBeInTheDocument();
    expect(within(row(/Dave/)).getByText('No access')).toBeInTheDocument();
    // Go's zero time means the user never signed in.
    expect(within(row(/Erin/)).getByText('Never')).toBeInTheDocument();
    expect(within(row(/admin@wirety\.local/)).getByText('(you)')).toBeInTheDocument();
  });

  it('is reserved to administrators', async () => {
    db.me = { ...db.me, role: 'user' };
    renderApp('/users');
    expect(await screen.findByText('Only administrators can manage users.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'New user' })).not.toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  describe('new user', () => {
    it('creates a user with a password, a role and networks', async () => {
      const { corp } = seedTeam();
      const { user } = renderApp('/users');

      await user.click(await screen.findByRole('button', { name: 'New user' }));
      const dialog = await findDialog('New user');
      await user.type(within(dialog).getByLabelText(/^Email/), 'alice@example.com');
      await user.type(within(dialog).getByLabelText(/^Name/), 'Alice');
      const password = within(dialog).getByLabelText(/^Password/);
      await user.type(password, 's3cret-pass');
      expect(password).toHaveAttribute('type', 'password');
      await user.click(within(dialog).getByRole('button', { name: 'Show password' }));
      expect(password).toHaveAttribute('type', 'text');
      expect(within(dialog).getByLabelText('Role')).toHaveValue('user');
      await user.click(within(dialog).getByRole('checkbox', { name: 'corp' }));

      await user.click(within(dialog).getByRole('button', { name: 'Create user' }));

      expect(await screen.findByText('User “Alice” created')).toBeInTheDocument();
      await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New user' })).not.toBeInTheDocument());
      expect(requestsTo('POST', /^\/users$/)[0].body).toEqual({
        email: 'alice@example.com',
        name: 'Alice',
        role: 'user',
        password: 's3cret-pass',
        authorized_networks: [corp.id],
      });
      expect(await within(row(/Alice/)).findByText('corp')).toBeInTheDocument();
    });

    it('creates an administrator without networks', async () => {
      seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'New user' }));
      const dialog = await findDialog('New user');
      await user.type(within(dialog).getByLabelText(/^Email/), 'root@example.com');
      await user.type(within(dialog).getByLabelText(/^Name/), 'Root');
      await user.type(within(dialog).getByLabelText(/^Password/), 'long-enough');
      // Networks checked before switching to administrator are not sent.
      await user.click(within(dialog).getByRole('checkbox', { name: 'lab' }));
      await user.selectOptions(within(dialog).getByLabelText('Role'), 'administrator');
      expect(within(dialog).queryByRole('group', { name: 'Authorized networks' })).not.toBeInTheDocument();

      await user.click(within(dialog).getByRole('button', { name: 'Create user' }));

      await screen.findByText('User “Root” created');
      expect(requestsTo('POST', /^\/users$/)[0].body).toEqual({
        email: 'root@example.com',
        name: 'Root',
        role: 'administrator',
        password: 'long-enough',
        authorized_networks: [],
      });
    });

    it('checks the form before sending anything', async () => {
      seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'New user' }));
      const dialog = await findDialog('New user');

      await user.click(within(dialog).getByRole('button', { name: 'Create user' }));

      expect(within(dialog).getByText('Enter the email address the user signs in with.')).toBeInTheDocument();
      expect(within(dialog).getByText('Give the user a name.')).toBeInTheDocument();
      expect(within(dialog).getByText('Use at least 8 characters.')).toBeInTheDocument();
      expect(within(dialog).getByLabelText(/^Email/)).toHaveAttribute('aria-invalid', 'true');

      await user.type(within(dialog).getByLabelText(/^Email/), 'alice@');
      await user.type(within(dialog).getByLabelText(/^Name/), 'Alice');
      await user.type(within(dialog).getByLabelText(/^Password/), 'short');
      await user.click(within(dialog).getByRole('button', { name: 'Create user' }));

      expect(within(dialog).getByText('Enter a valid email address.')).toBeInTheDocument();
      expect(within(dialog).queryByText('Give the user a name.')).not.toBeInTheDocument();
      expect(within(dialog).getByLabelText(/^Password/)).toHaveAttribute('aria-invalid', 'true');
      expect(requestsTo('POST', /^\/users$/)).toHaveLength(0);
    });

    it('shows the server error in the dialog', async () => {
      seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'New user' }));
      const dialog = await findDialog('New user');
      await user.type(within(dialog).getByLabelText(/^Email/), 'bob@example.com');
      await user.type(within(dialog).getByLabelText(/^Name/), 'Bobby');
      await user.type(within(dialog).getByLabelText(/^Password/), 'long-enough');

      await user.click(within(dialog).getByRole('button', { name: 'Create user' }));

      expect(await within(dialog).findByRole('alert')).toHaveTextContent('a user with this email already exists');
      expect(screen.getByRole('dialog', { name: 'New user' })).toBeInTheDocument();
    });

    it('starts empty each time and sends nothing on cancel', async () => {
      seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'New user' }));
      let dialog = await findDialog('New user');
      await user.type(within(dialog).getByLabelText(/^Name/), 'Draft');
      await user.click(within(dialog).getByRole('checkbox', { name: 'corp' }));
      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New user' })).not.toBeInTheDocument());

      await user.click(screen.getByRole('button', { name: 'New user' }));
      dialog = await findDialog('New user');
      expect(within(dialog).getByLabelText(/^Name/)).toHaveValue('');
      expect(within(dialog).getByRole('checkbox', { name: 'corp' })).not.toBeChecked();
      expect(requestsTo('POST', /^\/users$/)).toHaveLength(0);
    });

    it('is only offered in password mode', async () => {
      useSingleSignOn();
      seedTeam();
      renderApp('/users');
      await screen.findByRole('table');
      expect(screen.getByRole('button', { name: 'Default permissions' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'New user' })).not.toBeInTheDocument();
      // Single sign-on users have no password to reset.
      expect(screen.queryByRole('button', { name: 'Reset password for Bob' })).not.toBeInTheDocument();
    });
  });

  describe('edit user', () => {
    it('shows the details of the clicked user and saves name, role and networks together', async () => {
      const { lab, bob } = seedTeam();
      const { user } = renderApp('/users');

      await user.click(within(await screen.findByRole('table')).getByText('Bob'));
      const dialog = await findDialog('Edit user Bob');
      expect(within(dialog).getByText('bob@example.com')).toBeInTheDocument();
      expect(within(dialog).getByText('Last sign-in')).toBeInTheDocument();
      expect(within(dialog).getByText(bob.id)).toBeInTheDocument();
      expect(within(dialog).getByRole('checkbox', { name: 'corp' })).toBeChecked();

      await user.clear(within(dialog).getByLabelText(/^Name/));
      await user.type(within(dialog).getByLabelText(/^Name/), 'Robert');
      await user.click(within(dialog).getByRole('checkbox', { name: 'corp' }));
      await user.click(within(dialog).getByRole('checkbox', { name: 'lab' }));
      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

      expect(await screen.findByText('User “Robert” saved')).toBeInTheDocument();
      expect(requestsTo('PUT', /^\/users\//)).toHaveLength(1);
      expect(requestsTo('PUT', /^\/users\//)[0]).toEqual({
        method: 'PUT',
        path: `/users/${bob.id}`,
        body: { name: 'Robert', role: 'user', authorized_networks: [lab.id] },
      });
      expect(await within(row(/Robert/)).findByText('lab')).toBeInTheDocument();
    });

    it('promotes a user to administrator', async () => {
      const { bob } = seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Edit Bob' }));
      const dialog = await findDialog('Edit user Bob');

      await user.selectOptions(within(dialog).getByLabelText('Role'), 'administrator');
      expect(within(dialog).queryByRole('group', { name: 'Authorized networks' })).not.toBeInTheDocument();
      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

      await screen.findByText('User “Bob” saved');
      expect(requestsTo('PUT', /^\/users\//)[0].body).toEqual({ name: 'Bob', role: 'administrator', authorized_networks: [] });
      expect(db.users.find((u) => u.id === bob.id)!.role).toBe('administrator');
      expect(await within(row(/Bob/)).findByText('Administrator')).toBeInTheDocument();
    });

    it('starts from the saved user at each opening, and sends nothing on cancel', async () => {
      seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Edit Bob' }));
      let dialog = await findDialog('Edit user Bob');
      await user.click(within(dialog).getByRole('checkbox', { name: 'corp' }));
      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

      // Another user shows their own networks…
      await user.click(screen.getByRole('button', { name: 'Edit Carol' }));
      dialog = await findDialog('Edit user Carol');
      expect(within(dialog).getByLabelText('Role')).toHaveValue('administrator');
      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

      // …and reopening Bob drops the unsaved change.
      await user.click(screen.getByRole('button', { name: 'Edit Bob' }));
      dialog = await findDialog('Edit user Bob');
      expect(within(dialog).getByRole('checkbox', { name: 'corp' })).toBeChecked();
      expect(requestsTo('PUT', /^\/users\//)).toHaveLength(0);
    });

    it('requires a name', async () => {
      seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Edit Bob' }));
      const dialog = await findDialog('Edit user Bob');
      await user.clear(within(dialog).getByLabelText(/^Name/));

      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

      expect(within(dialog).getByText('Give the user a name.')).toBeInTheDocument();
      expect(within(dialog).getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');
      expect(requestsTo('PUT', /^\/users\//)).toHaveLength(0);
    });

    it('shows the server error in the dialog', async () => {
      seedTeam();
      server.use(http.put('*/api/v1/users/:id', () => HttpResponse.json({ error: 'database is locked' }, { status: 500 })));
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Edit Bob' }));
      const dialog = await findDialog('Edit user Bob');

      await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));

      expect(await within(dialog).findByRole('alert')).toHaveTextContent('database is locked');
      expect(screen.getByRole('dialog', { name: 'Edit user Bob' })).toBeInTheDocument();
    });

    it('protects the role of the signed-in administrator', async () => {
      seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Edit Administrator' }));
      const dialog = await findDialog('Edit user Administrator');
      expect(within(dialog).getByLabelText('Role')).toBeDisabled();
      expect(within(dialog).getByText('You cannot change your own role.')).toBeInTheDocument();
    });
  });

  describe('reset password', () => {
    it('sets a new password after checking its length', async () => {
      const { bob } = seedTeam();
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Reset password for Bob' }));
      const dialog = await findDialog('Reset password for Bob');

      await user.type(within(dialog).getByLabelText(/^New password/), 'short');
      await user.click(within(dialog).getByRole('button', { name: 'Reset password' }));
      expect(within(dialog).getByText('Use at least 8 characters.')).toBeInTheDocument();
      expect(requestsTo('PUT', /^\/users\//)).toHaveLength(0);

      await user.type(within(dialog).getByLabelText(/^New password/), '-but-longer');
      await user.click(within(dialog).getByRole('button', { name: 'Reset password' }));

      expect(await screen.findByText('Password for “Bob” reset')).toBeInTheDocument();
      expect(requestsTo('PUT', /^\/users\//)[0]).toEqual({
        method: 'PUT',
        path: `/users/${bob.id}`,
        body: { password: 'short-but-longer' },
      });
    });

    it('shows the server error in the dialog, and sends nothing on cancel', async () => {
      seedTeam();
      server.use(
        http.put('*/api/v1/users/:id', () =>
          HttpResponse.json({ error: 'password must be at least 8 characters' }, { status: 400 }),
        ),
      );
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Reset password for Bob' }));
      let dialog = await findDialog('Reset password for Bob');
      await user.type(within(dialog).getByLabelText(/^New password/), 'long-enough');
      await user.click(within(dialog).getByRole('button', { name: 'Reset password' }));
      expect(await within(dialog).findByRole('alert')).toHaveTextContent('password must be at least 8 characters');

      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      await user.click(screen.getByRole('button', { name: 'Reset password for Bob' }));
      dialog = await findDialog('Reset password for Bob');
      expect(within(dialog).getByLabelText(/^New password/)).toHaveValue('');
      expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
    });

    it('is not offered for the built-in administrator', async () => {
      seedTeam();
      renderApp('/users');
      await screen.findByRole('table');
      expect(screen.getByRole('button', { name: 'Reset password for Carol' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Reset password for Administrator' })).not.toBeInTheDocument();
    });
  });

  describe('delete user', () => {
    it('deletes a user after confirmation', async () => {
      const { bob } = seedTeam();
      const { user } = renderApp('/users');

      await user.click(await screen.findByRole('button', { name: 'Delete Bob' }));
      let confirm = await findDialog('Delete user “Bob”?');
      expect(within(confirm).getByText(/bob@example\.com will no longer be able to sign in/)).toBeInTheDocument();
      await user.click(within(confirm).getByRole('button', { name: 'Cancel' }));
      expect(requestsTo('DELETE', /^\/users\//)).toHaveLength(0);

      await user.click(screen.getByRole('button', { name: 'Delete Bob' }));
      confirm = await findDialog('Delete user “Bob”?');
      await user.click(within(confirm).getByRole('button', { name: 'Delete user' }));

      expect(await screen.findByText('User “Bob” deleted')).toBeInTheDocument();
      expect(requestsTo('DELETE', /^\/users\//)[0].path).toBe(`/users/${bob.id}`);
      await waitFor(() => expect(screen.queryByRole('row', { name: /Bob/ })).not.toBeInTheDocument());
    });

    it('shows the server error in a toast', async () => {
      seedTeam();
      server.use(http.delete('*/api/v1/users/:id', () => HttpResponse.json({ error: 'user not found' }, { status: 404 })));
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Delete Bob' }));
      const confirm = await findDialog('Delete user “Bob”?');
      await user.click(within(confirm).getByRole('button', { name: 'Delete user' }));

      expect(await screen.findByRole('alert')).toHaveTextContent('user not found');
      expect(row(/Bob/)).toBeInTheDocument();
    });

    it('cannot delete the signed-in or the built-in administrator', async () => {
      seedTeam();
      renderApp('/users');
      await screen.findByRole('table');
      expect(screen.getByRole('button', { name: 'Delete Carol' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Delete Administrator' })).not.toBeInTheDocument();
    });
  });

  describe('default permissions', () => {
    it('loads and saves the role and networks of new users', async () => {
      const { corp, lab } = seedTeam();
      db.defaults = { default_role: 'user', default_authorized_networks: [corp.id] };
      const { user } = renderApp('/users');

      await user.click(await screen.findByRole('button', { name: 'Default permissions' }));
      const dialog = await findDialog('Default permissions for new users');
      expect(await within(dialog).findByRole('checkbox', { name: 'corp' })).toBeChecked();
      expect(within(dialog).getByLabelText('Default role')).toHaveValue('user');
      await user.click(within(dialog).getByRole('checkbox', { name: 'lab' }));

      await user.click(within(dialog).getByRole('button', { name: 'Save' }));

      expect(await screen.findByText('Default permissions saved')).toBeInTheDocument();
      expect(requestsTo('PUT', /^\/users\/defaults$/)[0].body).toEqual({
        default_role: 'user',
        default_authorized_networks: [corp.id, lab.id],
      });
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('gives new administrators no network list', async () => {
      const { corp } = seedTeam();
      db.defaults = { default_role: 'user', default_authorized_networks: [corp.id] };
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Default permissions' }));
      const dialog = await findDialog('Default permissions for new users');
      await user.selectOptions(await within(dialog).findByLabelText('Default role'), 'administrator');
      expect(within(dialog).queryByRole('group', { name: 'Default authorized networks' })).not.toBeInTheDocument();

      await user.click(within(dialog).getByRole('button', { name: 'Save' }));

      await screen.findByText('Default permissions saved');
      expect(requestsTo('PUT', /^\/users\/defaults$/)[0].body).toEqual({
        default_role: 'administrator',
        default_authorized_networks: [],
      });
    });

    it('shows the server error in the dialog, and sends nothing on cancel', async () => {
      seedTeam();
      server.use(http.put('*/api/v1/users/defaults', () => HttpResponse.json({ error: 'invalid role' }, { status: 400 })));
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Default permissions' }));
      let dialog = await findDialog('Default permissions for new users');
      await user.click(await within(dialog).findByRole('button', { name: 'Save' }));
      expect(await within(dialog).findByRole('alert')).toHaveTextContent('invalid role');

      server.resetHandlers();
      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      await user.click(screen.getByRole('button', { name: 'Default permissions' }));
      dialog = await findDialog('Default permissions for new users');
      await user.click(await within(dialog).findByRole('checkbox', { name: 'corp' }));
      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      expect(requestsTo('PUT', /^\/users\/defaults$/)).toHaveLength(0);
    });

    it('cannot save before the current defaults are loaded', async () => {
      seedTeam();
      server.use(http.get('*/api/v1/users/defaults', () => HttpResponse.json({ error: 'storage unavailable' }, { status: 500 })));
      const { user } = renderApp('/users');
      await user.click(await screen.findByRole('button', { name: 'Default permissions' }));
      const dialog = await findDialog('Default permissions for new users');

      expect(await within(dialog).findByRole('alert')).toHaveTextContent('The current defaults could not be loaded: storage unavailable');
      expect(within(dialog).getByRole('button', { name: 'Save' })).toBeDisabled();
    });
  });
});
