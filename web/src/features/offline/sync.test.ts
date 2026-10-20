import { describe, expect, it } from 'vitest';
import { alien, heat } from '@/test/fixtures';
import {
  downloadIds,
  downloadState,
  heatDownload,
  manifest,
  offlineProfile,
  rules,
  storedDownload,
  storedProfile,
} from '@/test/offlineFixtures';
import {
  applySync,
  downloadsForProfile,
  FETCH_RETRY_MS,
  fetchOrder,
  heldDownloads,
  mergeDownload,
  offlineProfiles,
  otherAccountDownloads,
  waitsForWifi,
  type SyncInput,
} from './sync';
import type { OfflineSync, QueuedEvent } from './types';

const NOW = Date.parse('2026-10-20T12:00:00Z');

const answer = (overrides: Partial<OfflineSync> = {}): OfflineSync => ({
  serverTime: '2026-10-20T12:00:05Z',
  downloads: [],
  revoked: [],
  profiles: [offlineProfile()],
  rules,
  ...overrides,
});

const input = (overrides: Partial<SyncInput> = {}): SyncInput => ({
  accountId: 1,
  activeProfileId: 1,
  now: NOW,
  downloads: [],
  profiles: [],
  queued: [],
  answer: answer(),
  ...overrides,
});

describe('heldDownloads', () => {
  it('reports the account’s downloads with their complete flag, not pending deletes', () => {
    const held = heldDownloads(
      [
        storedDownload(),
        heatDownload({ complete: false, bytesWritten: 100 }),
        storedDownload({ id: downloadIds.e1, accountId: 2 }),
        storedDownload({ id: downloadIds.e2, pendingDelete: true }),
      ],
      1,
    );
    expect(held).toEqual([
      { id: downloadIds.alien, complete: true },
      { id: downloadIds.heat, complete: false },
    ]);
  });
});

