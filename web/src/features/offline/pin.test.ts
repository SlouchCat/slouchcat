import { describe, expect, it } from 'vitest';
import { offlineProfile, storedProfile } from '@/test/offlineFixtures';
import {
  base64urlDecode,
  base64urlEncode,
  deriveLocalVerifier,
  hexToBytes,
  lockDelayMs,
  lockRemainingMs,
  matchesLocalVerifier,
  matchesServerVerifier,
  NO_LOCK,
  pbkdf2,
  pinRequirement,
  registerFailure,
  verifyOfflinePin,
} from './pin';

const hex = (bytes: Uint8Array) =>
  [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
const SALT = '00112233445566778899aabbccddeeff';

describe('PBKDF2-SHA256', () => {
  it('matches the published test vector (P="password", S="salt", c=1 and 2)', async () => {
    const salt = new TextEncoder().encode('salt');
    expect(hex(await pbkdf2('password', salt, 1))).toBe(
      '120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b',
    );
    expect(hex(await pbkdf2('password', salt, 2))).toBe(
      'ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43',
    );
  });

  it('encodes hex and base64url without padding', () => {
    expect([...hexToBytes('00ff10')]).toEqual([0, 255, 16]);
    expect(() => hexToBytes('abc')).toThrow();
    const bytes = Uint8Array.from([251, 255, 0, 1]);
    expect(base64urlEncode(bytes)).toBe('-_8AAQ');
    expect([...base64urlDecode('-_8AAQ')]).toEqual([...bytes]);
  });
});

describe('verifiers (AN-19)', () => {
  it('derives the normal PIN’s verifier from the server’s hex salt and checks it', async () => {
    const verifier = await deriveLocalVerifier('4711', SALT, 10);
    expect(verifier).toMatchObject({ salt: SALT, iterations: 10 });
    expect(await matchesLocalVerifier('4711', verifier)).toBe(true);
    expect(await matchesLocalVerifier('4712', verifier)).toBe(false);
  });

  it('checks the separate offline PIN against the server’s verifier', async () => {
    const salt = Uint8Array.from({ length: 16 }, (_, index) => index);
    const hash = await pbkdf2('123456', salt, 10);
    const server = { salt: base64urlEncode(salt), iterations: 10, hash: base64urlEncode(hash) };
    expect(await matchesServerVerifier('123456', server)).toBe(true);
    expect(await matchesServerVerifier('654321', server)).toBe(false);
  });

  it('asks for the offline PIN first, the normal PIN with a current verifier, else locks', async () => {
    const verifier = await deriveLocalVerifier('4711', SALT, 5);
    const none = storedProfile();
    const pin = storedProfile({
      snapshot: offlineProfile({ hasPin: true, pinSalt: SALT }),
      pinVerifier: verifier,
    });
    const stale = storedProfile({
      snapshot: offlineProfile({ hasPin: true, pinSalt: 'f'.repeat(32) }),
      pinVerifier: verifier,
    });
    const without = storedProfile({ snapshot: offlineProfile({ hasPin: true, pinSalt: SALT }) });
    const offline = storedProfile({
      snapshot: offlineProfile({
        hasPin: true,
        pinSalt: SALT,
        offlinePin: { salt: base64urlEncode(hexToBytes(SALT)), iterations: 5, hash: verifier.hash },
      }),
      pinVerifier: verifier,
    });
    expect(pinRequirement(none)).toBe('none');
    expect(pinRequirement(pin)).toBe('pin');
    expect(pinRequirement(stale)).toBe('unavailable');
    expect(pinRequirement(without)).toBe('unavailable');
    expect(pinRequirement(offline)).toBe('offlinePin');

    expect(await verifyOfflinePin(none, '')).toBe(true);
    expect(await verifyOfflinePin(pin, '4711')).toBe(true);
    expect(await verifyOfflinePin(pin, '0000')).toBe(false);
    expect(await verifyOfflinePin(stale, '4711')).toBe(false);
    // The offline verifier has the same salt and hash here: the PIN matches through it.
    expect(await verifyOfflinePin(offline, '4711')).toBe(true);
  });
});

describe('local brake', () => {
  it('pauses 30 s after the third failure, doubling up to 15 min', () => {
    expect([1, 2, 3, 4, 5, 6, 7, 8, 9, 20].map((failures) => lockDelayMs(failures) / 1000)).toEqual(
      [0, 0, 30, 60, 120, 240, 480, 900, 900, 900],
    );
  });

  it('counts failures and names the time left', () => {
    let lock = NO_LOCK;
    lock = registerFailure(lock, 1000);
    lock = registerFailure(lock, 2000);
    expect(lock).toEqual({ failures: 2, until: 0 });
    expect(lockRemainingMs(lock, 2000)).toBe(0);
    lock = registerFailure(lock, 3000);
    expect(lock).toEqual({ failures: 3, until: 33_000 });
    expect(lockRemainingMs(lock, 13_000)).toBe(20_000);
    expect(lockRemainingMs(lock, 40_000)).toBe(0);
  });
});
