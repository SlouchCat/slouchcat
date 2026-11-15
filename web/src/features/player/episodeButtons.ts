import type { ItemRef } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { imageUrl } from '@/api/client';
import { episodeCode } from '@/features/catalog/labels';
import { formatDate, formatRuntime } from '@/lib/format';
import type { PlayerChromeEpisode } from '@/ui/aria';

export type EpisodeDirection = 'previous' | 'next';

/** «S01E05 · Die Brücke», or the title alone (a podcast episode, an episode without numbers). */
export function episodeName(t: TFunction, item: ItemRef): string {
  const code = episodeCode(t, item);
  return code ? t('item.nextEpisodeLabel', { code, title: item.title }) : item.title;
}

const minutesOf = (item: ItemRef) =>
  item.runtimeMinutes ??
  (item.durationMs !== undefined ? Math.round(item.durationMs / 60_000) : undefined);

interface EpisodeButtonOptions {
  direction: EpisodeDirection;
  /** The episode the server named (PlaybackInfo.previousEpisode, nextEpisode); none: greyed. */
  target: ItemRef | undefined;
  /** The title playing: a card over a season boundary names the season. */
  current: ItemRef;
  /** The UI language (intlLocale), for the date of a podcast episode. */
  language: string;
  onPress: () => void;
  /** Why the button is greyed although the server named an episode («Nicht auf diesem Gerät»). */
  unavailable?: string;
  /** The picture of the card; default: the episode's backdrop from the server. */
  cardImageUrl?: string;
}

/**
 * The previous or the next episode button of a player (concept 23, 3.2 and 3.2a): named after the
 * episode the server chose (rule 12: the client never works out which one), with its card; without
 * one greyed with «Keine weitere Folge» or «Keine frühere Folge».
 */
export function episodeButton(
  t: TFunction,
  {
    direction,
    target,
    current,
    language,
    onPress,
    unavailable,
    cardImageUrl,
  }: EpisodeButtonOptions,
): PlayerChromeEpisode {
  const next = direction === 'next';
  if (!target) {
    return {
      label: next ? t('player.chrome.nextEpisode') : t('player.chrome.previousEpisode'),
      onPress,
      unavailable: next ? t('player.chrome.noNextEpisode') : t('player.chrome.noPreviousEpisode'),
    };
  }
  const name = episodeName(t, target);
  // Over a season boundary the card names the season (S01E06 → «Staffel 2 · S02E01 · …»).
  const otherSeason =
    target.seasonNumber !== undefined &&
    target.seasonNumber > 0 &&
    current.seasonNumber !== undefined &&
    target.seasonNumber !== current.seasonNumber;
  const minutes = minutesOf(target);
  const detail =
    [
      target.kind === 'podcast_episode' && target.publishedAt
        ? formatDate(target.publishedAt, language)
        : undefined,
      minutes ? formatRuntime(t, minutes) : undefined,
    ]
      .filter((part): part is string => Boolean(part))
      .join(t('format.separator')) || undefined;
  return {
    label: next
      ? t('player.chrome.nextEpisodeNamed', { name })
      : t('player.chrome.previousEpisodeNamed', { name }),
    onPress,
    unavailable,
    card: {
      imageUrl: cardImageUrl ?? imageUrl(target.images.backdropSmall ?? target.images.backdrop),
      heading: next ? t('player.chrome.nextEpisodeCard') : t('player.chrome.previousEpisodeCard'),
      name: otherSeason
        ? t('player.chrome.cardSeason', { season: target.seasonNumber, name })
        : name,
      detail,
      keys: next ? t('player.help.rows.next.keys') : t('player.help.rows.previous.keys'),
    },
  };
}

/** Whether a title has episode buttons: a series episode or a podcast episode, not a film or book. */
export function hasEpisodeButtons(item: Pick<ItemRef, 'kind'>): boolean {
  return item.kind === 'episode' || item.kind === 'podcast_episode';
}
