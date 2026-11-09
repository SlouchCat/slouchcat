import type { CastMember, ExtraKind, HomeRowKey, ItemRef } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';

const pad = (value: number) => String(value).padStart(2, '0');

/** "S01E02", or "S01E03–E04" for a file with several episodes; undefined without numbers. */
export function episodeCode(t: TFunction, item: ItemRef): string | undefined {
  if (item.seasonNumber === undefined || item.episodeNumber === undefined) return undefined;
  const season = pad(item.seasonNumber);
  const episode = pad(item.episodeNumber);
  if (item.episodeEnd !== undefined && item.episodeEnd > item.episodeNumber) {
    return t('item.episodeCodeRange', { season, episode, end: pad(item.episodeEnd) });
  }
  return t('item.episodeCode', { season, episode });
}

/** The names of the start page rows: their headings, and the rows of the admin page «Startseite». */
export const homeRowTitleKey = {
  spotlight: 'home.rows.spotlight',
  picks: 'home.picks',
  continue: 'home.rows.continue',
  recent: 'home.rows.recent',
  released: 'home.rows.released',
  watchlist: 'home.rows.watchlist',
  collections: 'home.rows.collections',
  genres: 'home.rows.genres',
  libraries: 'home.rows.libraries',
  library: 'home.rows.library',
  radio: 'home.rows.radio',
} as const satisfies Record<HomeRowKey, string>;

/**
 * The name of a start page row: «Neu in Filme» for a library row, named by its library (concept 15
 * P-7), the fixed name of every other row.
 */
export const homeRowTitle = (
  t: TFunction,
  row: { key: HomeRowKey; libraryName?: string },
): string => t(homeRowTitleKey[row.key], { library: row.libraryName ?? '' });

/** Audiobooks, podcasts and their episodes (M6): square covers, "gehört", the audio player. */
export const isAudioKind = (kind: ItemRef['kind']): boolean =>
  kind === 'audiobook' || kind === 'podcast' || kind === 'podcast_episode';

/**
 * A finished title for the check mark on its cover: a film, episode or book by its progress, a
 * series or podcast by `watched` (every visible episode completed, contract 1.22.0).
 */
export const isFinished = (item: ItemRef): boolean =>
  item.progress?.completed === true || item.watched === true;

export const extraKindLabel = (t: TFunction, kind: ExtraKind | undefined): string =>
  t(`extraKind.${kind ?? 'other'}`);

/** "als Ellen Ripley" for a role with a character, else the kind of credit ("Regie"). */
export function creditCaption(
  t: TFunction,
  credit: Pick<CastMember, 'role' | 'character'>,
): string | undefined {
  if (credit.character) return t('person.as', { character: credit.character });
  return credit.role === 'cast' ? undefined : t(`item.cast.role.${credit.role}`);
}

/** Display name of a trailer language: the app languages by name, others by code. */
export function languageName(t: TFunction, code: string): string {
  return code === 'de' || code === 'en' ? t(`language.${code}`) : code.toUpperCase();
}
