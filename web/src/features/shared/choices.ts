import type { Rating, RatingSystem } from '@slouchcat/api-client';
import { accentNames } from '@slouchcat/design-tokens';
import type { TFunction } from 'i18next';
import { languages } from '@/i18n';

/**
 * The ages an app offers without a rating system (an older server that sends none): the FSK
 * steps the server had before concept 22 stage 3. Their labels are «ab n» then.
 */
const FALLBACK_AGES = [0, 6, 12, 16, 18];

/** The age steps of the rating system, lowest first; the FSK ages without one. */
export function ratingAges(system?: RatingSystem): number[] {
  return system ? system.steps.map((step) => step.age) : [...FALLBACK_AGES];
}

/**
 * The label of an age (concept 22 section 4.11): the certificate of the rating country when the
 * age is one of its steps («FSK 12», «PG-13», «-12»), else «ab n». Certificate names come from the
 * server's table (a named exception of rule 9), the words around them from the translations.
 */
export function ratingLabel(t: TFunction, age: number, system?: RatingSystem): string {
  const label = system?.steps.find((step) => step.age === age)?.label;
  return label ?? t('rating.age', { age });
}

/**
 * The label of an age filter («bis FSK 12», «bis PG-13», «bis 13»): titles with a minimum age of
 * at most `age` (section 4.11: a filter «bis 12» means age ≤ 12).
 */
export function ratingFilterLabel(t: TFunction, age: number, system?: RatingSystem): string {
  const label = system?.steps.find((step) => step.age === age)?.label;
  return t('rating.upTo', { rating: label ?? String(age) });
}

/**
 * Rating radio options: the steps of the rating country; ratings above `max` (the ceiling of a
 * guest account) are shown but off. A `current` value outside the steps (a profile from before a
 * change of the rating country, or an age a newer server stored) is offered too, as «ab n», so
 * saving the form keeps it instead of turning it into another one.
 */
export function ratingOptions(
  t: TFunction,
  system: RatingSystem | undefined,
  max: number = 18,
  current?: number,
) {
  const ages = ratingAges(system);
  if (current !== undefined && !ages.includes(current)) ages.push(current);
  return ages
    .sort((a, b) => a - b)
    .map((age) => ({
      value: String(age),
      label: ratingLabel(t, age, system),
      isDisabled: age > max,
    }));
}

/**
 * The rating countries of the server's table as options, named in the interface language by Intl
 * (rule 9: no country name in the code) with the rating body where it has one («Frankreich
 * (CNC)»), in alphabetical order.
 */
export function ratingCountryOptions(
  t: TFunction,
  locale: string,
  systems: readonly RatingSystem[],
): { value: string; label: string }[] {
  const regions = new Intl.DisplayNames([locale], { type: 'region' });
  return systems
    .map((rs) => {
      const country = regions.of(rs.country) ?? rs.country;
      return {
        value: rs.country,
        label: rs.name ? t('rating.system', { country, name: rs.name }) : country,
      };
    })
    .sort((a, b) => a.label.localeCompare(b.label, locale));
}

/** The steps of a rating system in one line («FSK 0, FSK 6, … FSK 18»), as the profiles offer them. */
export const ratingStepsText = (t: TFunction, system: RatingSystem): string =>
  system.steps.map((step) => ratingLabel(t, step.age, system)).join(t('format.separator'));

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
  return /^\d+$/.test(value) && age <= 18 ? age : 0;
}

/**
 * Rating of a title from a radio value, undefined while none (or an unknown one) is chosen. Unlike
 * `ratingOf` (a profile ceiling, where 0 is the safe fallback) there is no fallback: age 0 makes a
 * title visible to every profile, so it must be an explicit choice.
 */
export const titleRatingOf = (value: string): Rating | undefined => {
  const age = Number(value);
  return /^\d+$/.test(value) && age <= 18 ? age : undefined;
};
