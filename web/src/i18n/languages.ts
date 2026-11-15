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
  /** The English name, which the wizard's language step shows below and searches too (data). */
  englishName: string;
  /** The locale every Intl call formats with (de formats Swiss: 1’234’567.5). */
  intlLocale: string;
}

/**
 * The languages the web app ships, in the order of the language choice (concept 22 table 4.1).
 * The list grows with the language packs and the contract's enum Language together (concept 22
 * section 4.3); a code enters one release after the app learnt to fall back to English for an
 * unknown one. German (Germany) is an overlay of German (Switzerland): `web/src/locales/de-DE.json`
 * holds only the texts that read differently in Germany (stage 2). The other twenty came with
 * stage 4; every one formats in its own code (Intl knows nb, pt-BR, zh-TW as they are).
 */
export const languages = [
  {
    code: 'de',
    name: 'Deutsch (Schweiz)',
    englishName: 'German (Switzerland)',
    intlLocale: 'de-CH',
  },
  {
    code: 'de-DE',
    name: 'Deutsch (Deutschland)',
    englishName: 'German (Germany)',
    intlLocale: 'de-DE',
  },
  { code: 'en', name: 'English', englishName: 'English', intlLocale: 'en' },
  { code: 'fr', name: 'Français', englishName: 'French', intlLocale: 'fr' },
  { code: 'es', name: 'Español', englishName: 'Spanish', intlLocale: 'es' },
  { code: 'it', name: 'Italiano', englishName: 'Italian', intlLocale: 'it' },
  { code: 'nl', name: 'Nederlands', englishName: 'Dutch', intlLocale: 'nl' },
  {
    code: 'pt-BR',
    name: 'Português (Brasil)',
    englishName: 'Portuguese (Brazil)',
    intlLocale: 'pt-BR',
  },
  {
    code: 'pt-PT',
    name: 'Português (Portugal)',
    englishName: 'Portuguese (Portugal)',
    intlLocale: 'pt-PT',
  },
  { code: 'pl', name: 'Polski', englishName: 'Polish', intlLocale: 'pl' },
  { code: 'sv', name: 'Svenska', englishName: 'Swedish', intlLocale: 'sv' },
  { code: 'da', name: 'Dansk', englishName: 'Danish', intlLocale: 'da' },
  { code: 'nb', name: 'Norsk bokmål', englishName: 'Norwegian Bokmål', intlLocale: 'nb' },
  { code: 'fi', name: 'Suomi', englishName: 'Finnish', intlLocale: 'fi' },
  { code: 'cs', name: 'Čeština', englishName: 'Czech', intlLocale: 'cs' },
  { code: 'hu', name: 'Magyar', englishName: 'Hungarian', intlLocale: 'hu' },
  { code: 'tr', name: 'Türkçe', englishName: 'Turkish', intlLocale: 'tr' },
  { code: 'ru', name: 'Русский', englishName: 'Russian', intlLocale: 'ru' },
  { code: 'uk', name: 'Українська', englishName: 'Ukrainian', intlLocale: 'uk' },
  { code: 'ja', name: '日本語', englishName: 'Japanese', intlLocale: 'ja' },
  { code: 'ko', name: '한국어', englishName: 'Korean', intlLocale: 'ko' },
  { code: 'zh-CN', name: '简体中文', englishName: 'Chinese (Simplified)', intlLocale: 'zh-CN' },
  { code: 'zh-TW', name: '繁體中文', englishName: 'Chinese (Traditional)', intlLocale: 'zh-TW' },
] as const satisfies readonly ShippedLanguage[];

type ShippedCode = (typeof languages)[number]['code'];
// Every language of the contract has a pack: a code the contract adds fails to compile here.
const everyContractLanguageShipped: [Exclude<Language, ShippedCode>] extends [never]
  ? true
  : never = true;
void everyContractLanguageShipped;

/** The codes of the shipped languages. */
export const supportedLanguages: readonly Language[] = languages.map((language) => language.code);

/** The name of a language in the language itself; an unknown code stands for itself. */
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
 * The name of an interface language for a profile: the own name of a shipped language, else the
 * name Intl gives in the UI language (a code a newer server knows), else the code itself.
 */
export function languageName(code: string, uiLanguage: string): string {
  const shippedName = languages.find((entry) => entry.code === code)?.name;
  if (shippedName) return shippedName;
  try {
    return new Intl.DisplayNames([intlLocale(uiLanguage)], { type: 'language' }).of(code) ?? code;
  } catch {
    return code; // not a well-formed language code
  }
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

/** The shipped language of a candidate code, compared without case; undefined otherwise. */
function shipped(code: string): Language | undefined {
  const lower = code.toLowerCase();
  return supportedLanguages.find((entry) => entry.toLowerCase() === lower);
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
