import type { SessionInfo } from '@slouchcat/api-client';
import { afterEach, describe, expect, it } from 'vitest';
import { session as baseSession } from '@/test/api';
import { applyProfileAccent } from './session';

const session = (accent?: string): SessionInfo => ({
  ...baseSession,
  profile: { ...baseSession.profile!, accent: accent as never },
});

const rootProperty = (name: string) => document.documentElement.style.getPropertyValue(name);

describe('applyProfileAccent', () => {
  afterEach(() => applyProfileAccent(null));

  it("points the accent properties at the profile's accent tokens", () => {
    applyProfileAccent(session('teal'));
    expect(rootProperty('--sc-accent')).toBe('var(--sc-color-accent-teal-base)');
    expect(rootProperty('--sc-on-accent')).toBe('var(--sc-color-accent-teal-on)');
  });

  it('keeps the global.css defaults for no session, no profile, no accent or an unknown name', () => {
    const noProfile: SessionInfo = { account: baseSession.account };
    for (const value of [null, noProfile, session(), session('magenta'), session('teal-base')]) {
      applyProfileAccent(session('sky'));
      applyProfileAccent(value);
      expect(rootProperty('--sc-accent')).toBe('');
      expect(rootProperty('--sc-on-accent')).toBe('');
    }
  });
});
