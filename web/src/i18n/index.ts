import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import de from '@/locales/de.json';
import en from '@/locales/en.json';

export const supportedLanguages = ['de', 'en'] as const;
export type Language = (typeof supportedLanguages)[number];
export const defaultLanguage: Language = 'de';

const isSupported = (value: string): value is Language =>
  (supportedLanguages as readonly string[]).includes(value);

/** Picks the first supported language from the browser preferences; German otherwise. */
export function detectLanguage(preferences: readonly string[] = navigator.languages): Language {
  for (const tag of preferences) {
    const base = tag.toLowerCase().split('-')[0] ?? '';
    if (isSupported(base)) return base;
  }
  return defaultLanguage;
}

export const resources = {
  de: { translation: de },
  en: { translation: en },
} as const;

i18n.on('languageChanged', (language) => {
  document.documentElement.lang = language;
});

void i18n.use(initReactI18next).init({
  resources,
  lng: detectLanguage(),
  fallbackLng: 'en',
  supportedLngs: supportedLanguages,
  interpolation: { escapeValue: false },
  initAsync: false,
});

export default i18n;
