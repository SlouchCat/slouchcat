import { describe, expect, it } from 'vitest';
import { detectLanguage, languageName } from './languages';

// Aliases of concept 22 section 4.2 whose target is not shipped yet land on a shipped language or
// English; none of them ever proposes a language without a pack.
describe('detectLanguage aliases before their language ships', () => {
  it.each(['pt', 'pt-PT', 'zh', 'zh-Hans', 'zh-HK', 'no', 'nn', 'nb', 'ko', 'uk'])(
    '%s → en',
    (tag) => {
      expect(detectLanguage([tag])).toBe('en');
    },
  );
});

describe('languageName', () => {
  it('names a shipped language in its own language, any other one in the UI language', () => {
    expect(languageName('de', 'en')).toBe('Deutsch (Schweiz)');
    expect(languageName('en', 'de')).toBe('English');
    expect(languageName('ja', 'de')).toBe('Japanisch');
    expect(languageName('ja', 'en')).toBe('Japanese');
    expect(languageName('not a code!', 'de')).toBe('not a code!');
  });
});