describe('applySync', () => {
  it('removes revoked downloads (AN-4, K-7) and does not store them again', () => {
    const result = applySync(
      input({
        downloads: [storedDownload(), heatDownload()],
        answer: answer({
          revoked: [downloadIds.heat],
          downloads: [downloadState(), downloadState({ id: downloadIds.heat })],
        }),
      }),
    );
    expect(result.remove).toEqual([downloadIds.heat]);
    expect(result.put.map((download) => download.id)).toEqual([downloadIds.alien]);
  });

  it('ignores revoked ids the device does not hold', () => {
    const result = applySync(input({ answer: answer({ revoked: [downloadIds.e1] }) }));
    expect(result.remove).toEqual([]);
  });

  it('takes allowedProfileIds from the server and keeps the device’s file progress', () => {
    const local = storedDownload({ complete: false, bytesWritten: 4000, subtitlesStored: [] });
    const [merged] = applySync(
      input({
        downloads: [local],
        answer: answer({ downloads: [downloadState({ allowedProfileIds: [1, 3] })] }),
      }),
    ).put;
    expect(merged).toMatchObject({
      allowedProfileIds: [1, 3],
      bytesWritten: 4000,
      complete: false,
      manifest: manifest(),
    });
  });

  it('keeps the title data when the answer has none (another profile is active)', () => {
    const merged = mergeDownload(
      storedDownload(),
      downloadState({ item: undefined, allowedProfileIds: [1, 2] }),
      1,
      NOW,
    );
    expect(merged.item?.title).toBe('Alien');
    expect(merged.allowedProfileIds).toEqual([1, 2]);
  });

  it('stores new downloads (created by a rule or another tab) with nothing fetched yet', () => {
    const [created] = applySync(
      input({ answer: answer({ downloads: [downloadState({ id: downloadIds.e1, auto: true })] }) }),
    ).put;
    expect(created).toMatchObject({
      id: downloadIds.e1,
      accountId: 1,
      auto: true,
      bytesWritten: 0,
      complete: false,
      subtitlesStored: [],
    });
  });

  it('lists ready downloads the device lacks for the fetch, oldest first', () => {
    const result = applySync(
      input({
        downloads: [storedDownload()],
        answer: answer({
          downloads: [
            downloadState(),
            downloadState({ id: downloadIds.e2, createdAt: '2026-10-20T11:00:00Z' }),
            downloadState({ id: downloadIds.e1, createdAt: '2026-10-20T10:30:00Z' }),
            downloadState({ id: downloadIds.heat, state: 'preparing' }),
          ],
        }),
      }),
    );
    expect(result.fetch).toEqual([downloadIds.e1, downloadIds.e2]);
  });

  it('clears a stopped fetch when the server has the copy ready again', () => {
    const merged = mergeDownload(
      storedDownload({ complete: false, fetchError: 'evicted' }),
      downloadState({ state: 'ready' }),
      1,
      NOW,
    );
    expect(merged.fetchError).toBeUndefined();
    expect(fetchOrder([merged])).toEqual([downloadIds.alien]);
  });

  it('retries errors of the device only after FETCH_RETRY_MS, not at every sync (W7, K4)', () => {
    for (const code of ['quota', 'storage', 'network', 'range']) {
      const failed = storedDownload({ complete: false, fetchError: code, fetchErrorAt: NOW });
      const merged = mergeDownload(failed, downloadState({ state: 'ready' }), 1, NOW);
      expect(merged.fetchError, code).toBe(code);
      expect(fetchOrder([merged], NOW + FETCH_RETRY_MS - 1), code).toEqual([]);
      expect(fetchOrder([merged], NOW + FETCH_RETRY_MS), code).toEqual([downloadIds.alien]);
    }
  });

  it('clears any error once the server prepared the copy again (new readyAt, O1)', () => {
    const failed = storedDownload({ complete: false, fetchError: 'quota', fetchErrorAt: NOW });
    const merged = mergeDownload(
      failed,
      downloadState({ state: 'ready', readyAt: '2026-10-21T08:00:00Z' }),
      1,
      NOW,
    );
    expect(merged.fetchError).toBeUndefined();
    expect(merged.readyAt).toBe('2026-10-21T08:00:00Z');
  });

  it('keeps where the stored bytes came from (readyAt and ETag of the copy)', () => {
    const local = storedDownload({ fileReadyAt: '2026-10-20T10:05:00Z', fileEtag: '"a-1"' });
    const merged = mergeDownload(local, downloadState(), 1, NOW);
    expect(merged).toMatchObject({ fileReadyAt: '2026-10-20T10:05:00Z', fileEtag: '"a-1"' });
  });

  it('measures the clock offset from serverTime', () => {
    expect(applySync(input()).serverOffsetMs).toBe(5000);
  });

  it('stores the profile snapshots and drops profiles the server no longer lists', () => {
    const result = applySync(
      input({
        profiles: [
          storedProfile(),
          storedProfile({ id: 7, snapshot: offlineProfile({ id: 7, name: 'Alt' }) }),
          storedProfile({ id: 9, accountId: 2, snapshot: offlineProfile({ id: 9 }) }),
        ],
        answer: answer({ profiles: [offlineProfile(), offlineProfile({ id: 2, name: 'Kind' })] }),
      }),
    );
    expect(result.putProfiles.map((profile) => profile.id)).toEqual([1, 2]);
    expect(result.removeProfiles).toEqual([7]);
    expect(result.putProfiles[0]?.rules).toEqual(rules);
    expect(result.putProfiles[1]?.rules).toBeUndefined();
  });

  it('keeps the device’s PIN verifier only while the salt stays the same', () => {
    const verifier = { salt: 'a'.repeat(32), iterations: 1, hash: 'x' };
    const local = storedProfile({ pinVerifier: verifier });
    const same = applySync(
      input({
        downloads: [storedDownload()],
        profiles: [local],
        answer: answer({ profiles: [offlineProfile({ hasPin: true, pinSalt: 'a'.repeat(32) })] }),
      }),
    ).putProfiles[0];
    expect(same?.pinVerifier).toEqual(verifier);
    const changed = applySync(
      input({
        downloads: [storedDownload()],
        profiles: [local],
        answer: answer({ profiles: [offlineProfile({ hasPin: true, pinSalt: 'b'.repeat(32) })] }),
      }),
    ).putProfiles[0];
    expect(changed?.pinVerifier).toBeUndefined();
  });

  it('keeps PIN material only for profiles a download on the device names (AN-19, W4)', () => {
    const verifier = { salt: 'a'.repeat(32), iterations: 1, hash: 'x' };
    const offlinePin = { salt: 'c2FsdA', iterations: 1, hash: 'aGFzaA' };
    const snapshot = offlineProfile({
      hasPin: true,
      pinSalt: 'a'.repeat(32),
      pinIterations: 600_000,
      offlinePin,
    });
    const withDownload = applySync(
      input({
        downloads: [storedDownload()],
        profiles: [storedProfile({ pinVerifier: verifier })],
        answer: answer({ downloads: [downloadState()], profiles: [snapshot] }),
      }),
    ).putProfiles[0];
    expect(withDownload?.snapshot).toMatchObject({ pinSalt: 'a'.repeat(32), offlinePin });
    expect(withDownload?.pinVerifier).toEqual(verifier);
    // The active profile on a device without its downloads: no salt, no offline PIN, no verifier.
    const without = applySync(
      input({
        downloads: [heatDownload()],
        profiles: [storedProfile({ pinVerifier: verifier })],
        answer: answer({ profiles: [snapshot] }),
      }),
    ).putProfiles[0];
    expect(without?.snapshot.hasPin).toBe(true);
    expect(without?.snapshot.pinSalt).toBeUndefined();
    expect(without?.snapshot.pinIterations).toBeUndefined();
    expect(without?.snapshot.offlinePin).toBeUndefined();
    expect(without?.pinVerifier).toBeUndefined();
  });

  it('counts today’s queued play against the snapshot’s remaining watch time', () => {
    const event = (playedMs: number, clientTime: string, profileId = 1): QueuedEvent => ({
      accountId: 1,
      event: {
        profileId,
        itemId: heat.id,
        playbackId: '01JB8Z6Q4M7N9P2R3S5T7V9P01',
        positionMs: 0,
        durationMs: 1000,
        playedMs,
        seq: 1,
        state: 'playing',
        clientTime,
      },
    });
    const [profile] = applySync(
      input({
        queued: [
          event(60_000, '2026-10-20T11:00:00Z'),
          event(30_000, '2026-10-19T20:00:00Z'), // yesterday: booked to that day
          event(45_000, '2026-10-20T11:30:00Z', 2), // another profile
        ],
        answer: answer({
          profiles: [
            offlineProfile({
              watchTime: {
                limitMinutes: 60,
                remainingMinutes: 20,
                resetsAt: '2026-10-21T00:00:00Z',
              },
            }),
          ],
        }),
      }),
    ).putProfiles;
    expect(profile?.watch).toMatchObject({
      limitMinutes: 60,
      remainingMinutes: 20,
      resetsAt: Date.parse('2026-10-21T00:00:00Z'),
      period: -1,
      playedMs: 60_000,
      syncedAt: NOW,
    });
  });
});

