import { describe, expect, it } from 'vitest';
import { storedDownload } from '@/test/offlineFixtures';
import { resumePosition } from './playback';

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
