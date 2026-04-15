---
id: identity-providers
title: Identity Providers
sidebar_position: 4
---

Wirety supports any **standard OIDC provider** as well as **GitHub OAuth** as an identity source. Once configured, users log in through their organisation's IdP; Wirety creates an account on first login and an admin can then assign roles and network access from the UI.

| Provider | Protocol | Requires proxy? |
|----------|----------|-----------------|
| Keycloak | OIDC | No |
| Azure Entra ID | OIDC | No |
| Slack | OIDC | No |
| GitHub | OAuth 2.0 | No |

> Any other standard OIDC provider (Authentik, Zitadel, Okta, Google Workspace, …) works out of the box with the generic configuration.

---

## Generic OIDC setup

All standard OIDC providers share the same four environment variables:

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://your-provider.example.com   # OIDC issuer URL
AUTH_CLIENT_ID=wirety                               # OAuth client / app ID
AUTH_CLIENT_SECRET=your-client-secret
```

The server fetches `{AUTH_ISSUER_URL}/.well-known/openid-configuration` on startup and caches the result for one hour (`AUTH_JWKS_CACHE_TTL`, default `3600`).

The OAuth redirect URI to register in your provider is:

```
https://<your-wirety-domain>/
```

---

## Keycloak

### 1. Create a client

In your realm, go to **Clients → Create client**:

| Field | Value |
|-------|-------|
| Client type | OpenID Connect |
| Client ID | `wirety` (or any name) |
| Client authentication | On (confidential client) |

Under **Valid redirect URIs**, add `https://<your-wirety-domain>/*`.

### 2. Copy the credentials

Open the **Credentials** tab and copy the client secret.

### 3. Configure Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://keycloak.example.com/realms/<your-realm>
AUTH_CLIENT_ID=wirety
AUTH_CLIENT_SECRET=<client-secret>
```

### Notes

- Keycloak includes `email`, `name`, and `sub` in the ID token by default — no extra configuration needed.
- If you want to pre-assign users to the `administrator` role automatically, use a Keycloak client mapper to add a custom claim and manage roles in the Wirety UI after first login.

---

## Azure Entra ID

### 1. Register an application

In the Azure portal, go to **App registrations → New registration**:

| Field | Value |
|-------|-------|
| Supported account types | Accounts in this organisational directory only |
| Redirect URI (Web) | `https://<your-wirety-domain>/` |

### 2. Create a client secret

Go to **Certificates & secrets → New client secret**. Copy the secret value immediately.

### 3. Find your tenant ID

The tenant ID is in **Overview → Directory (tenant) ID**.

### 4. Configure Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://login.microsoftonline.com/<tenant-id>/v2.0
AUTH_CLIENT_ID=<application-client-id>
AUTH_CLIENT_SECRET=<client-secret>
```

### Notes

- Azure Entra ID does not include the `email` claim in the ID token by default. Wirety automatically falls back to the userinfo endpoint and, if needed, the `upn` (user principal name) claim.
- Azure returns `expires_in` as a quoted string (`"3600"`) rather than an integer. Wirety handles this transparently.
- Make sure the application has the **User.Read** delegated permission granted.

---

## Slack

### 1. Create a Slack app

Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App → From scratch**.

### 2. Configure OAuth

Under **OAuth & Permissions**:

- Add redirect URL: `https://<your-wirety-domain>/`
- Add the following **User Token Scopes**: `openid`, `email`, `profile`

### 3. Install the app

Click **Install to Workspace** and authorise. Copy the **Client ID** and **Client Secret** from **Basic Information**.

### 4. Configure Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://slack.com
AUTH_CLIENT_ID=<client-id>
AUTH_CLIENT_SECRET=<client-secret>
```

### Notes

- Slack is a full OIDC provider — no proxy or additional tooling required.
- Slack does not support RP-initiated logout (`end_session_endpoint`). Clicking **Logout** in Wirety invalidates the Wirety session only; the Slack workspace session is unaffected.
- Access is naturally scoped to users who have the app installed in their workspace. If your Slack app is distributed across multiple workspaces, any workspace member can log in — there is no per-workspace restriction enforced at the Wirety level.

---

## GitHub

GitHub is an OAuth 2.0 provider, not OIDC. Wirety handles the differences internally — the configuration and login experience are identical to OIDC providers.

### 1. Create an OAuth App

Go to **GitHub → Settings → Developer settings → OAuth Apps → New OAuth App** (or your organisation's equivalent under **Organization Settings → Developer settings**).

| Field | Value |
|-------|-------|
| Application name | Wirety |
| Homepage URL | `https://<your-wirety-domain>/` |
| Authorization callback URL | `https://<your-wirety-domain>/` |

Copy the **Client ID**. Generate a **Client Secret** and copy it.

### 2. Configure Wirety

```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://github.com
AUTH_CLIENT_ID=<client-id>
AUTH_CLIENT_SECRET=<client-secret>
```

### Notes

- The `AUTH_ISSUER_URL=https://github.com` value is the trigger that activates the GitHub-specific code path. Do not append a path.
- GitHub uses OAuth scopes `read:user` and `user:email` — these are requested automatically.
- If a user's email is set to private on GitHub, Wirety fetches it from the `/user/emails` API endpoint automatically. The user must have at least one verified email address.
- GitHub access tokens do not expire. Wirety sessions created via GitHub last **30 days**, after which the user must log in again.
- GitHub does not support RP-initiated logout. Clicking **Logout** in Wirety invalidates the Wirety session only.
- **Any GitHub user can log in**, regardless of whether the OAuth App is created under a personal account or an organisation account. GitHub org ownership of an OAuth App only controls who *administers* the app — it does not restrict the authorisation page to org members. The authorisation URL is public and there is no org membership assertion in the standard GitHub OAuth flow. For org-level access control, use a proxy such as [Dex](https://dexidp.io) with GitHub as a connector and an `orgs` allowlist.

---

## Role assignment

Regardless of provider, roles are managed within Wirety — the IdP only supplies identity (email, name, unique ID). Role provisioning follows this logic:

1. **First user** to log in becomes an `administrator` automatically.
2. **Subsequent users** receive the default role configured under **Settings → User Defaults** (`user` by default).
3. An administrator can promote or demote any user at any time from **Admin → Users**.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Redirect loop after login | Redirect URI mismatch | Check the URI registered in your provider matches `https://<your-wirety-domain>/` exactly |
| `email claim is empty` error | Provider not sending email | Add `email` scope; for Azure check User.Read permission |
| `Failed to discover OIDC endpoints` | Wrong issuer URL | Verify `AUTH_ISSUER_URL` points to the issuer root (without trailing slash) |
| Clock-related JWT errors | Server time skew | Synchronise the Wirety server clock with NTP |
| GitHub: `empty access token` | Bad client secret | Regenerate and update `AUTH_CLIENT_SECRET` |
| GitHub: no email returned | All GitHub emails are private and unverified | User must verify at least one email address on GitHub |
