import type { Health } from '@slouchcat/api-client';
import { paths } from '@/app/paths';

/** Poll interval of the restart panel after a restore (design 4: every 2 s). */
export const RESTART_POLL_MS = 2000;

/** How long the restart panel waits for the new server before it says that something is wrong. */
export const RESTART_GIVE_UP_MS = 90_000;

/**
 * Whether `health` comes from a server process that started after the restore was accepted at
 * `acceptedAt` (ms, the same clock as `answeredAt`). The old process may still answer for a moment
 * after the 202; its uptime spans the whole wait, the new one's is shorter.
 */
export const restartedSince = (health: Health, acceptedAt: number, answeredAt: number): boolean =>
  health.uptimeSeconds < (answeredAt - acceptedAt) / 1000;

/**
 * Opens the sign-in page with a full load: the restored database has its own accounts and
 * sessions, so nothing this app still holds in memory is valid any more.
 */
export function reloadToLogin(): void {
  window.location.assign(paths.login);
}
