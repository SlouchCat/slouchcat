import {
  deriveLocalVerifier,
  lockRemainingMs,
  NO_LOCK,
  pinRequirement,
  registerFailure,
  verifyOfflinePin,
} from './pin';
import { getProfile, listDownloads, listProfiles, updateProfile } from './store';
import { offlineSupported } from './support';
import { profilesWithDownloads, withoutPinMaterial } from './sync';
import { OFFLINE_PIN_ITERATIONS } from './types';

/**
 * After a successful online PIN entry (PinDialog): derives the verifier of the normal PIN, but
 * only when the device holds a download the profile may play and the last sync delivered its
 * PIN salt (AN-19). A newer salt (PIN changed meanwhile) keeps it out.
 */
export async function rememberPinVerifier(profileId: number, pin: string): Promise<void> {
  if (!offlineSupported()) return;
  const profile = await getProfile(profileId);
  const salt = profile?.snapshot.pinSalt;
  if (!profile?.snapshot.hasPin || !salt) return;
  const withDownloads = profilesWithDownloads(await listDownloads());
  if (!withDownloads.has(`${profile.accountId}:${profileId}`)) return;
  const verifier = await deriveLocalVerifier(
    pin,
    salt,
    profile.snapshot.pinIterations ?? OFFLINE_PIN_ITERATIONS,
  );
  await updateProfile(profileId, (current) =>
    current?.snapshot.pinSalt === salt ? { ...current, pinVerifier: verifier } : undefined,
  );
}

/**
 * Drops the PIN material (salt, offline PIN, the device's verifier) of every profile that no
 * download on the device names any more (the last one deleted or revoked, AN-19).
 */
export async function dropUnusedPinMaterial(): Promise<void> {
  const withDownloads = profilesWithDownloads(await listDownloads());
  for (const profile of await listProfiles()) {
    const { snapshot } = profile;
    const material = snapshot.pinSalt !== undefined || snapshot.offlinePin !== undefined;
    if (withDownloads.has(`${profile.accountId}:${profile.id}`)) continue;
    if (!material && profile.pinVerifier === undefined) continue;
    await updateProfile(
      profile.id,
      (current) =>
        current && {
          ...current,
          snapshot: withoutPinMaterial(current.snapshot),
          pinVerifier: undefined,
        },
    );
  }
}

export type OpenResult =
  | { ok: true }
  | { ok: false; reason: 'invalid' | 'locked'; retryInMs: number }
  | { ok: false; reason: 'unavailable' };

/**
 * Opens a profile in the offline library: without a PIN at once, else with the offline PIN or
 * the normal PIN through the verifier. Wrong PINs count towards the local brake (pin.ts).
 */
export async function openOfflineProfile(
  profileId: number,
  pin: string,
  now = Date.now(),
): Promise<OpenResult> {
  const profile = await getProfile(profileId);
  if (!profile) return { ok: false, reason: 'unavailable' };
  const requirement = pinRequirement(profile);
  if (requirement === 'none') return { ok: true };
  if (requirement === 'unavailable') return { ok: false, reason: 'unavailable' };
  const waiting = lockRemainingMs(profile.lock, now);
  if (waiting > 0) return { ok: false, reason: 'locked', retryInMs: waiting };
  if (await verifyOfflinePin(profile, pin)) {
    await updateProfile(profileId, (current) => current && { ...current, lock: NO_LOCK });
    return { ok: true };
  }
  const updated = await updateProfile(
    profileId,
    (current) => current && { ...current, lock: registerFailure(current.lock, now) },
  );
  const retryInMs = updated ? lockRemainingMs(updated.lock, now) : 0;
  return { ok: false, reason: retryInMs > 0 ? 'locked' : 'invalid', retryInMs };
}
