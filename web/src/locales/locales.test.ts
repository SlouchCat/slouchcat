import { describe, expect, it } from 'vitest';
import { supportedLanguages } from '@/i18n/languages';
import de from './de.json';

// The language files of the web app (concept 22 section 4.9); de is the reference. de, de-DE (the
// overlay of German (Germany)) and en are checked strictly: every pull request carries their texts.
// For the other shipped languages a missing text or a text de no longer has is a warning in main
// (section 4.8: they are completed before each release, `task i18n:check`); a broken structure
// (plural forms, placeholders, tags, empty texts) fails for every language.

type Texts = Record<string, string>;

function flatten(value: unknown, prefix = '', out: Texts = {}): Texts {
  if (typeof value === 'string') out[prefix] = value;
  else if (typeof value === 'object' && value !== null)
    for (const [key, child] of Object.entries(value))
      flatten(child, prefix ? `${prefix}.${key}` : key, out);
  return out;
}

const files = import.meta.glob<unknown>('./*.json', { eager: true, import: 'default' });
const texts = (code: string): Texts => {
  const file = files[`./${code}.json`];
  if (!file) throw new Error(`no ${code}.json`);
  return flatten(file);
};

const PLURAL = /_(zero|one|two|few|many|other)$/;
/** The key without its plural suffix. */
const baseOf = (key: string) => key.replace(PLURAL, '');

/** The {{placeholders}} and <tags> of a text, sorted, each once. */
function markers(text: string): string[] {
  const found = [
    ...[...text.matchAll(/\{\{\s*([\w.]+)[^}]*\}\}/g)].map((m) => `{{${m[1]}}}`),
    ...[...text.matchAll(/<\/?(\w+)\s*\/?>/g)].map((m) => `<${m[1]}>`),
  ];
  return [...new Set(found)].sort();
}

const reference = flatten(de);

/** The languages whose files must be complete in every commit. */
const STRICT = new Set<string>(['de', 'de-DE', 'en']);

describe.each(supportedLanguages)('locales/%s.json', (code) => {
  const own = texts(code);

  it('has every text of de.json, apart from the plural forms of its language', () => {
    // German (Germany) is an overlay: it holds only what reads differently (section 4.2).
    if (code === 'de-DE') return;
    const bases = (t: Texts) => [...new Set(Object.keys(t).map(baseOf))].sort();
    if (STRICT.has(code)) {
      expect(bases(own)).toEqual(bases(reference));
      return;
    }
    const ownBases = new Set(bases(own));
    const missing = bases(reference).filter((base) => !ownBases.has(base));
    if (missing.length > 0)
      console.warn(
        `${code}: ${missing.length} texts missing (before the release: task i18n:check)`,
      );
  });

  it('has exactly the plural forms Intl.PluralRules names for the language', () => {
    const categories = new Intl.PluralRules(code).resolvedOptions().pluralCategories;
    const forms = new Map<string, Set<string>>();
    for (const key of Object.keys(own)) {
      const match = PLURAL.exec(key);
      if (!match || match[1] === 'zero') continue;
      const base = baseOf(key);
      forms.set(base, (forms.get(base) ?? new Set()).add(match[1]!));
    }
    for (const [base, found] of forms) {
      // de and en keep the singular under the bare key (admin.home.browse.hanging next to
      // hanging_other); i18next takes it where `_one` is missing, so it counts as `one`.
      if (own[base] !== undefined) found.add('one');
      const want = [...categories];
      expect([...found].sort(), base).toEqual(want.sort());
    }
  });

  it('keeps the placeholders and tags of every text, and no text is empty', () => {
    const stale: string[] = [];
    for (const [key, text] of Object.entries(own)) {
      expect(text.trim(), key).not.toBe('');
      const source = reference[key] ?? reference[`${baseOf(key)}_other`] ?? reference[baseOf(key)];
      if (source === undefined && !STRICT.has(code)) {
        stale.push(key); // a text de dropped; i18next never asks for it
        continue;
      }
      expect(source, `${key} has no German source`).toBeDefined();
      // A plural form may leave out {{count}} ("Ein Titel"), the other markers stay.
      const strip = (list: string[]) => list.filter((m) => m !== '{{count}}');
      expect(strip(markers(text)), key).toEqual(strip(markers(source!)));
    }
    if (stale.length > 0)
      console.warn(`${code}: ${stale.length} texts de.json no longer has: ${stale.join(', ')}`);
  });

  it('flags texts identical to English (a warning: «Trailer» is right in many languages)', () => {
    const language: string = code;
    if (language === 'de' || language === 'en') return;
    const english = texts('en');
    const same = Object.keys(own).filter((key) => own[key] === english[key]);
    if (same.length > 0) console.warn(`${language}: ${same.length} texts identical to English`);
  });
});

describe('German', () => {
  it('de.json uses Swiss spelling (no ß)', () => {
    expect(JSON.stringify(de)).not.toContain('ß');
  });

  it('de-DE.json only overlays texts of de.json, and every one of them differs', () => {
    const germany = texts('de-DE');
    expect(Object.keys(germany).length).toBeGreaterThan(150);
    for (const [key, text] of Object.entries(germany)) {
      expect(reference[key], `${key} is not in de.json`).toBeDefined();
      expect(text, `${key} is the same as in de.json`).not.toBe(reference[key]);
      expect(markers(text), key).toEqual(markers(reference[key]!));
      expect(text, key).not.toMatch(/[«»]/);
    }
  });

  it('de-DE.json keeps the ss of short vowels (dass, lässt, Passwort, Schlüssel)', () => {
    const all = Object.values(texts('de-DE')).join('\n');
    expect(all).not.toMatch(/daß|läßt|Paßwort|Schlüßel|muß\b|Adreße/);
  });
});
