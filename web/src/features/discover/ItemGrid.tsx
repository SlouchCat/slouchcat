import type { ItemRef } from '@slouchcat/api-client';
import { useEffect, useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { isAudioKind } from '@/features/catalog/labels';
import { PosterCard, setSpatialFocus, SpatialButton, SpatialGroup } from '@/ui';
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
  /** The age rating on each card that has one (library of films and series). */
  showRating?: boolean;
  /** Extra class of the grid (the library's column count). */
  className?: string;
}

/** Titles as poster cards in a grid; arrow keys move between them (spatial navigation). */
export function ItemGrid({
  items,
  focusKey,
  label,
  below,
  showWatched = false,
  showRating = false,
  className,
}: ItemGridProps) {
  const { t } = useTranslation();
  return (
    <SpatialGroup
      as="ul"
      focusKey={focusKey}
      className={className ? `${styles.grid} ${className}` : styles.grid}
      aria-label={label}
    >
      {items.map((item) => {
        const audio = isAudioKind(item.kind);
        const progress = showWatched ? item.progress : undefined;
        return (
          <li key={item.id} className={styles.cell}>
            <PosterCard
              focusKey={cardKey(focusKey, item)}
              href={paths.item(item.id)}
              title={item.title}
              caption={
                item.kind === 'episode' && item.seriesTitle
                  ? item.seriesTitle
                  : (caption(item) ?? t(`kind.${item.kind}`))
              }
              posterUrl={imageUrl(item.images.poster)}
              posterSmallUrl={imageUrl(item.images.posterSmall)}
              accent={item.accent}
              shape={audio ? 'square' : 'poster'}
              badge={
                showRating && item.rating !== undefined
                  ? t('rating.fsk', { age: item.rating })
                  : undefined
              }
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
            />
            {below?.(item)}
          </li>
        );
      })}
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
