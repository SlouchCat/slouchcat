import {
  OFFLINE_PIN_ITERATIONS,
  type LocalPinVerifier,
  type OfflinePinVerifier,
  type PinLock,
  type StoredProfile,
} from './types';

// The offline PIN check (AN-19, declared exception 1 of plan principle 1): the device compares a
// PIN with a PBKDF2-SHA256 verifier the server delivered (separate offline PIN) or one the device
// derived itself after a successful online PIN entry (normal PIN, with the server's salt). The aim
// is a curious child at a shared device, not a manipulated browser.

/** Failures that pass without a pause; the third one starts the brake. */
const FREE_FAILURES = 2;
const FIRST_LOCK_MS = 30_000;
const MAX_LOCK_MS = 15 * 60_000;
const HASH_BYTES = 32;

export function hexToBytes(hex: string): Uint8Array<ArrayBuffer> {
  if (!/^(?:[0-9a-f]{2})*$/i.test(hex)) throw new Error('invalid hex');
  const bytes = new Uint8Array(hex.length / 2);
  for (let index = 0; index < bytes.length; index += 1) {
    bytes[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16);
  }
  return bytes;
}

export function base64urlEncode(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function base64urlDecode(text: string): Uint8Array<ArrayBuffer> {
  const base64 = text.replace(/-/g, '+').replace(/_/g, '/');
  const binary = atob(base64 + '='.repeat((4 - (base64.length % 4)) % 4));
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

/** PBKDF2-SHA256 of the PIN (UTF-8) through WebCrypto. */
export async function pbkdf2(
  pin: string,
  salt: Uint8Array<ArrayBuffer>,
  iterations: number,
  bytes = HASH_BYTES,
): Promise<Uint8Array> {
  const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(pin), 'PBKDF2', false, [
    'deriveBits',
  ]);
  const bits = await crypto.subtle.deriveBits(
    { name: 'PBKDF2', hash: 'SHA-256', salt, iterations },
    key,
    bytes * 8,
  );
  return new Uint8Array(bits);
}

/** Compares without stopping at the first difference. */
function sameBytes(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let difference = 0;
  for (let index = 0; index < a.length; index += 1) difference |= (a[index] ?? 0) ^ (b[index] ?? 0);
  return difference === 0;
}

/** The verifier of the normal PIN: PBKDF2 over the PIN with the server's hex salt (OfflineProfile). */
export async function deriveLocalVerifier(
  pin: string,
  saltHex: string,
  iterations = OFFLINE_PIN_ITERATIONS,
): Promise<LocalPinVerifier> {
  const hash = await pbkdf2(pin, hexToBytes(saltHex), iterations);
  return { salt: saltHex, iterations, hash: base64urlEncode(hash) };
}

/** Whether `pin` matches a verifier of the server (separate offline PIN, base64url salt). */
export async function matchesServerVerifier(
  pin: string,
  verifier: OfflinePinVerifier,
): Promise<boolean> {
  const expected = base64urlDecode(verifier.hash);
  const actual = await pbkdf2(
    pin,
    base64urlDecode(verifier.salt),
    verifier.iterations,
    expected.length,
  );
  return sameBytes(actual, expected);
}

/** Whether `pin` matches the device's verifier of the normal PIN. */
export async function matchesLocalVerifier(
  pin: string,
  verifier: LocalPinVerifier,
): Promise<boolean> {
  const expected = base64urlDecode(verifier.hash);
  const actual = await pbkdf2(pin, hexToBytes(verifier.salt), verifier.iterations, expected.length);
  return sameBytes(actual, expected);
}

/**
 * What opening a profile offline takes: nothing, the separate offline PIN, the normal PIN through
 * the device's verifier, or it is not possible (a PIN without a verifier on this device: the PIN
 * has to be entered online once).
 */
export type PinRequirement = 'none' | 'offlinePin' | 'pin' | 'unavailable';

export function pinRequirement(profile: StoredProfile): PinRequirement {
  const { snapshot, pinVerifier } = profile;
  if (snapshot.offlinePin) return 'offlinePin';
  if (!snapshot.hasPin) return 'none';
  // A new salt means the PIN changed: the old verifier is void.
  if (pinVerifier && snapshot.pinSalt && pinVerifier.salt === snapshot.pinSalt) return 'pin';
  return 'unavailable';
}

/** Checks a PIN offline against the verifier the profile requires. */
export async function verifyOfflinePin(profile: StoredProfile, pin: string): Promise<boolean> {
  switch (pinRequirement(profile)) {
    case 'none':
      return true;
    case 'offlinePin':
      return profile.snapshot.offlinePin
        ? matchesServerVerifier(pin, profile.snapshot.offlinePin)
        : false;
    case 'pin':
      return profile.pinVerifier ? matchesLocalVerifier(pin, profile.pinVerifier) : false;
    case 'unavailable':
      return false;
  }
}

/** Pause after `failures` wrong PINs: none for two, 30 s after the third, doubling, at most 15 min. */
export function lockDelayMs(failures: number): number {
  if (failures <= FREE_FAILURES) return 0;
  return Math.min(MAX_LOCK_MS, FIRST_LOCK_MS * 2 ** (failures - FREE_FAILURES - 1));
}

export const NO_LOCK: PinLock = { failures: 0, until: 0 };

export function registerFailure(lock: PinLock, now: number): PinLock {
  const failures = lock.failures + 1;
  const delay = lockDelayMs(failures);
  return { failures, until: delay > 0 ? now + delay : 0 };
}

/** Milliseconds until the next attempt is accepted (0: now). */
export function lockRemainingMs(lock: PinLock, now: number): number {
  return Math.max(0, lock.until - now);
}
