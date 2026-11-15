import type { SessionInfo } from '@slouchcat/api-client';
import { afterEach, describe, expect, it } from 'vitest';
import { session as baseSession } from '@/test/api';
import i18n, { setLanguage } from '@/i18n';
import { applyProfileAccent, applyProfileLanguage } from './session';

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

describe('applyProfileLanguage (concept 22 section 4.3)', () => {
  afterEach(async () => {
    await setLanguage('de');
  });

  const withLanguage = (language: string): SessionInfo => ({
    ...baseSession,
    profile: { ...baseSession.profile!, language: language as never },
  });

  it('switches to the language of the profile', async () => {
    await applyProfileLanguage(withLanguage('en'));
    expect(i18n.language).toBe('en');
  });

  it('shows English for a code a newer server knows and this app ships no pack for', async () => {
    for (const code of ['ja', 'de-DE', 'pt-BR', 'xx']) {
      await setLanguage('de');
      await expect(applyProfileLanguage(withLanguage(code))).resolves.toBeUndefined();
      expect(i18n.language).toBe('en');
      expect(document.documentElement.lang).toBe('en');
    }
  });

  it('keeps the language without a profile', async () => {
    await applyProfileLanguage({ account: baseSession.account });
    expect(i18n.language).toBe('de');
  });
});
