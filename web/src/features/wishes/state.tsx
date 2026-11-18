import type { ItemRef } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { ComponentProps } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { Chip } from '@/ui';
import type {
  ExternalTitleImages,
  WishEvent,
  WishFailReason,
  WishProblem,
  WishRequestReason,
  WishState,
  WishStateDetail,
} from './queries';

// What every page of the wishes says about a wish: the state chip with its one tone map, the words
// of a state, a failure and a reason, and the images of a title outside the library. Only the
// server's state is put into words, never what Radarr or Sonarr report (concept 25, 5.3).

type ChipTone = NonNullable<ComponentProps<typeof Chip>['tone']>;

/**
 * How long a tile whose plus went with its request waits for the list to come back before it
 * gives up taking the focus (the search section, the wishes area).
 */
export const REFOCUS_MS = 5000;

/** One tone per state, on every page (preview, search, «Meine Wünsche», administration). */
export const wishTone = {
  queued: 'neutral',
  pending: 'warning',
  processing: 'info',
  partial: 'info',
  available: 'success',
  declined: 'danger',
  failed: 'danger',
  withdrawn: 'outline',
} as const satisfies Record<WishState, ChipTone>;

const isWishState = (value: unknown): value is WishState =>
  typeof value === 'string' && Object.hasOwn(wishTone, value);

/** The open states (`WishStateFilter` `open`); the others are closed. */
export const OPEN_STATES: ReadonlySet<WishState> = new Set([
  'queued',
  'pending',
  'processing',
  'partial',
]);

/** «In Beschaffung · wird eingelesen»: the state, with its detail where the server names one. */
export function wishStateText(t: TFunction, state: WishState, detail?: WishStateDetail): string {
  const text = t(`wishes.state.${state}`);
  return detail ? `${text}${t('format.separator')}${t(`wishes.detail.${detail}`)}` : text;
}

/** The state of a wish as a chip, in the tone of `wishTone`. */
export function WishStateChip({ state, detail }: { state: WishState; detail?: WishStateDetail }) {
  const { t } = useTranslation();
  return <Chip tone={wishTone[state]}>{wishStateText(t, state, detail)}</Chip>;
}

const EVENT_KINDS = [
  'created',
  'submitted',
  'attached',
  'joined',
  'withdrawn',
  'withdraw_deleted',
  'owner_changed',
  'reopened',
] as const;

const isEventKind = (kind: string): kind is (typeof EVENT_KINDS)[number] =>
  EVENT_KINDS.some((known) => known === kind);

const detailOf = (event: WishEvent): Record<string, unknown> => {
  try {
    const parsed: unknown = JSON.parse(event.detail ?? '{}');
    return typeof parsed === 'object' && parsed !== null ? (parsed as Record<string, unknown>) : {};
  } catch {
    return {};
  }
};

/**
 * One step of a wish's history in words: «Freigabe ausstehend» for a change of state, «Staffel 2
 * verfügbar», «Angefordert» …; a kind this client does not know reads «Geändert».
 */
export function eventText(t: TFunction, event: WishEvent): string {
  const detail = detailOf(event);
  if (event.kind === 'state_changed' && isWishState(detail.to)) {
    return t(`wishes.state.${detail.to}`);
  }
  if (event.kind === 'season_available' && Array.isArray(detail.seasons)) {
    return t('wishes.event.season_available', {
      seasons: detail.seasons.join(t('format.listSeparator')),
      count: detail.seasons.length,
    });
  }
  return isEventKind(event.kind) ? t(`wishes.event.${event.kind}`) : t('wishes.event.changed');
}

/** «Staffel 1, 2» of a series wish; empty for a film. */
export const seasonsText = (t: TFunction, seasons: readonly { number: number }[]): string =>
  seasons.length === 0
    ? ''
    : t('wishes.seasons', {
        count: seasons.length,
        list: seasons.map((season) => season.number).join(t('format.listSeparator')),
      });

/** Why a wish failed, in plain words (concept 25, 2). */
export const failText = (t: TFunction, reason: WishFailReason) => t(`wishes.fail.${reason}`);

/** Why the profile cannot request a title now (WishRequestReason). */
export const reasonText = (t: TFunction, reason: WishRequestReason) => t(`wishes.reason.${reason}`);

/** The problem extension of a wish operation (`reason`, `item`, `quota`, `needed`). */
export const wishProblem = (error: unknown): WishProblem | undefined =>
  isApiError(error) && error.problem ? error.problem : undefined;

/**
 * A failed wish operation in the viewer's words: TMDB out of reach reads as a viewer's matter, not
 * as the administration's key field; every other code as `problemMessage` says it.
 */
export function wishProblemMessage(t: TFunction, error: unknown): string {
  if (isApiError(error) && error.code === 'tmdb_unreachable') return t('wishes.tmdbUnreachable');
  return problemMessage(t, error);
}

/** The library title a 409 `already_available` names, if any. */
export const availableItem = (error: unknown): ItemRef | undefined => {
  const problem = wishProblem(error);
  return isApiError(error, 409) && problem?.code === 'already_available' ? problem.item : undefined;
};

/**
 * The images of a title outside the library as the detail head and the cards take them. The one
 * place that reads `ExternalTitleImages`: the server's addresses (never TMDB's) in the sizes of a
 * library title's images, so the head and the cards build their `srcset` alike.
 */
export const externalImages = (images: ExternalTitleImages): ItemRef['images'] => images;

/** The poster in w400 and w200 for a card, undefined without one. */
export const externalPoster = (images: ExternalTitleImages) => ({
  posterUrl: imageUrl(images.poster),
  posterSmallUrl: imageUrl(images.posterSmall),
});
