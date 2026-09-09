import { useEffect, useRef } from 'react';

/**
 * useAttention draws the user's attention when `active` becomes (and stays)
 * true — used to signal that a device needs captive-portal sign-in.
 *
 * Two signals, both best-effort:
 *
 *  1. A short beep on the false → true transition. Browsers block audio until
 *     the page has had a user gesture, so this may be silently suppressed on the
 *     very first alert; we never throw.
 *
 *  2. While `active` is true AND the tab is in the background, the browser tab
 *     title flashes between its real title and `message`. This is the reliable
 *     signal for a user who has the dashboard open in another tab. We only flash
 *     when hidden (no point flashing a tab the user is already looking at — the
 *     modal is right there), and we always restore the original title on
 *     cleanup, when the tab regains focus, or when `active` goes false.
 */
export function useAttention(active: boolean, message = '🔴 Action required'): void {
  const prevActive = useRef(false);

  // One-shot beep on the inactive → active transition.
  useEffect(() => {
    if (active && !prevActive.current) {
      playBeep();
    }
    prevActive.current = active;
  }, [active]);

  // Tab-title flashing while active and the tab is hidden.
  useEffect(() => {
    if (!active) return;

    const original = document.title;
    let showingMessage = false;
    let intervalId: number | undefined;

    const startFlashing = () => {
      if (intervalId !== undefined) return;
      intervalId = window.setInterval(() => {
        document.title = showingMessage ? original : message;
        showingMessage = !showingMessage;
      }, 1000);
    };
    const stopFlashing = () => {
      if (intervalId !== undefined) {
        window.clearInterval(intervalId);
        intervalId = undefined;
      }
      document.title = original;
    };
    const onVisibilityChange = () => {
      if (document.hidden) startFlashing();
      else stopFlashing();
    };

    if (document.hidden) startFlashing();
    document.addEventListener('visibilitychange', onVisibilityChange);

    return () => {
      document.removeEventListener('visibilitychange', onVisibilityChange);
      stopFlashing();
    };
  }, [active, message]);
}

// playBeep emits a short sine "ding" via the Web Audio API. No asset needed.
// Fully guarded: missing API, a suspended context (autoplay policy), or any
// runtime error is swallowed — the beep is a nice-to-have, never a requirement.
function playBeep(): void {
  try {
    const AudioCtor =
      window.AudioContext ||
      (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!AudioCtor) return;

    const ctx = new AudioCtor();
    void ctx.resume?.(); // best-effort; no-op / rejects silently if blocked

    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = 'sine';
    osc.frequency.value = 880; // A5
    const t = ctx.currentTime;
    gain.gain.setValueAtTime(0.0001, t);
    gain.gain.exponentialRampToValueAtTime(0.15, t + 0.02);
    gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.35);
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.start(t);
    osc.stop(t + 0.36);
    osc.onended = () => {
      try {
        void ctx.close();
      } catch {
        /* ignore */
      }
    };
  } catch {
    /* audio unavailable or blocked — ignore */
  }
}
