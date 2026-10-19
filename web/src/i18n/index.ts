import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

export const supportedLanguages = ['de', 'en'] as const;
export type Language = (typeof supportedLanguages)[number];
export const defaultLanguage: Language = 'de';

/** True for a language the UI has texts for. */
export const isLanguage = (value: string | undefined): value is Language =>
  value !== undefined && (supportedLanguages as readonly string[]).includes(value);

/** Picks the first supported language from the browser preferences; German otherwise. */
export function detectLanguage(preferences: readonly string[] = navigator.languages): Language {
  for (const tag of preferences) {
    const base = tag.toLowerCase().split('-')[0] ?? '';
    if (isLanguage(base)) return base;
  }
  return defaultLanguage;
}

// Only the language the browser asks for is part of the start; the other one is fetched when a
// profile (or a form) first needs it.
const loaders: Record<Language, () => Promise<{ default: object }>> = {
  de: () => import('@/locales/de.json'),
  en: () => import('@/locales/en.json'),
};

/** Makes the texts of `language` available (loads them once). */
export async function ensureLanguage(language: Language): Promise<void> {
  if (i18n.hasResourceBundle(language, 'translation')) return;
  const bundle = await loaders[language]();
  i18n.addResourceBundle(language, 'translation', bundle.default, true, true);
}

/** Switches the UI language, loading its texts first. */
export async function setLanguage(language: Language): Promise<void> {
  await ensureLanguage(language);
  if (i18n.resolvedLanguage !== language) await i18n.changeLanguage(language);
}

/** The language the UI shows now, as a supported one (German when none is set yet). */
export function currentLanguage(): Language {
  return isLanguage(i18n.resolvedLanguage) ? i18n.resolvedLanguage : defaultLanguage;
}

i18n.on('languageChanged', (language) => {
  document.documentElement.lang = language;
});

const initial = detectLanguage();
const initialBundle = await loaders[initial]();

await i18n.use(initReactI18next).init({
  resources: { [initial]: { translation: initialBundle.default } },
  lng: initial,
  fallbackLng: 'en',
  supportedLngs: supportedLanguages,
  interpolation: { escapeValue: false },
  initAsync: false,
});

export default i18n;
