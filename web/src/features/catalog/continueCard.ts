import type { ItemRef } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { formatRuntime } from '@/lib/format';

// The cards of «Weiterschauen» (concept 15 P-1), shared by the start page row and the list behind
// its title (concept 20 P-7): an episode stands as its series, with the series' poster and title and
// «S4 · F7 · Noch 20 min» under it, and its pen edits the series.

/** "Noch 20 min": the time left of a started title. */
export function remaining(t: TFunction, progress: NonNullable<ItemRef['progress']>): string {
  const minutes = Math.max(1, Math.round((progress.durationMs - progress.positionMs) / 60_000));
  return t('home.remaining', { time: formatRuntime(t, minutes) });
}

/**
 * The series a card of «Weiterschauen» shows instead of its episode: its poster and title.
 * Undefined for every other card and for an episode without `seriesRef`, which show the title
 * itself.
 */
export const continueSeries = (item: ItemRef) =>
  item.kind === 'episode' ? item.seriesRef : undefined;

/**
 * The title a card's pen edits (concept 15 P-6): the title of the cover it sits on. On a series card
 * of «Weiterschauen» that is the series, whose poster and title the card shows, not the episode the
 * card opens.
 */
export function continueEdited(item: ItemRef): Pick<ItemRef, 'id' | 'kind' | 'title'> {
  const series = continueSeries(item);
  return series ? { id: series.id, kind: 'series', title: series.title } : item;
}

/**
 * The progress of a started title: a position past 0. A next episode with a progress row at 0
 * (opened, never played) counts as new, without «Noch …» and without a bar.
 */
export const startedProgress = (item: ItemRef) =>
  item.progress && item.progress.positionMs > 0 ? item.progress : undefined;

/** "S4 · F7", or "S4 · F7–F8" for a file with several episodes; undefined without numbers. */
function shortEpisodeCode(t: TFunction, item: ItemRef): string | undefined {
  if (item.seasonNumber === undefined || item.episodeNumber === undefined) return undefined;
  const numbers = { season: item.seasonNumber, episode: item.episodeNumber };
  return item.episodeEnd !== undefined && item.episodeEnd > item.episodeNumber
    ? t('home.continue.episodeRange', { ...numbers, end: item.episodeEnd })
    : t('home.continue.episode', numbers);
}

/**
 * Title, caption and spoken name of a series card (continueSeries): "S4 · F7 · Noch 20 min" under
 * the series title, "S4 · F8 · Neu" for the next episode not started yet; the name adds the
 * episode's own title ("Die Simpsons, S4 · F7, Donuts im Stau, Noch 20 min").
 */
export function seriesCardText(t: TFunction, series: { title: string }, item: ItemRef) {
  const code = shortEpisodeCode(t, item);
  const progress = startedProgress(item);
  const state = progress ? remaining(t, progress) : t('home.continue.new');
  return {
    title: series.title,
    caption: [code, state].filter(Boolean).join(t('format.separator')),
    label: [series.title, code, item.title, state].filter(Boolean).join(t('format.listSeparator')),
  };
}
