import { useInfiniteQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { cardKey, ItemGrid, MoreButton } from '@/features/discover/ItemGrid';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Button, Notice, Skeleton, SpatialButton, SpatialGroup, SpatialPage } from '@/ui';
import { picksPageQuery } from './queries';
import { useBackFocus } from './useBackFocus';
import styles from '@/features/discover/Discover.module.css';

const GRID_KEY = 'picks-item';
const BACK_KEY = 'picks-back';

/**
 * Every one of Cat's recommendations for the active profile (concept 08), in Cat's order and with
 * Cat's note under the poster; the start page row shows the first 20 of them.
 */
export function PicksPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('picks.page.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const list = useInfiniteQuery(picksPageQuery);
  const items = useMemo(() => list.data?.pages.flatMap((page) => page.items) ?? [], [list.data]);
  const total = list.data?.pages[0]?.total ?? 0;
  const cardKeys = useMemo(() => items.map((item) => cardKey(GRID_KEY, item)), [items]);
  useBackFocus(GRID_KEY, cardKeys);

  const first = items[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first) : BACK_KEY}>
      <main className={styles.page} aria-busy={list.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('picks.page.title')}</h1>
          {list.isSuccess && (
            <p className={styles.muted}>{t('picks.page.count', { count: total })}</p>
          )}
          <p className={styles.overview}>{t('picks.page.intro')}</p>
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

        {items.length > 0 && (
          <section className={styles.section}>
            <ItemGrid items={items} focusKey={GRID_KEY} label={t('picks.page.title')} />
            <MoreButton
              shown={items.length}
              keys={cardKeys}
              hasMore={list.hasNextPage}
              loading={list.isFetchingNextPage}
              failed={list.isFetchNextPageError}
              label={t('library.page.more', { count: Math.max(0, total - items.length) })}
              focusKey="picks-more"
              onMore={() => void list.fetchNextPage()}
            />
          </section>
        )}
      </main>
    </SpatialPage>
  );
}
