import { describe, expect, it } from 'vitest';
import { detectLanguage } from './languages';

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
