import type { AdminPodcast } from '@slouchcat/api-client';

/** The filters of the list (12.2): every podcast, those that store, link, load, or have an error. */
export const PODCAST_FILTERS = ['all', 'save', 'link', 'loading', 'error'] as const;
export type PodcastFilter = (typeof PODCAST_FILTERS)[number];

/** The orders of the list: by title, newest episode, stored bytes, number of episodes. */
export const PODCAST_SORTS = ['title', 'newest', 'size', 'episodes'] as const;
export type PodcastSort = (typeof PODCAST_SORTS)[number];

/** Whether a podcast belongs to a filter, from what the server sent about it. */
export function matchesFilter(podcast: AdminPodcast, filter: PodcastFilter): boolean {
  switch (filter) {
    case 'save':
      return podcast.effectiveMode === 'save';
    case 'link':
      return podcast.effectiveMode === 'link';
    case 'loading':
      return podcast.downloads.queued + podcast.downloads.running > 0;
    case 'error':
      return Boolean(podcast.lastError) || podcast.downloads.failed > 0;
    default:
      return true;
  }
}

/** Whether a podcast's title or author contains the search text (case and accents aside). */
export function matchesSearch(podcast: AdminPodcast, search: string, language: string): boolean {
  const needle = fold(search, language);
  if (!needle) return true;
  return fold(`${podcast.title} ${podcast.author ?? ''}`, language).includes(needle);
}

const fold = (text: string, language: string) =>
  text.trim().toLocaleLowerCase(language).normalize('NFD').replace(/\p{M}/gu, '');

/**
 * The podcasts in an order. The sort is stable: podcasts that compare equal keep the server's
 * order (newest subscription first). A title compares without leading signs («{ungeskriptet}»
 * sorts under U).
 */
export function sortPodcasts(
  podcasts: readonly AdminPodcast[],
  sort: PodcastSort,
  collator: Intl.Collator,
): AdminPodcast[] {
  const key = (title: string) => title.replace(/^[^\p{L}\p{N}]+/u, '');
  const compare: Record<PodcastSort, (a: AdminPodcast, b: AdminPodcast) => number> = {
    title: (a, b) => collator.compare(key(a.title), key(b.title)),
    newest: (a, b) => (b.latestEpisodeAt ?? '').localeCompare(a.latestEpisodeAt ?? ''),
    size: (a, b) => b.storedBytes - a.storedBytes,
    episodes: (a, b) => b.episodeCount - a.episodeCount,
  };
  return [...podcasts].sort(compare[sort]);
}
