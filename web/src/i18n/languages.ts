import type { Language } from '@slouchcat/api-client';

/** One interface language the web app ships (concept 22 section 4.4). */
export interface ShippedLanguage {
  /** The code profiles store; one of the contract's enum Language. */
  code: Language;
  /**
   * The name in the language itself («Deutsch», «English», later «Français», «日本語»). The names are data, not UI
   * text: everybody finds their language in their own script whatever language runs now
   * (CLAUDE.md rule 9 names this file as an exception). Intl.DisplayNames gives no usable names.
   */
  name: string;
  /** The locale every Intl call formats with (de formats Swiss: 1’234’567.5). */
  intlLocale: string;
}

/**
 * The languages the web app ships, in the order of the language choice. The list grows with the
 * language packs and the contract's enum Language together (concept 22 section 4.3); a code
 * enters one release after the app learnt to fall back to English for an unknown one.
 * `web/src/locales/de-DE.json` exists already (stage 1) but is not shipped yet.
 */
export const languages = [
  // «Deutsch (Schweiz)» once German (Germany) is selectable too (stage 2); one German is «Deutsch».
  { code: 'de', name: 'Deutsch', intlLocale: 'de-CH' },
  { code: 'en', name: 'English', intlLocale: 'en' },
] as const satisfies readonly ShippedLanguage[];

type ShippedCode = (typeof languages)[number]['code'];
// Every language of the contract has a pack: a code the contract adds fails to compile here.
const everyContractLanguageShipped: [Exclude<Language, ShippedCode>] extends [never]
  ? true
  : never = true;
void everyContractLanguageShipped;

/** The codes of the shipped languages. */
export const supportedLanguages: readonly Language[] = languages.map((language) => language.code);

/** The language of an unknown code and of a browser that asks for none we ship (concept 22, O-2). */
export const fallbackLanguage: Language = 'en';

/** True for a language the UI ships texts for. */
export const isLanguage = (value: string | undefined): value is Language =>
  value !== undefined && (supportedLanguages as readonly string[]).includes(value);

/**
 * The locale of every Intl call for a UI language: de-CH for de (Swiss numbers and dates), the
 * code itself for every other one. Intl calls take this, never `i18n.language` (an ESLint rule
 * holds that).
 */
export function intlLocale(language: string): string {
  return languages.find((entry) => entry.code === language)?.intlLocale ?? language;
}

/**
 * Codes that name a language under another code (concept 22 section 4.2, step 2 and 3 of the
 * detection), keyed by the lower-case tag or its leading subtags. A bare `de` mostly comes from
 * Germany or Austria (Swiss browsers send de-CH).
 */
const aliases: Record<string, string> = {
  'de-at': 'de-DE',
  'de-lu': 'de-DE',
  'de-be': 'de-DE',
  'de-ch': 'de',
  'de-li': 'de',
  de: 'de-DE',
  pt: 'pt-BR',
  'zh-hant': 'zh-TW',
  'zh-hk': 'zh-TW',
  'zh-mo': 'zh-TW',
  'zh-hans': 'zh-CN',
  'zh-sg': 'zh-CN',
  zh: 'zh-CN',
  no: 'nb',
  nn: 'nb',
};

/**
 * The shipped language of a candidate code, compared without case: the code itself, or, as long
 * as German (Germany) is not shipped (stage 1), German (Switzerland) for it. Undefined otherwise.
 */
function shipped(code: string): Language | undefined {
  const lower = code.toLowerCase();
  const found = supportedLanguages.find((entry) => entry.toLowerCase() === lower);
  if (found) return found;
  return lower === 'de-de' ? shipped('de') : undefined;
}

/**
 * Picks a shipped language from the browser preferences (concept 22 section 4.2). For each tag in
 * order, on ever shorter leading subtags: a known alias first (a bare de is German of Germany,
 * de-AT too, zh-Hant-TW traditional Chinese), else the subtags as a code (pt-BR, then fr for
 * fr-CH). English when no tag gives a shipped language: the detection only proposes shipped ones.
 */
export function detectLanguage(preferences: readonly string[]): Language {
  for (const tag of preferences) {
    const subtags = tag.toLowerCase().replaceAll('_', '-').split('-');
    for (let n = subtags.length; n > 0; n--) {
      const prefix = subtags.slice(0, n).join('-');
      const found = shipped(aliases[prefix] ?? prefix);
      if (found) return found;
    }
  }
  return fallbackLanguage;
}
