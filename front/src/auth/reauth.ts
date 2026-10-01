// Helpers for transparently re-authenticating after the session expires.
//
// When the server session is gone but the identity provider's own session is
// still alive, sending the browser back through the OIDC redirect signs the
// user in again without any prompt. The dashboard then keeps running (and
// keeps showing captive-portal alerts) instead of stopping on the sign-in page.

const RETURN_PATH_KEY = 'auth_return_to';
const AUTO_REAUTH_AT_KEY = 'auth_auto_reauth_at';

// At most one automatic attempt per window: if the identity provider cannot
// sign the user in silently (session gone, error), the next expiry shows the
// sign-in page instead of looping through redirects.
const AUTO_REAUTH_WINDOW_MS = 60_000;

/** Remembers the current page so the sign-in flow can bring the user back to it. */
export function rememberReturnPath(): void {
  const path = window.location.pathname + window.location.search + window.location.hash;
  if (path !== '/' && !path.startsWith('/login') && !path.startsWith('/?')) {
    sessionStorage.setItem(RETURN_PATH_KEY, path);
  }
}

/** Returns (and forgets) the page to go back to after signing in, if any. */
export function takeReturnPath(): string | null {
  const path = sessionStorage.getItem(RETURN_PATH_KEY);
  sessionStorage.removeItem(RETURN_PATH_KEY);
  // Only same-origin absolute paths.
  return path && path.startsWith('/') && !path.startsWith('//') ? path : null;
}

/** Reports whether an automatic re-authentication may run now, and records it. */
export function claimAutoReauth(): boolean {
  const last = Number(sessionStorage.getItem(AUTO_REAUTH_AT_KEY) || 0);
  if (Date.now() - last < AUTO_REAUTH_WINDOW_MS) {
    return false;
  }
  sessionStorage.setItem(AUTO_REAUTH_AT_KEY, String(Date.now()));
  return true;
}
