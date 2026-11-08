import type { ItemRef } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useOnline } from '@/lib/useOnline';
import { Button } from '@/ui';
import { useWatchlistMutation, useWatchlistPending } from './queries';
import styles from './CoverWatchlist.module.css';

/**
 * The kinds with the watchlist button on their cover (concept 17 D-9): not seasons, episodes,
 * extras and podcast episodes. Pass the button as `PosterCard` `mark` only for these.
 */
export const WATCHLIST_KINDS: ReadonlySet<string> = new Set([
  'movie',
  'series',
  'audiobook',
  'podcast',
]);

interface CoverWatchlistProps {
  item: Pick<ItemRef, 'id' | 'kind' | 'title' | 'inWatchlist'>;
  /**
   * For a row with one Tab stop (the spotlight row's roving tabindex): the button of a poster
   * that is not the stop.
   */
  excludeFromTabOrder?: boolean;
  /**
   * Called once a removal went through, while the card still stands: the watchlist page and the
   * start page's watchlist row move the focus to the neighbour before the card goes.
   */
  onRemoved?: () => void;
}

/**
 * The watchlist button on a cover (concept 17 D-9): a toggle beside the card's link, after the pen
 * in the Tab order and outside the arrow-key navigation, like `CoverPen`. Filled while the title is
 * on the watchlist. Offline it is disabled (the watchlist lives on the server). While a change of
 * the title is in flight (from this or another button) a press does nothing. A failed change turns
 * back in every list (useWatchlistMutation) and says why next to the button.
 */
export function CoverWatchlist({ item, excludeFromTabOrder, onRemoved }: CoverWatchlistProps) {
  const { t } = useTranslation();
  const online = useOnline();
  const mutation = useWatchlistMutation();
  const pending = useWatchlistPending(item.id);
  if (!WATCHLIST_KINDS.has(item.kind)) return null;
  const on = item.inWatchlist;
  return (
    <span className={styles.root}>
      <Button
        variant="secondary"
        size="sm"
        icon="bookmark"
        iconOnly
        overlay
        aria-pressed={on}
        aria-label={t(on ? 'watchlist.cover.remove' : 'watchlist.cover.add', {
          title: item.title,
        })}
        title={t(on ? 'item.watchlist.added' : 'item.watchlist.add')}
        isDisabled={!online}
        aria-disabled={pending || undefined}
        excludeFromTabOrder={excludeFromTabOrder}
        onPress={() => {
          if (pending) return;
          mutation.mutate({ id: item.id, on: !on }, { onSuccess: on ? onRemoved : undefined });
        }}
      />
      {mutation.isError && (
        <span className={styles.error} role="alert">
          {problemMessage(t, mutation.error)}
        </span>
      )}
    </span>
  );
}
