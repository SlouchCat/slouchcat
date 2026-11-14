import type { CollectionRef } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { CollectionSize, CollectionSort } from './collectionsParams';

// What the cards and the page of a collection say about it (concept 21 P-10): the facts come from
// the server (years, runtime, seenCount, started); the page only arranges them.

type Facts = Pick<CollectionRef, 'count' | 'firstYear' | 'lastYear' | 'seenCount' | 'started'>;

/** «2001–2011», one year when both are the same, undefined without a year. */
export function collectionYears(t: TFunction, facts: Pick<Facts, 'firstYear' | 'lastYear'>) {
  const { firstYear: first, lastYear: last } = facts;
  if (first === undefined && last === undefined) return undefined;
  if (first === undefined || last === undefined || first === last) return String(first ?? last);
  return t('collection.years', { first, last });
}

/** The line under a card: «8 Titel · 2001–2011». */
export function collectionCaption(t: TFunction, facts: Facts): string {
  return [t('collection.count', { count: facts.count }), collectionYears(t, facts)]
    .filter(Boolean)
    .join(t('format.separator'));
}

/**
 * The bar on the cover of a started collection: the share of its members seen, at least a sliver
 * so a collection with a part only begun shows that it is started.
 */
export function collectionProgress(t: TFunction, facts: Facts) {
  if (!facts.started) return undefined;
  const seen = facts.seenCount ?? 0;
  return {
    percent: Math.max(4, facts.count > 0 ? (seen / facts.count) * 100 : 0),
    label: t('collection.seen', { count: seen, total: facts.count }),
  };
}

/** The smallest count of members of each size chip («Alle», «Ab 2 Titeln», «Ab 3», «Ab 5»). */
export const SIZE_MIN: Record<CollectionSize, number> = { all: 1, '2': 2, '3': 3, '5': 5 };

/**
 * The overview's list (concept 21 P-10, variant A): the collections whose name holds the search
 * text (case and accents aside), with at least the size's members, by name, by count (then name)
 * or started first (then name). The server sends every collection in one answer.
 */
export function arrangeCollections(
  collections: readonly CollectionRef[],
  { q, size, sort }: { q: string; size: CollectionSize; sort: CollectionSort },
  language: string,
): CollectionRef[] {
  const collator = new Intl.Collator(language, { sensitivity: 'base' });
  const fold = (text: string) =>
    text
      .normalize('NFD')
      .replace(/\p{Diacritic}/gu, '')
      .toLocaleLowerCase(language);
  const needle = fold(q.trim());
  const min = SIZE_MIN[size];
  const byName = (a: CollectionRef, b: CollectionRef) => collator.compare(a.name, b.name);
  const order: Record<CollectionSort, (a: CollectionRef, b: CollectionRef) => number> = {
    name: byName,
    count: (a, b) => b.count - a.count || byName(a, b),
    started: (a, b) => Number(b.started === true) - Number(a.started === true) || byName(a, b),
  };
  return collections
    .filter((c) => c.count >= min && (needle === '' || fold(c.name).includes(needle)))
    .sort(order[sort]);
}
