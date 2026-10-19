import type { CastMember, ExtraKind, ItemRef } from '@slouchcat/api-client';
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
