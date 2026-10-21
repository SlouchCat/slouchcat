import type { ItemRef } from '@slouchcat/api-client';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  setSpatialFocus,
} from '@/ui';
import { cardKey, ItemGrid, MoreButton } from './ItemGrid';
import { useWatchlistMutation, watchlistQuery } from './queries';
import styles from './Discover.module.css';

const GRID_KEY = 'watchlist-item';
const BACK_KEY = 'watchlist-back';
const removeKey = (item: ItemRef) => `watchlist-remove-${item.id}`;

/** The watchlist of the active profile (E-5): its titles, each with "Entfernen". */
export function WatchlistPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('watchlist.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const list = useInfiniteQuery(watchlistQuery);
  const remove = useWatchlistMutation();
  const items = useMemo(() => list.data?.pages.flatMap((page) => page.items) ?? [], [list.data]);

  const onRemove = (item: ItemRef) => {
    // The focus goes to the neighbour's button (or back) before the card disappears.
    const index = items.findIndex((entry) => entry.id === item.id);
    const neighbour = items[index + 1] ?? items[index - 1];
    remove.mutate(
      { id: item.id, on: false },
      { onSuccess: () => setSpatialFocus(neighbour ? removeKey(neighbour) : BACK_KEY) },
    );
  };

  const first = items[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first) : BACK_KEY}>
      <main className={styles.page} aria-busy={list.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('watchlist.title')}</h1>
          <SpatialGroup className={styles.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
        </div>

        {list.isPending && <Skeleton shape="block" />}
        {list.isError && !list.data && (
          <Notice
            tone="danger"
            title={problemMessage(t, list.error)}
            actions={
              <Button icon="refresh" onPress={() => void list.refetch()}>
                {t('common.retry')}
              </Button>
            }
          />
        )}
        {remove.isError && <Notice tone="danger" title={problemMessage(t, remove.error)} />}

        {list.isSuccess && items.length === 0 && (
          <Notice
            title={t('watchlist.empty.title')}
            actions={
              <SpatialGroup>
                <SpatialButtonLink
                  focusKey="watchlist-search"
                  href={paths.search}
                  variant="primary"
                >
                  {t('watchlist.empty.action')}
                </SpatialButtonLink>
              </SpatialGroup>
            }
          >
            {t('watchlist.empty.body')}
          </Notice>
        )}

        {items.length > 0 && (
          <section className={styles.section}>
            <ItemGrid
              items={items}
              focusKey={GRID_KEY}
              label={t('watchlist.title')}
              below={(item) => (
                <SpatialButton
                  focusKey={removeKey(item)}
                  variant="ghost"
                  icon="trash"
                  aria-label={t('watchlist.removeLabel', { title: item.title })}
                  aria-disabled={remove.isPending || undefined}
                  onPress={() => {
                    if (!remove.isPending) onRemove(item);
                  }}
                >
                  {t('watchlist.remove')}
                </SpatialButton>
              )}
            />
            <MoreButton
              shown={items.length}
              keys={items.map((item) => cardKey(GRID_KEY, item))}
              hasMore={list.hasNextPage}
              loading={list.isFetchingNextPage}
              failed={list.isFetchNextPageError}
              label={t('watchlist.more')}
              focusKey="watchlist-more"
              onMore={() => void list.fetchNextPage()}
            />
          </section>
        )}
      </main>
    </SpatialPage>
  );
}
