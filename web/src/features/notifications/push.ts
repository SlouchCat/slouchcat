import type { PushSubscriptionRequest } from '@slouchcat/api-client';
import { deviceName, type UserAgentData } from '@/features/player/capabilities';
import { base64UrlToBytes } from '@/lib/base64url';

// The browser side of Web Push (M8, A-6): permission, the service worker's PushManager and the
// subscription the server sends to. The worker shows the messages (public/push-sw.js). iPhone and
// iPad offer PushManager in the installed app only (untested for lack of a device, AN-7).

/** Why this device could not be subscribed; the page shows a sentence per reason. */
export type PushProblem = 'unsupported' | 'denied' | 'noWorker' | 'failed';

export class PushError extends Error {
  readonly problem: PushProblem;

  constructor(problem: PushProblem, cause?: unknown) {
    super(`push: ${problem}`, { cause });
    this.name = 'PushError';
    this.problem = problem;
  }
}

/** How long the page waits for the service worker (it registers once the browser is idle). */
const WORKER_TIMEOUT_MS = 10_000;

/** Service worker, PushManager and notifications: a secure context with a current browser. */
export function pushSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.isSecureContext === true &&
    'serviceWorker' in navigator &&
    'PushManager' in window &&
    'Notification' in window
  );
}

function workerReady(): Promise<ServiceWorkerRegistration> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => reject(new PushError('noWorker')), WORKER_TIMEOUT_MS);
  });
  return Promise.race([navigator.serviceWorker.ready, timeout]).finally(() => clearTimeout(timer));
}

/** Same bytes: a subscription made with an older VAPID key (rotated by the admin) is useless. */
function sameKey(a: ArrayBuffer | null | undefined, b: Uint8Array): boolean {
  if (!a || a.byteLength !== b.byteLength) return false;
  const bytes = new Uint8Array(a);
  return bytes.every((byte, index) => byte === b[index]);
}

/** "Chrome 156 · Linux" for the list of devices on the settings page. */
function thisDeviceName(): string | undefined {
  const nav = navigator as Navigator & { userAgentData?: UserAgentData };
  return deviceName({ userAgent: nav.userAgent, userAgentData: nav.userAgentData ?? null });
}

/**
 * Asks for permission and subscribes this browser with the server's VAPID key (`publicKey` from
 * GET /push/vapid-key, or a function that fetches it once permission is granted); returns what
 * POST /me/push-subscriptions wants. The permission prompt comes first, while the click still
 * counts as a user gesture (Firefox asks only then).
 */
export async function subscribeThisDevice(
  publicKey: () => Promise<string>,
): Promise<PushSubscriptionRequest> {
  if (!pushSupported()) throw new PushError('unsupported');
  const permission = await Notification.requestPermission();
  if (permission !== 'granted') throw new PushError('denied');
  const key = base64UrlToBytes(await publicKey());
  const registration = await workerReady();
  let subscription: PushSubscription;
  try {
    const existing = await registration.pushManager.getSubscription();
    if (existing && !sameKey(existing.options.applicationServerKey, key)) {
      await existing.unsubscribe();
    }
    subscription =
      existing && sameKey(existing.options.applicationServerKey, key)
        ? existing
        : await registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: key,
          });
  } catch (error) {
    throw new PushError('failed', error);
  }
  const json = subscription.toJSON();
  const p256dh = json.keys?.p256dh;
  const auth = json.keys?.auth;
  if (!json.endpoint || !p256dh || !auth) throw new PushError('failed');
  const name = thisDeviceName();
  return { endpoint: json.endpoint, keys: { p256dh, auth }, ...(name ? { deviceName: name } : {}) };
}

/**
 * Ends this browser's subscription at its push service, so it stops receiving even what the
 * server sent before the removal. Best effort: no worker or no subscription is fine.
 */
export async function unsubscribeThisDevice(): Promise<void> {
  if (!pushSupported()) return;
  try {
    const registration = await navigator.serviceWorker.getRegistration();
    await (await registration?.pushManager.getSubscription())?.unsubscribe();
  } catch {
    // The server forgets the subscription anyway; the push service drops it with time.
  }
}
