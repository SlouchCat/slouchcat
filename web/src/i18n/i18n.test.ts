import { afterEach, describe, expect, it, vi } from 'vitest';
import i18n, {
  bundledLanguages,
  currentLanguage,
  ensureLanguage,
  isLanguage,
  languageChain,
  setLanguage,
  shippedLanguageFor,
} from './index';
import { detectLanguage, intlLocale, languages, supportedLanguages } from './languages';
import { pseudoText, setPseudoLanguage } from './pseudo';
import { clearUnlistedLanguage } from './index';

describe('languages', () => {
  it('ships German in two variants and English, named in their own language', () => {
    expect(supportedLanguages).toEqual(['de', 'de-DE', 'en']);
    expect(languages.map((l) => l.name)).toEqual([
      'Deutsch (Schweiz)',
      'Deutsch (Deutschland)',
      'English',
    ]);
    expect(isLanguage('de')).toBe(true);
    expect(isLanguage('de-DE')).toBe(true);
    expect(isLanguage('fr')).toBe(false);
    expect(isLanguage(undefined)).toBe(false);
  });

  it('has a language file for every shipped language', () => {
    expect(bundledLanguages()).toEqual(['de', 'de-DE', 'en']);
  });

  it('formats German in Swiss and every other language in its own locale', () => {
    expect(intlLocale('de')).toBe('de-CH');
    expect(intlLocale('en')).toBe('en');
    expect(intlLocale('de-DE')).toBe('de-DE');
    expect(intlLocale('en-XA')).toBe('en-XA');
    expect(new Intl.NumberFormat(intlLocale('de')).format(1234567.5)).toMatch(
      /^1['’]234['’]567\.5$/,
    );
    expect(new Intl.NumberFormat(intlLocale('de-DE')).format(1234567.5)).toBe('1.234.567,5');
  });
});

describe('detectLanguage (concept 22 section 4.2)', () => {
  it.each([
    [['de-CH'], 'de'],
    [['de-LI'], 'de'],
    // A bare de mostly comes from Germany or Austria (O-6); Swiss browsers send de-CH.
    [['de'], 'de-DE'],
    [['de-AT'], 'de-DE'],
    [['de-LU'], 'de-DE'],
    [['de-BE'], 'de-DE'],
    [['de-DE'], 'de-DE'],
    [['en-GB'], 'en'],
    [['EN_us'], 'en'],
    // Unknown languages go to the next preference, and to English at the end (O-2).
    [['fr-CH', 'en-GB'], 'en'],
    [['fr-CH', 'de-CH'], 'de'],
    [['fr'], 'en'],
    [['ja', 'zh-Hant-TW'], 'en'],
    [[], 'en'],
  ])('%j → %s', (preferences, want) => {
    expect(detectLanguage(preferences)).toBe(want);
  });
});

describe('loading', () => {
  afterEach(async () => {
    clearUnlistedLanguage();
    await setLanguage('de');
  });

  it('loads the whole chain: de-DE, then de, then English', async () => {
    expect(languageChain('de-DE')).toEqual(['de-DE', 'de', 'en']);
    expect(languageChain('de')).toEqual(['de', 'en']);
    expect(languageChain('en')).toEqual(['en']);
    i18n.removeResourceBundle('de-DE', 'translation');
    i18n.removeResourceBundle('en', 'translation');
    await ensureLanguage('de-DE');
    for (const code of ['de-DE', 'de', 'en'])
      expect(i18n.hasResourceBundle(code, 'translation')).toBe(true);
  });

  it('loads English after the first paint when asked to', async () => {
    i18n.removeResourceBundle('en', 'translation');
    const frames: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      frames.push(cb);
      return frames.length;
    });
    await ensureLanguage('de', { englishLater: true });
    expect(i18n.hasResourceBundle('de', 'translation')).toBe(true);
    expect(i18n.hasResourceBundle('en', 'translation')).toBe(false);
    raf.mockRestore();
    frames.forEach((cb) => cb(0));
    await vi.waitFor(() => expect(i18n.hasResourceBundle('en', 'translation')).toBe(true));
  });

  it('two quick switches end on the latest, whatever loads first', async () => {
    await setLanguage('de');
    i18n.removeResourceBundle('en', 'translation');
    // en has to load, de is there: without the counter the older switch to en would end last.
    const toEnglish = setLanguage('en');
    const toGerman = setLanguage('de');
    await Promise.all([toEnglish, toGerman]);
    expect(i18n.language).toBe('de');
  });

  it('switches between the shipped languages', async () => {
    await setLanguage('en');
    expect(i18n.t('home.rows.recent')).toBe('Recently added');
    expect(document.documentElement.lang).toBe('en');
    expect(document.documentElement.dir).toBe('ltr');
    await setLanguage('de');
    expect(i18n.t('home.rows.recent')).toBe('Neu hinzugefügt');
    expect(currentLanguage()).toBe('de');
  });

  it('stands in de for de-DE and English for any other code without a pack', async () => {
    expect(shippedLanguageFor('de-DE')).toBe('de');
    expect(shippedLanguageFor('fr')).toBe('en');
    expect(shippedLanguageFor('de')).toBe('de');
    await setLanguage('de-DE');
    expect(i18n.language).toBe('de');
  });

  it('shows English for a code it ships no pack for, and never throws', async () => {
    await expect(setLanguage('ja')).resolves.toBeUndefined();
    expect(i18n.language).toBe('en');
    expect(currentLanguage()).toBe('en');
  });

  it('a text German lacks comes from English', async () => {
    await ensureLanguage('de');
    i18n.addResource('en', 'translation', 'test.onlyEnglish', 'Only English');
    await setLanguage('de');
    const untyped = i18n.t as unknown as (key: string) => string;
    expect(untyped('test.onlyEnglish')).toBe('Only English');
  });

  it('en-XA lengthens every text by 40 % in brackets and stays while profiles change', async () => {
    expect(pseudoText('Recently added')).toBe('[Recently added~~~~~~]');
    expect(pseudoText('{{count}} titles')).toMatch(/^\[\{\{count\}\} titles~+\]$/);
    await setPseudoLanguage();
    expect(i18n.language).toBe('en-XA');
    expect(i18n.t('home.rows.recent')).toBe('[Recently added~~~~~~]');
    await setLanguage('de');
    expect(i18n.language).toBe('en-XA');
    clearUnlistedLanguage();
    await setLanguage('de');
    expect(i18n.language).toBe('de');
  });
});

describe('{{value, number}} in a text', () => {
  it('formats like intlLocale: Swiss for de, English for en', async () => {
    await setLanguage('de');
    const swiss = new Intl.NumberFormat('de-CH').format(1234567.5);
    expect(i18n.t('item.audio.volume', { series: 'S', volume: 1234567.5 })).toBe(
      `S · Band ${swiss}`,
    );
    await setLanguage('en');
    expect(i18n.t('item.audio.volume', { series: 'S', volume: 1234567.5 })).toContain(
      '1,234,567.5',
    );
    await setLanguage('de');
  });
});
