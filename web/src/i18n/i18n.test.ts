import { describe, expect, it } from 'vitest';
import i18n, { detectLanguage, ensureLanguage, isLanguage, setLanguage } from './index';

describe('languages', () => {
  it('knows the supported languages', () => {
    expect(isLanguage('de')).toBe(true);
    expect(isLanguage('fr')).toBe(false);
    expect(isLanguage(undefined)).toBe(false);
    expect(detectLanguage(['fr-CH', 'en-GB'])).toBe('en');
    expect(detectLanguage(['fr'])).toBe('de');
  });

  it('loads the texts of a language once, on demand, and switches to it', async () => {
    await ensureLanguage('en');
    await ensureLanguage('de');
    expect(i18n.hasResourceBundle('en', 'translation')).toBe(true);
    expect(i18n.hasResourceBundle('de', 'translation')).toBe(true);
    await setLanguage('en');
    expect(i18n.t('home.rows.recent')).toBe('Recently added');
    await setLanguage('de');
    expect(i18n.t('home.rows.recent')).toBe('Neu hinzugefügt');
  });
});