describe('looking up what a profile may see (O-7)', () => {
  const downloads = [
    storedDownload(),
    heatDownload(),
    storedDownload({ id: downloadIds.e1, accountId: 2, allowedProfileIds: [1] }),
    storedDownload({ id: downloadIds.e2, pendingDelete: true }),
  ];

  it('shows a profile only the downloads whose allowedProfileIds name it', () => {
    expect(downloadsForProfile(downloads, 1, 1).map((download) => download.id)).toEqual([
      downloadIds.alien,
    ]);
    expect(downloadsForProfile(downloads, 1, 2).map((download) => download.id)).toEqual([
      downloadIds.heat,
    ]);
    expect(downloadsForProfile(downloads, 1, 3)).toEqual([]);
  });

  it('offers only profiles that may play a download on the device', () => {
    const profiles = [
      storedProfile(),
      storedProfile({ id: 2, snapshot: offlineProfile({ id: 2, name: 'Anna', isPrimary: false }) }),
      storedProfile({ id: 3, snapshot: offlineProfile({ id: 3, name: 'Ohne', isPrimary: false }) }),
    ];
    expect(offlineProfiles(profiles, downloads, 1).map((profile) => profile.id)).toEqual([1, 2]);
  });

  it('offers only the profiles of the account last signed in on the device (W2)', () => {
    const profiles = [
      storedProfile(),
      storedProfile({ id: 9, accountId: 2, snapshot: offlineProfile({ id: 9, name: 'Gast' }) }),
    ];
    const both = [
      storedDownload(),
      storedDownload({ id: downloadIds.e1, accountId: 2, allowedProfileIds: [9] }),
    ];
    expect(offlineProfiles(profiles, both, 1).map((profile) => profile.id)).toEqual([1]);
    expect(offlineProfiles(profiles, both, 2).map((profile) => profile.id)).toEqual([9]);
    expect(offlineProfiles(profiles, both, undefined)).toEqual([]);
  });

  it('names the downloads of other accounts for the revocation list (W3)', () => {
    expect(otherAccountDownloads(downloads, 1)).toEqual([downloadIds.e1]);
    expect(otherAccountDownloads(downloads, 2)).toEqual([
      downloadIds.alien,
      downloadIds.heat,
      downloadIds.e2,
    ]);
  });
});

