import { beforeEach, describe, expect, it, vi } from 'vitest';
import { offlineProfile, storedProfile } from '@/test/offlineFixtures';
import { putProfiles, resetOfflineStore, storedProfiles } from '@/test/offlineStore';
import { openOfflineProfile, rememberPinVerifier } from './offlineProfiles';
import { deriveLocalVerifier } from './pin';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/support', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./support')>()),
  offlineSupported: () => true,
}));

const SALT = '0123456789abcdef0123456789abcdef';
const protectedProfile = (overrides = {}) =>
  storedProfile({
    snapshot: offlineProfile({ hasPin: true, pinSalt: SALT, pinIterations: 10 }),
    ...overrides,
  });

beforeEach(() => resetOfflineStore());

describe('rememberPinVerifier', () => {
  it('stores a verifier after an online PIN entry when the sync delivered a salt', async () => {
    await putProfiles([protectedProfile()]);
    await rememberPinVerifier(1, '4711');
    expect(storedProfiles()[0]?.pinVerifier).toEqual(await deriveLocalVerifier('4711', SALT, 10));
  });

  it('stores nothing without a salt (no downloads of the profile here)', async () => {
    await putProfiles([storedProfile({ snapshot: offlineProfile({ hasPin: true }) })]);
    await rememberPinVerifier(1, '4711');
    expect(storedProfiles()[0]?.pinVerifier).toBeUndefined();
    await rememberPinVerifier(99, '4711');
    expect(storedProfiles()).toHaveLength(1);
  });
});

describe('openOfflineProfile', () => {
  it('opens a profile without PIN at once', async () => {
    await putProfiles([storedProfile()]);
    expect(await openOfflineProfile(1, '')).toEqual({ ok: true });
  });

  it('refuses a profile with PIN but without a verifier on this device', async () => {
    await putProfiles([protectedProfile()]);
    expect(await openOfflineProfile(1, '4711')).toEqual({ ok: false, reason: 'unavailable' });
  });

  it('checks the PIN and brakes after the third wrong one (30 s, then doubling)', async () => {
    await putProfiles([
      protectedProfile({ pinVerifier: await deriveLocalVerifier('4711', SALT, 10) }),
    ]);
    const start = 1_000_000;
    expect(await openOfflineProfile(1, '0000', start)).toMatchObject({ reason: 'invalid' });
    expect(await openOfflineProfile(1, '0000', start + 1)).toMatchObject({ reason: 'invalid' });
    expect(await openOfflineProfile(1, '0000', start + 2)).toEqual({
      ok: false,
      reason: 'locked',
      retryInMs: 30_000,
    });
    // Even the right PIN waits for the pause.
    expect(await openOfflineProfile(1, '4711', start + 10_000)).toMatchObject({
      reason: 'locked',
      retryInMs: 20_002,
    });
    expect(await openOfflineProfile(1, '0000', start + 40_000)).toMatchObject({
      reason: 'locked',
      retryInMs: 60_000,
    });
    expect(await openOfflineProfile(1, '4711', start + 100_001)).toEqual({ ok: true });
    expect(storedProfiles()[0]?.lock).toEqual({ failures: 0, until: 0 });
  });
});
