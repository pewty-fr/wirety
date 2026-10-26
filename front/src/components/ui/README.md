# Wirety UI kit

Every page, modal and form is built from these components, so they all look and
behave the same. The Groups page (`src/pages/groups/`) is the reference
implementation: read it before writing a page.

## Look

- Neutral graphite surfaces, the brand orange (`primary-*`, 500 = `#FF7F2A`)
  only for actions, focus and active states. No gradients.
- Use the semantic tokens, which switch with dark mode — no `dark:` pairs
  needed: `bg-canvas` (page), `bg-surface` (cards, modals), `bg-surface-muted`,
  `bg-surface-hover`, `border-line`, `border-line-strong`, `text-ink`,
  `text-ink-secondary`, `text-ink-muted`, `text-ink-subtle`.
- Inter for text; `font-mono` (JetBrains Mono) for IPs, CIDRs, keys, IDs.
- Icons: `@heroicons/react` — `24/outline` in navigation and empty states,
  `20/solid` (`size-4`) in buttons and rows.
- Sentence case everywhere ("New group", "Save changes"), curly quotes around
  names (“engineering”).

## Pages

```tsx
<Page>
  <PageHeader title="Groups" description="…" actions={<Button icon={<PlusIcon className="size-4" />}>New group</Button>} />
  <NetworkFilter … />                       // list pages filtered by network
  {loading ? <Spinner/> : items.length ? <Table>…</Table> : <EmptyState … />}
</Page>
```

- Create buttons have a text label ("New group"), never an icon alone.
- Row actions are `IconButton`s with a `label` (accessible name + tooltip):
  `label={`Edit ${name}`}`, `label={`Delete ${name}`}` with `tone="danger"`.
- Pages reserved to administrators render `<Alert tone="warning" title="Administrators only">`.

## Forms

- A create/edit dialog is a `FormModal`: title, description, fields, an error
  `Alert` on top, Cancel + submit in the footer, submit disabled with a spinner
  while submitting. One submit applies everything — no per-field Edit/Save.
- Fields: `TextInput`, `Textarea`, `Select` (short fixed lists), `Combobox`
  (searchable: networks, peers, users…), `Checkbox`. Always give a `label`;
  mark `required`; show validation with `error` (shown under the field, sets
  `aria-invalid`). Validate before sending; never use `alert()`.
- Several sections: `Tabs` inside the form (tabs with an invalid field get
  `alert`). Attachments (peers of a group, groups of a policy…) use
  `AttachList`, staged in the form state and applied on submit.
- Submit labels say what happens: "Create group", "Save changes", "Delete peer".
- Don't add a "Close" button to a modal footer: the header's × is already
  named "Close".
- Passwords: `PasswordInput` (show/hide toggle). Icons or buttons inside a
  field: `TextInput`'s `leading` / `trailing`.
- API errors: `apiErrorMessage(err)` (the server's message) in the modal's
  `error`, or a toast for actions outside a form.

## Feedback

- `const toast = useToast()` — `toast.success('Group “x” created')` after a
  mutation, `toast.error(apiErrorMessage(err))` when an action outside a form
  fails.
- `const confirm = useConfirm()` — instead of `window.confirm`:
  `if (await confirm({ title: 'Delete group “x”?', message: '…', confirmLabel: 'Delete group', destructive: true }))`.

## Data

- Read with react-query hooks (`src/hooks/useQueries.ts`: `useNetworks`,
  `useGroups(networkId)`, `usePolicies`, `useRoutes`, `useUsers`,
  `useNetworkPeers`…). After a change, invalidate the family
  (`queryClient.invalidateQueries({ queryKey: ['groups'] })`).
- Mutations call `api.*` (`src/api/client.ts`) — sequentially when the order
  matters — inside `useMutation` or the form's submit handler.
- Use the item's own `network_id` for its API calls, not the page filter
  (which is '' when showing every network).
- Shared validation: `src/utils/validation.ts` (`getCIDRError`, `isIPv4`,
  `isIPv6`, `isIPAddress`); dates: `src/utils/dates.ts`.

## Styling pitfalls

- Classes are joined, not merged: a text colour passed to `TD`, `Button` or
  `IconButton` competes with the component's own and Tailwind's order decides.
  Put the colour on a child element instead.

## Tests

Integration tests render the whole app against an in-memory fake API (MSW):

```tsx
const net = seedNetwork({ name: 'corp' });          // src/test/fakeApi.ts
const { user } = renderApp('/groups');               // src/test/renderApp.tsx
await user.click(await screen.findByRole('button', { name: 'New group' }));
const dialog = await findDialog('New group');        // src/test/helpers.ts
await pick(user, dialog, 'Network', 'corp');         // a Combobox option
await user.type(within(dialog).getByLabelText(/^Name/), 'engineering');
await user.click(within(dialog).getByRole('button', { name: 'Create group' }));
expect(await screen.findByText('Group “engineering” created')).toBeInTheDocument();
expect(requestsTo('POST', /\/groups$/)[0].body).toEqual({ … });   // what the API received
```

Query by role and label (`getByRole`, `getByLabelText`), never by class. Cover,
for each form: the happy path with the exact payload, validation (no request
sent), the server error shown in the dialog, and confirmations (cancel sends
nothing). Override one endpoint for a test with `server.use(http.post(…))`;
`db.signedIn = false` and `db.authConfig` set the sign-in state;
`renderApp(path, { captivePortalAlert: true })` lets the captive-portal alert
show (it is dismissed by default).
Run with `npm test`.
