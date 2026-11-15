import type { Rating } from '@slouchcat/api-client';
import { accentNames } from '@slouchcat/design-tokens';
import type { TFunction } from 'i18next';
import { languages } from '@/i18n';

export const ratings = [0, 6, 12, 16, 18] as const satisfies readonly Rating[];

/**
 * The label of an age rating: «FSK 12» for a step of today's table, «ab 13» for any other age. A
 * newer server may store minimum ages outside the FSK steps (concept 22 section 4.11, stage 3);
 * an older app shows them instead of a wrong FSK step.
 */
export function ratingLabel(t: TFunction, age: number): string {
  return (ratings as readonly number[]).includes(age)
    ? t('rating.fsk', { age })
    : t('rating.age', { age });
}

/**
 * FSK radio options; ratings above `max` (the ceiling of a guest account) are shown but off. A
 * `current` value outside the steps (an age a newer server stored) is offered too, so saving the
 * form keeps it instead of turning it into another one.
 */
export function ratingOptions(t: TFunction, max: number = 18, current?: number) {
  const ages: number[] = [...ratings];
  if (current !== undefined && !ages.includes(current)) ages.push(current);
  return ages
    .sort((a, b) => a - b)
    .map((age) => ({ value: String(age), label: ratingLabel(t, age), isDisabled: age > max }));
}

/** The shipped interface languages, each named in its own language (i18n/languages.ts). */
export const languageOptions = () =>
  languages.map((language) => ({ value: language.code, label: language.name }));

export const accentOptions = (t: TFunction) =>
  accentNames.map((name) => ({ value: name, label: t(`accent.${name}`) }));

/**
 * Rating from a radio value: an age from 0 to 18 (the radio offers the steps and the current value,
 * see ratingOptions); anything else falls back to the lowest, the safest choice.
 */
export function ratingOf(value: string): Rating {
  const age = Number(value);
  return /^\d+$/.test(value) && age <= 18 ? (age as Rating) : 0;
}

/**
 * Rating of a title from a radio value, undefined while none (or an unknown one) is chosen. Unlike
 * `ratingOf` (a profile ceiling, where 0 is the safe fallback) there is no fallback: FSK 0 makes a
 * title visible to every profile, so it must be an explicit choice.
 */
export const titleRatingOf = (value: string): Rating | undefined =>
  ratings.find((age) => String(age) === value);
