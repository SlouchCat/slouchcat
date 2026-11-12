import type { ItemRef } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { alien, podcast } from '@/test/fixtures';
import { isFinished } from './labels';

const completed: NonNullable<ItemRef['progress']> = {
  positionMs: 10_200_000,
  durationMs: 10_200_000,
  percent: 100,
  completed: true,
  updatedAt: '2026-10-10T20:00:00Z',
  watchedAt: '2026-10-10T20:00:00Z',
};

const series = (overrides: Partial<ItemRef>): ItemRef => ({
  id: '01JB8Z6Q4M7N9P2R3S5T7V9W4A',
  kind: 'series',
  title: 'Slouchcat Test Show',
  inWatchlist: false,
  images: {},
  ...overrides,
});

describe('isFinished (contract 1.22.0)', () => {
  it('takes a film by watchedAt (contract 1.31.0), not by completed', () => {
    expect(isFinished({ ...alien, progress: completed })).toBe(true);
    // Watched once and started again: still finished.
    const again = { ...completed, completed: false, positionMs: 1000, percent: 1 };
    expect(isFinished({ ...alien, progress: again })).toBe(true);
    // Completed without watchedAt (never finished as far as the server knows): not finished.
    const bare = { ...completed };
    delete bare.watchedAt;
    expect(isFinished({ ...alien, progress: bare })).toBe(false);
    expect(isFinished({ ...alien, progress: undefined })).toBe(false);
  });

  it('takes a series or podcast by watched alone, never by its progress', () => {
    expect(isFinished(series({ watched: true }))).toBe(true);
    expect(isFinished(series({ watched: false, progress: completed }))).toBe(false);
    expect(isFinished(series({ progress: completed }))).toBe(false);
    expect(isFinished({ ...podcast, watched: true })).toBe(true);
    expect(isFinished({ ...podcast, progress: completed })).toBe(false);
  });
});
