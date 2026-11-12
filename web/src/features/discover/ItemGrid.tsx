import type { ItemRef } from '@slouchcat/api-client';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { isAudioKind } from '@/features/catalog/labels';
import { CoverWatched, WATCHED_KINDS } from '@/features/catalog/watched';
import { Notice, PosterCard, SpatialButton, SpatialGroup } from '@/ui';
import { CoverWatchlist, WATCHLIST_KINDS } from './CoverWatchlist';
import styles from './Discover.module.css';

/** Focus key of a title's card in a grid. */
export const cardKey = (prefix: string, item: ItemRef) => `${prefix}-${item.id}`;

/**
 * Cat's note on a pick, else the year, or the author of an audiobook or podcast (as on the start
 * page).
 */
const caption = (item: ItemRef): string | undefined =>
  item.note ??
  (isAudioKind(item.kind) && item.author
    ? item.author
    : item.year !== undefined
      ? String(item.year)
      : undefined);

interface ItemGridProps {
  items: readonly ItemRef[];
  /** Focus key prefix: the grid is `<prefix>`, each card `<prefix>-<id>`. */
  focusKey: string;
  label: string;
  /** Extra controls under a card (the watchlist's "Entfernen"). */
  below?: (item: ItemRef) => ReactNode;
  /**
   * The progress of a started title on each card (library). The check mark «gesehen» is on every
   * card anyway (concept 20 P-5).
   */
  showWatched?: boolean;
  /** Extra class of the grid (the library's column count). */
  className?: string;
  /**
   * A control beside the card's link, over the cover (the library's pen, concept 15 7.4): after
   * the link in the Tab order, outside the arrow-key navigation (PosterCard `tool`).
   */
  tool?: (item: ItemRef) => ReactNode;
  /**
   * The watchlist button on each cover that can carry one (concept 17 D-9, CoverWatchlist): after
   * the pen in the Tab order. On by default, so every grid of titles has it.
   */
  mark?: boolean;
  /** Called once a title was taken off the watchlist from its cover (the watchlist page). */
  onUnmarked?: (item: ItemRef) => void;
  /** Called once the check mark of a title was removed from its cover (the page «Gesehen»). */
  onUnwatched?: (item: ItemRef) => void;
  /**
   * The caption and the bar of a card when the page knows more than the title (the page «Gesehen»:
   * «gesehen am 8. Nov.», «19 von 96 Folgen · zuletzt 9. Nov.»).
   */
  describe?: (item: ItemRef) => CardFacts;
}

/** What a page may say on a card instead of the year and the progress (ItemGrid `describe`). */
export interface CardFacts {
  caption?: string;
  progress?: { percent: number; label: string };
}

interface ItemCellProps {
  item: ItemRef;
  /** Focus key prefix of the grid: the card is `<prefix>-<id>`. */
  focusKey: string;
  showWatched?: boolean;
  tool?: (item: ItemRef) => ReactNode;
  mark?: boolean;
  onUnmarked?: (item: ItemRef) => void;
  onUnwatched?: (item: ItemRef) => void;
  describe?: (item: ItemRef) => CardFacts;
  /** Return false to consume an arrow press (the virtual library grid, concept 17 E-12). */
  onArrowPress?: (direction: string) => boolean;
  /** The caption on one line (the virtual library grid, whose rows have a fixed height). */
  clampCaption?: boolean;
  /** A square cover in a square frame (PosterCard `frame`, the library overview of audio). */
  frame?: 'poster' | 'own';
}

/** One title's poster card in a grid: progress, pen, check mark and watchlist button (ItemGrid). */
export function ItemCell({
  item,
  focusKey,
  showWatched = false,
  tool,
  mark = true,
  onUnmarked,
  onUnwatched,
  describe,
  onArrowPress,
  clampCaption,
  frame,
}: ItemCellProps) {
  const { t } = useTranslation();
  const audio = isAudioKind(item.kind);
  const progress = showWatched ? item.progress : undefined;
  const facts = describe?.(item);
  return (
    <PosterCard
      focusKey={cardKey(focusKey, item)}
      href={paths.item(item.id)}
      title={item.title}
      caption={
        facts?.caption ??
        (item.kind === 'episode' && item.seriesTitle
          ? item.seriesTitle
          : (caption(item) ?? t(`kind.${item.kind}`)))
      }
      clampCaption={clampCaption}
      posterUrl={imageUrl(item.images.poster)}
      posterSmallUrl={imageUrl(item.images.posterSmall)}
      accent={item.accent}
      shape={audio ? 'square' : 'poster'}
      frame={frame}
      progress={
        facts
          ? facts.progress
          : // The running position (a second run too); a finished title shows its check instead.
            progress && !progress.completed && progress.percent > 0
            ? {
                percent: progress.percent,
                label: t(audio ? 'item.listened' : 'item.watched', {
                  percent: Math.round(progress.percent),
                }),
              }
            : undefined
      }
      onArrowPress={onArrowPress}
      tool={tool?.(item)}
      check={
        WATCHED_KINDS.has(item.kind) ? (
          <CoverWatched item={item} onRemoved={onUnwatched && (() => onUnwatched(item))} />
        ) : undefined
      }
      mark={
        mark && WATCHLIST_KINDS.has(item.kind) ? (
          <CoverWatchlist item={item} onRemoved={onUnmarked && (() => onUnmarked(item))} />
        ) : undefined
      }
    />
  );
}

/** Titles as poster cards in a grid; arrow keys move between them (spatial navigation). */
export function ItemGrid({
  items,
  focusKey,
  label,
  below,
  showWatched = false,
  className,
  tool,
  mark = true,
  onUnmarked,
  onUnwatched,
  describe,
}: ItemGridProps) {
  return (
    <SpatialGroup
      as="ul"
      focusKey={focusKey}
      className={className ? `${styles.grid} ${className}` : styles.grid}
      aria-label={label}
    >
      {items.map((item) => (
        <li key={item.id} className={styles.cell}>
          <ItemCell
            item={item}
            focusKey={focusKey}
            showWatched={showWatched}
            tool={tool}
            mark={mark}
            onUnmarked={onUnmarked}
            onUnwatched={onUnwatched}
            describe={describe}
          />
          {below?.(item)}
        </li>
      ))}
    </SpatialGroup>
  );
}

/**
 * A page of a list that could not be read (concept 18 C-3: the lists load as they scroll, without
 * «Mehr laden»): «Erneut versuchen» under the list reads the failed pages again.
 */
export function PageRetry({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <Notice
      tone="danger"
      title={t('library.page.pageFailed')}
      actions={
        <SpatialGroup>
          <SpatialButton variant="secondary" icon="refresh" onPress={onRetry}>
            {t('common.retry')}
          </SpatialButton>
        </SpatialGroup>
      }
    />
  );
}
