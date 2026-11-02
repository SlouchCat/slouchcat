import type { AdminPick } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { phaseOf } from './columns';

// The filter tabs go by the time window only (rule 12): the boundaries of `phaseOf`.

const now = Date.parse('2026-11-02T12:00:00Z');
const pick = (window: Pick<AdminPick, 'startsAt' | 'endsAt'>): AdminPick => ({
  id: 1,
  itemId: '01JABCDEFGHJKMNPQRSTVWXYZ0',
  kind: 'movie',
  title: 'Heat',
  position: 0,
  createdAt: '2026-10-01T10:00:00Z',
  ...window,
});

describe('phaseOf', () => {
  it('a pick without window is active', () => {
    expect(phaseOf(pick({}), now)).toBe('active');
  });

  it('starting now is active, starting later is scheduled', () => {
    expect(phaseOf(pick({ startsAt: '2026-11-02T12:00:00Z' }), now)).toBe('active');
    expect(phaseOf(pick({ startsAt: '2026-11-02T12:00:01Z' }), now)).toBe('scheduled');
  });

  it('ending now is expired, ending later is active', () => {
    expect(phaseOf(pick({ endsAt: '2026-11-02T12:00:00Z' }), now)).toBe('expired');
    expect(phaseOf(pick({ endsAt: '2026-11-02T12:00:01Z' }), now)).toBe('active');
  });

  it('open ends: only a past start or only a later end is active', () => {
    expect(phaseOf(pick({ startsAt: '2020-01-01T00:00:00Z' }), now)).toBe('active');
    expect(phaseOf(pick({ endsAt: '2099-01-01T00:00:00Z' }), now)).toBe('active');
  });

  it('a window wholly past is expired, wholly ahead is scheduled', () => {
    expect(
      phaseOf(pick({ startsAt: '2020-01-01T00:00:00Z', endsAt: '2020-02-01T00:00:00Z' }), now),
    ).toBe('expired');
    expect(
      phaseOf(pick({ startsAt: '2099-01-01T00:00:00Z', endsAt: '2099-02-01T00:00:00Z' }), now),
    ).toBe('scheduled');
  });
});