describe('waitsForWifi (AN-13)', () => {
  const wifiRule = storedProfile({ rules: { ...rules, enabled: true, wifiOnly: true } });

  it('holds auto-downloads on cellular while an enabled rule says Wi-Fi only', () => {
    const auto = storedDownload({ auto: true });
    expect(waitsForWifi(auto, [wifiRule], 'cellular')).toBe(true);
    expect(waitsForWifi(auto, [wifiRule], 'wifi')).toBe(false);
    expect(waitsForWifi(auto, [wifiRule], 'unknown')).toBe(false);
  });

  it('never holds a download the viewer started', () => {
    expect(waitsForWifi(storedDownload(), [wifiRule], 'cellular')).toBe(false);
  });

  it('does not hold without such a rule', () => {
    const auto = storedDownload({ auto: true });
    const off = storedProfile({ rules: { ...rules, enabled: true, wifiOnly: false } });
    expect(waitsForWifi(auto, [off], 'cellular')).toBe(false);
  });
});

describe('server progress of the active profile (W-5, O-5)', () => {
  const progress = (positionMs: number, updatedAt: string, completed = false) => ({
    positionMs,
    durationMs: 90_000,
    percent: Math.round((positionMs / 90_000) * 100),
    completed,
    updatedAt,
  });

  it('stores the item’s progress for the active profile, on the device’s clock', () => {
    const [merged] = applySync(
      input({
        downloads: [storedDownload()],
        answer: answer({
          downloads: [
            downloadState({
              item: { ...alien, progress: progress(30_000, '2026-10-20T11:59:00Z') },
            }),
          ],
        }),
      }),
    ).put;
    // serverTime is 5 s ahead of the device (answer()): updatedAt moves 5 s back.
    expect(merged?.serverPositions).toEqual({
      '1': { positionMs: 30_000, completed: false, at: Date.parse('2026-10-20T11:58:55Z') },
    });
    // The device's own position stays as it was.
    expect(merged?.positions).toBeUndefined();
  });

  it('keeps the other profiles’ entries and replaces only the active one', () => {
    const local = storedDownload({
      serverPositions: {
        '1': { positionMs: 1000, completed: false, at: 1 },
        '2': { positionMs: 2000, completed: true, at: 2 },
      },
    });
    const merged = mergeDownload(
      local,
      downloadState({ item: { ...alien, progress: progress(45_000, '2026-10-20T12:00:00Z') } }),
      1,
      NOW,
      { profileId: 1, offsetMs: 0 },
    );
    expect(merged.serverPositions).toEqual({
      '1': { positionMs: 45_000, completed: false, at: Date.parse('2026-10-20T12:00:00Z') },
      '2': { positionMs: 2000, completed: true, at: 2 },
    });
  });

  it('keeps the stored values without the item (another profile is active) or without a profile', () => {
    const serverPositions = { '1': { positionMs: 30_000, completed: false, at: 1 } };
    const local = storedDownload({ serverPositions });
    expect(
      mergeDownload(local, downloadState({ item: undefined }), 1, NOW, {
        profileId: 2,
        offsetMs: 0,
      }).serverPositions,
    ).toEqual(serverPositions);
    // createDownloads merges without a sync profile.
    expect(mergeDownload(local, downloadState(), 1, NOW).serverPositions).toEqual(serverPositions);
  });

  it('drops the active profile’s entry when the server has no progress for it', () => {
    const local = storedDownload({
      serverPositions: { '1': { positionMs: 30_000, completed: false, at: 1 } },
    });
    const merged = mergeDownload(
      local,
      downloadState({ item: { ...alien, progress: undefined } }),
      1,
      NOW,
      { profileId: 1, offsetMs: 0 },
    );
    expect(merged.serverPositions).toBeUndefined();
  });
});
