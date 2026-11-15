import { describe, expect, it } from 'vitest';
import { detectLanguage, languageName } from './languages';

// The aliases and the base-language match of concept 22 section 4.2 for the languages of stage 4;
// a language without a pack still goes to English.
describe('detectLanguage for the languages of stage 4', () => {
  it.each([
    ['pt', 'pt-BR'],
    ['pt-BR', 'pt-BR'],
    ['pt-PT', 'pt-PT'],
    ['pt-AO', 'pt-BR'],
    ['zh', 'zh-CN'],
    ['zh-Hans', 'zh-CN'],
    ['zh-Hans-CN', 'zh-CN'],
    ['zh-SG', 'zh-CN'],
    ['zh-CN', 'zh-CN'],
    ['zh-Hant', 'zh-TW'],
    ['zh-Hant-TW', 'zh-TW'],
    ['zh-HK', 'zh-TW'],
    ['zh-MO', 'zh-TW'],
    ['zh-TW', 'zh-TW'],
    ['no', 'nb'],
    ['nn', 'nb'],
    ['nb', 'nb'],
    ['nb-NO', 'nb'],
    ['fr-CH', 'fr'],
    ['it-CH', 'it'],
    ['es-MX', 'es'],
    ['nl-BE', 'nl'],
    ['sv-FI', 'sv'],
    ['ja-JP', 'ja'],
    ['ko-KR', 'ko'],
    ['uk-UA', 'uk'],
    ['ru', 'ru'],
    ['tr-TR', 'tr'],
    ['ar', 'en'],
    ['he-IL', 'en'],
  ])('%s → %s', (tag, want) => {
    expect(detectLanguage([tag])).toBe(want);
  });

  it('takes the first preference with a shipped language', () => {
    expect(detectLanguage(['ar', 'fr-CH', 'de-CH'])).toBe('fr');
    expect(detectLanguage(['zh-Hant-HK', 'en'])).toBe('zh-TW');
  });
});

describe('languageName', () => {
  it('names a shipped language in its own language, any other one in the UI language', () => {
    expect(languageName('de', 'en')).toBe('Deutsch (Schweiz)');
    expect(languageName('en', 'de')).toBe('English');
    expect(languageName('ja', 'de')).toBe('日本語');
    expect(languageName('zh-TW', 'fr')).toBe('繁體中文');
    expect(languageName('ar', 'de')).toBe('Arabisch');
    expect(languageName('ar', 'en')).toBe('Arabic');
    expect(languageName('not a code!', 'de')).toBe('not a code!');
  });
});
