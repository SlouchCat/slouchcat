import { ensureLanguage, setUnlistedLanguage } from './index';
import i18n from './index';

/**
 * The pseudo language of development and Storybook (concept 22 section 4.4): English, every text
 * in brackets and 40 % longer, so a text that a longer language would cut off shows before a real
 * translation meets it. en-XA is a valid locale code: Intl formats it as English instead of
 * throwing. It is never shipped, never in the contract and never offered to a profile.
 */
export const PSEUDO_LANGUAGE = 'en-XA';

type Texts = { [key: string]: string | Texts };

/** One text lengthened by 40 % and put in brackets; placeholders and tags stay intact. */
export function pseudoText(text: string): string {
  return `[${text}${'~'.repeat(Math.ceil(text.length * 0.4))}]`;
}

/** Every text of a bundle through pseudoText, keys unchanged. */
export function pseudoBundle(texts: Texts): Texts {
  return Object.fromEntries(
    Object.entries(texts).map(([key, value]) => [
      key,
      typeof value === 'string' ? pseudoText(value) : pseudoBundle(value),
    ]),
  );
}

/** Switches the UI to en-XA, built from the English texts. */
export async function setPseudoLanguage(): Promise<void> {
  await ensureLanguage('en');
  const english = i18n.getResourceBundle('en', 'translation') as Texts;
  await setUnlistedLanguage(PSEUDO_LANGUAGE, pseudoBundle(english));
}
