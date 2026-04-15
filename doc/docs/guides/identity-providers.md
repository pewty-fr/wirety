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
- When using group-based access control with Azure, the `groups` claim contains **object GUIDs** (e.g. `a1b2c3d4-...`), not human-readable names. Set `AUTH_GROUPS_CLAIM=groups` and use the GUID values in `AUTH_ADMIN_GROUP` / `AUTH_USER_GROUP`. You can find the GUIDs in **Azure AD → Groups → your group → Object ID**.

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
- **Any GitHub user can log in by default**, regardless of whether the OAuth App is created under a personal account or an organisation account. GitHub org ownership of an OAuth App only controls who *administers* the app — it does not restrict the authorisation page to org members. To restrict access, use `AUTH_ADMIN_GROUP` and `AUTH_USER_GROUP` (see [Group-based access control](#group-based-access-control) below), or use a proxy such as [Dex](https://dexidp.io) with GitHub as a connector.
- **Group-based access control for GitHub** — set `AUTH_USER_GROUP` and/or `AUTH_ADMIN_GROUP` using org names (`my-org`) or team slugs (`my-org/platform-team`). When groups are configured, Wirety automatically requests the `read:org` scope so it can verify org and team membership. **Known limitation:** group membership changes (adding or removing a user from an org or team) only take effect in Wirety when the user logs in again, because GitHub sessions use opaque tokens rather than refreshable JWTs.

---

## Role assignment

Regardless of provider, roles are managed within Wirety — the IdP only supplies identity (email, name, unique ID). Role provisioning follows this logic:

1. **First user** to log in becomes an `administrator` automatically.
2. **Subsequent users** receive the default role configured under **Settings → User Defaults** (`user` by default).
3. An administrator can promote or demote any user at any time from **Admin → Users**.

---

## Group-based access control

Four optional environment variables let you gate login and auto-assign roles based on group memberships from your IdP:

| Variable | Purpose |
|----------|---------|
| `AUTH_EMAIL_CLAIM` | JWT claim to use as the user's email (default: `email`) |
| `AUTH_GROUPS_CLAIM` | JWT claim carrying group memberships (e.g. `groups`, `roles`) |
| `AUTH_ADMIN_GROUP` | Groups whose members are automatically assigned the `administrator` role |
| `AUTH_USER_GROUP` | Groups required for regular user login — users outside these groups are rejected |

**Rules:**
- A user in both `AUTH_ADMIN_GROUP` and `AUTH_USER_GROUP` is always assigned the `administrator` role.
- When `AUTH_USER_GROUP` is set, users who belong to none of the configured groups receive a generic "You are not authorized" response. The rejection is visible in the audit log with full claim values for debugging.
- Setting `AUTH_USER_GROUP` without `AUTH_ADMIN_GROUP` is a startup error — it would make it impossible to ever create an administrator.
- When group variables are set, the first-user-is-admin shortcut is skipped; role is always derived from live group claims.
- For OIDC providers, group membership is re-evaluated each time the access token is refreshed (typically hourly). For GitHub, membership is evaluated only at login (see the GitHub notes above).

**Example — Keycloak with groups:**
```bash
AUTH_GROUPS_CLAIM=groups
AUTH_ADMIN_GROUP=wirety-admins
AUTH_USER_GROUP=wirety-users
```

**Example — GitHub with org/team:**
```bash
AUTH_ADMIN_GROUP=my-org/platform-infra
AUTH_USER_GROUP=my-org
```

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
