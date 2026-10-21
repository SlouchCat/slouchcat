import { describe, expect, it } from 'vitest';
import { storedDownload } from '@/test/offlineFixtures';
import { resumePosition, resumeRate } from './playback';

describe('resumePosition (W-5, O-5)', () => {
  const own = (positionMs: number, at: number) => ({ '1': { positionMs, at } });
  const server = (positionMs: number, at: number, completed = false) => ({
    '1': { positionMs, completed, at },
  });

  it('starts at 0 without any position', () => {
    expect(resumePosition(storedDownload(), 1)).toBe(0);
  });

  it('takes the only position there is', () => {
    expect(resumePosition(storedDownload({ positions: own(12_000, 100) }), 1)).toBe(12_000);
    expect(resumePosition(storedDownload({ serverPositions: server(30_000, 100) }), 1)).toBe(
      30_000,
    );
  });

  it('takes the newer of the device’s and the server’s position', () => {
    // Watched online to 30 s after the last offline playback: the server's is newer.
    expect(
      resumePosition(
        storedDownload({ positions: own(12_000, 100), serverPositions: server(30_000, 200) }),
        1,
      ),
    ).toBe(30_000);
    // Played offline after the last sync (the events are not up yet): the device's is newer.
    expect(
      resumePosition(
        storedDownload({ positions: own(45_000, 300), serverPositions: server(30_000, 200) }),
        1,
      ),
    ).toBe(45_000);
  });

  it('starts a title the server has as completed at 0, as online', () => {
    expect(
      resumePosition(
        storedDownload({ positions: own(12_000, 100), serverPositions: server(88_000, 200, true) }),
        1,
      ),
    ).toBe(0);
  });

  it('reads only the positions of the profile that plays', () => {
    const download = storedDownload({
      positions: { '2': { positionMs: 5000, at: 100 } },
      serverPositions: { '2': { positionMs: 7000, completed: false, at: 200 } },
    });
    expect(resumePosition(download, 1)).toBe(0);
    expect(resumePosition(download, 2)).toBe(7000);
  });
});

describe('resumeRate (H-3, M6)', () => {
  const server = (at: number, playbackRate?: number) => ({
    positionMs: 0,
    completed: false,
    at,
    ...(playbackRate !== undefined ? { playbackRate } : {}),
  });

  it('starts at 1 without any speed', () => {
    expect(resumeRate(storedDownload(), 1)).toBe(1);
    expect(resumeRate(storedDownload({ serverPositions: { '1': server(100) } }), 1)).toBe(1);
  });

  it('takes the newer of the device’s and the server’s speed', () => {
    const playbackRates = { '1': { rate: 1.5, at: 200 } };
    expect(resumeRate(storedDownload({ playbackRates }), 1)).toBe(1.5);
    expect(resumeRate(storedDownload({ serverPositions: { '1': server(100, 2) } }), 1)).toBe(2);
    expect(
      resumeRate(storedDownload({ playbackRates, serverPositions: { '1': server(100, 2) } }), 1),
    ).toBe(1.5);
    expect(
      resumeRate(storedDownload({ playbackRates, serverPositions: { '1': server(300, 2) } }), 1),
    ).toBe(2);
    // A server progress without a speed does not reset the device's.
    expect(
      resumeRate(storedDownload({ playbackRates, serverPositions: { '1': server(300) } }), 1),
    ).toBe(1.5);
  });

  it('reads only the speeds of the profile that plays', () => {
    const download = storedDownload({
      playbackRates: { '1': { rate: 2.5, at: 100 } },
      serverPositions: { '2': server(50, 1.25) },
    });
    expect(resumeRate(download, 1)).toBe(2.5);
    expect(resumeRate(download, 2)).toBe(1.25);
    expect(resumeRate(download, 3)).toBe(1);
  });
});
