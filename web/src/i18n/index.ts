import type { Language } from '@slouchcat/api-client';
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import {
  detectLanguage as detectFrom,
  fallbackLanguage,
  intlLocale,
  isLanguage,
} from './languages';

export {
  fallbackLanguage,
  intlLocale,
  isLanguage,
  languageName,
  languages,
  supportedLanguages,
} from './languages';
export type { Language } from '@slouchcat/api-client';

type Bundle = Record<string, unknown>;

// One chunk per language file of src/locales (concept 22 section 4.4): a language pack is a file,
// not a line here. Only the chain of the language that runs is part of the start.
const loaders: Record<string, () => Promise<Bundle>> = Object.fromEntries(
  Object.entries(import.meta.glob<Bundle>('../locales/*.json', { import: 'default' })).map(
    ([path, load]) => [path.replace(/^.*\/(.+)\.json$/, '$1'), load],
  ),
);

/** The codes there is a language file for (de-DE too, though it is not selectable yet). */
export const bundledLanguages = (): string[] => Object.keys(loaders).sort();

/**
 * The order the texts of `language` are looked up in (concept 22 section 4.2): the language, then
 * German for de-DE (an overlay holding only the texts that differ), then English. i18next walks
 * the same order (de-DE, de, then fallbackLng).
 */
export function languageChain(language: string): string[] {
  const chain = [language];
  if (language === 'de-DE') chain.push('de');
  chain.push(fallbackLanguage);
  return [...new Set(chain)];
}

/** Adds the bundle of one language file once; an unknown code adds nothing. */
async function loadBundle(language: string): Promise<void> {
  const load = loaders[language];
  if (!load || i18n.hasResourceBundle(language, 'translation')) return;
  const bundle = await load();
  i18n.addResourceBundle(language, 'translation', bundle, true, true);
}

/** Runs `task` once the browser painted the next frame and is idle. */
function afterFirstPaint(task: () => void): void {
  requestAnimationFrame(() => {
    if (typeof window.requestIdleCallback === 'function') window.requestIdleCallback(task);
    else window.setTimeout(task, 0);
  });
}

interface EnsureOptions {
  /**
   * English, the last link of the chain, comes after the first paint instead of before the texts
   * are used: the start does not wait for a second bundle. Until it is there, a text the chain
   * lacks shows its key.
   */
  englishLater?: boolean;
}

/** Makes the texts of `language` and of its whole chain available (each file loads once). */
export async function ensureLanguage(
  language: string,
  { englishLater = false }: EnsureOptions = {},
): Promise<void> {
  const chain = languageChain(language);
  const now =
    englishLater && language !== fallbackLanguage
      ? chain.filter((code) => code !== fallbackLanguage)
      : chain;
  await Promise.all(now.map(loadBundle));
  if (now.length < chain.length) afterFirstPaint(() => void loadBundle(fallbackLanguage));
}

/** Switches the UI language, loading its texts first. */
async function switchTo(language: string): Promise<void> {
  await ensureLanguage(language, { englishLater: true });
  if (i18n.language !== language) await i18n.changeLanguage(language);
}

// A language that is not shipped, set by setUnlistedLanguage; it outlives every setLanguage.
let pinned: string | undefined;

/**
 * Switches the UI to a shipped language. Any other code (a language a newer server knows, before
 * this app ships its pack) shows English instead of failing (concept 22 section 4.3).
 */
export async function setLanguage(language: string): Promise<void> {
  await switchTo(pinned ?? (isLanguage(language) ? language : fallbackLanguage));
}

/**
 * Switches to a bundle that is not a shipped language and keeps it while profiles change: the
 * pseudo language en-XA of development and Storybook (i18n/pseudo.ts); clearUnlistedLanguage ends it.
 */
export async function setUnlistedLanguage(language: string, bundle: Bundle): Promise<void> {
  if (!i18n.hasResourceBundle(language, 'translation'))
    i18n.addResourceBundle(language, 'translation', bundle, true, true);
  pinned = language;
  await switchTo(language);
}

/** Ends setUnlistedLanguage: setLanguage chooses again. */
export function clearUnlistedLanguage(): void {
  pinned = undefined;
}

/** The language the UI shows now, as a shipped one (English when none is set yet). */
export function currentLanguage(): Language {
  return isLanguage(i18n.language) ? i18n.language : fallbackLanguage;
}

/** The shipped language for the browser's preferences (concept 22 section 4.2). */
export const detectLanguage = (preferences: readonly string[] = navigator.languages): Language =>
  detectFrom(preferences);

i18n.on('languageChanged', (language) => {
  document.documentElement.lang = language;
  // No right-to-left language yet (concept 22 section 4.10); set with the language for when one
  // comes.
  document.documentElement.dir = 'ltr';
});

const initial = detectLanguage();

await i18n.use(initReactI18next).init({
  resources: {},
  lng: initial,
  fallbackLng: fallbackLanguage,
  interpolation: { escapeValue: false },
  initAsync: false,
});
// `{{count, number}}` in a text formats like every Intl call of the app: Swiss for de.
i18n.services.formatter?.addCached('number', (language, options: Intl.NumberFormatOptions) => {
  const format = new Intl.NumberFormat(intlLocale(language ?? initial), options);
  return (value: number) => format.format(value);
});
await switchTo(initial);

export default i18n;
