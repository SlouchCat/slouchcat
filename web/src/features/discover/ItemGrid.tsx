import type { ItemRef } from '@slouchcat/api-client';
import { useEffect, useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { isAudioKind } from '@/features/catalog/labels';
import { PosterCard, setSpatialFocus, SpatialButton, SpatialGroup } from '@/ui';
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
  /** The watch state on each card: a check mark when finished, else the progress (library). */
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
}

interface ItemCellProps {
  item: ItemRef;
  /** Focus key prefix of the grid: the card is `<prefix>-<id>`. */
  focusKey: string;
  showWatched?: boolean;
  tool?: (item: ItemRef) => ReactNode;
  mark?: boolean;
  onUnmarked?: (item: ItemRef) => void;
  /** Return false to consume an arrow press (the virtual library grid, concept 17 E-12). */
  onArrowPress?: (direction: string) => boolean;
  /** The caption on one line (the virtual library grid, whose rows have a fixed height). */
  clampCaption?: boolean;
}

/** One title's poster card in a grid: watch state, pen and watchlist button (ItemGrid). */
export function ItemCell({
  item,
  focusKey,
  showWatched = false,
  tool,
  mark = true,
  onUnmarked,
  onArrowPress,
  clampCaption,
}: ItemCellProps) {
  const { t } = useTranslation();
  const audio = isAudioKind(item.kind);
  const progress = showWatched ? item.progress : undefined;
  return (
    <PosterCard
      focusKey={cardKey(focusKey, item)}
      href={paths.item(item.id)}
      title={item.title}
      caption={
        item.kind === 'episode' && item.seriesTitle
          ? item.seriesTitle
          : (caption(item) ?? t(`kind.${item.kind}`))
      }
      clampCaption={clampCaption}
      posterUrl={imageUrl(item.images.poster)}
      posterSmallUrl={imageUrl(item.images.posterSmall)}
      accent={item.accent}
      shape={audio ? 'square' : 'poster'}
      watchedLabel={
        progress?.completed
          ? t(audio ? 'library.status.heard' : 'library.status.watched')
          : undefined
      }
      progress={
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
          />
          {below?.(item)}
        </li>
      ))}
    </SpatialGroup>
  );
}

interface MoreButtonProps {
  /** Number of entries shown now: the first of the next page gets the focus. */
  shown: number;
  /** Focus keys of all entries, in order (after the next page has arrived). */
  keys: readonly string[];
  hasMore: boolean;
  loading: boolean;
  failed: boolean;
  label: string;
  focusKey: string;
  onMore: () => void;
}

/**
 * "Mehr laden" under a paged list. After a page the focus moves to its first entry (the button
 * may go away with the last page); a failed page turns the button into a retry.
 */
export function MoreButton({
  shown,
  keys,
  hasMore,
  loading,
  failed,
  label,
  focusKey,
  onMore,
}: MoreButtonProps) {
  const { t } = useTranslation();
  const focusFrom = useRef<number | null>(null);
  const joined = keys.join('|');
  useEffect(() => {
    const target = focusFrom.current === null ? undefined : joined.split('|')[focusFrom.current];
    if (!target) return;
    focusFrom.current = null;
    setSpatialFocus(target);
  }, [joined]);
  if (!hasMore && !failed) return null;
  return (
    <SpatialGroup>
      <SpatialButton
        focusKey={focusKey}
        variant="secondary"
        icon={failed ? 'refresh' : 'plus'}
        aria-disabled={loading || undefined}
        onPress={() => {
          if (loading) return;
          focusFrom.current = shown;
          onMore();
        }}
      >
        {failed ? t('common.retry') : label}
      </SpatialButton>
    </SpatialGroup>
  );
}
