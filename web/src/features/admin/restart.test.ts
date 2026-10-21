import { describe, expect, it } from 'vitest';
import { restartedSince } from './restart';

const health = (uptimeSeconds: number) => ({
  status: 'ok' as const,
  version: '0.9.0',
  uptimeSeconds,
});

describe('restartedSince', () => {
  it('is false for the old process, whose uptime spans the whole wait', () => {
    // Accepted at 10 s; the old process ran for an hour before.
    expect(restartedSince(health(3604), 10_000, 14_000)).toBe(false);
    expect(restartedSince(health(4), 10_000, 14_000)).toBe(false);
  });

  it('is true for a process that started after the restore was accepted', () => {
    expect(restartedSince(health(1), 10_000, 14_000)).toBe(true);
    expect(restartedSince(health(0), 10_000, 12_500)).toBe(true);
  });
});
