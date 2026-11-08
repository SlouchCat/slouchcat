import type { TFunction } from 'i18next';
import { facetValueName } from '@/app/radio/labels';
import type { SelectGroup, SelectOption } from '@/ui';

// The values of the radio's facets the administration picks from (concept 16, 5.4): the keys of
// the server's default rules, joined with the values the catalog and the rules use (a key the
// server added later shows as it is, labels.ts).

export const GENRES = [
  'pop',
  'rock',
  'jazz',
  'classical',
  'electronic',
  'hiphop',
  'schlager',
  'folk',
  'alternative',
  'oldies',
  'chillout',
  'world',
  'country',
  'metal',
  'mixed',
  'blues',
  'soul',
  'rnb',
  'latin',
  'reggae',
  'dance',
  'indie',
  'religious',
  'soundtrack',
  'kids',
];
export const KINDS = ['music', 'news', 'talk', 'sport', 'culture'];
export const MOODS = ['chill', 'party', 'focus', 'sleep', 'workout'];

export type RuleFacet = 'genre' | 'kind' | 'mood';
export const RULE_FACETS: readonly RuleFacet[] = ['genre', 'kind', 'mood'];

const KNOWN: Record<RuleFacet, readonly string[]> = { genre: GENRES, kind: KINDS, mood: MOODS };

/** The values of a facet: the known keys and the extra ones, each once, in that order. */
export const facetValues = (facet: RuleFacet, extra: readonly string[] = []) => [
  ...new Set([...KNOWN[facet], ...extra]),
];

/** The label of a facet («Genre», «Sendeart», «Stimmung», «Region», «Sprache»). */
export const facetLabel = (t: TFunction, facet: RuleFacet | 'region' | 'language') =>
  t(`radio.page.filters.${facet}`);

/** A rule target as the select's value: `genre:pop`, `genre:` (none) or `ignore`. */
export const targetOf = (facet: string, value: string | null | undefined) =>
  facet === 'ignore' ? 'ignore' : `${facet}:${value ?? ''}`;

/** The rule's facet and value from a select's value; null for the empty choice. */
export function parseTarget(
  target: string,
): { facet: RuleFacet | 'ignore'; value: string | null } | null {
  if (target === 'ignore') return { facet: 'ignore', value: null };
  const [facet, value] = target.split(':');
  if (!facet || !RULE_FACETS.includes(facet as RuleFacet)) return null;
  return { facet: facet as RuleFacet, value: value ? value : null };
}

/** The options of one facet («Genre: Pop» …), as a group of a select. */
export const facetGroup = (
  t: TFunction,
  facet: RuleFacet,
  values: readonly string[],
): SelectGroup<string> => ({
  group: facetLabel(t, facet),
  options: values.map((value): SelectOption<string> => ({
    value: targetOf(facet, value),
    label: t('admin.radio.rules.target', {
      facet: facetLabel(t, facet),
      value: facetValueName(t, facet, value),
    }),
  })),
});
