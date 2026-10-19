import type { Rating } from '@slouchcat/api-client';
import { accentNames } from '@slouchcat/design-tokens';
import type { TFunction } from 'i18next';
import { supportedLanguages } from '@/i18n';

export const ratings = [0, 6, 12, 16, 18] as const satisfies readonly Rating[];

/** FSK radio options; ratings above `max` (the ceiling of a guest account) are shown but off. */
export function ratingOptions(t: TFunction, max: Rating = 18) {
  return ratings.map((age) => ({
    value: String(age),
    label: t('rating.fsk', { age }),
    isDisabled: age > max,
  }));
}

export const languageOptions = (t: TFunction) =>
  supportedLanguages.map((language) => ({ value: language, label: t(`language.${language}`) }));

export const accentOptions = (t: TFunction) =>
  accentNames.map((name) => ({ value: name, label: t(`accent.${name}`) }));

/** Rating from a radio value; unknown values fall back to the lowest, the safest choice. */
export const ratingOf = (value: string): Rating =>
  ratings.find((age) => String(age) === value) ?? 0;

/**
 * Rating of a title from a radio value, undefined while none (or an unknown one) is chosen. Unlike
 * `ratingOf` (a profile ceiling, where 0 is the safe fallback) there is no fallback: FSK 0 makes a
 * title visible to every profile, so it must be an explicit choice.
 */
export const titleRatingOf = (value: string): Rating | undefined =>
  ratings.find((age) => String(age) === value);
