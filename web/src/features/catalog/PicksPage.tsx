import type { ItemRef } from '@slouchcat/api-client';
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { cardKey } from '@/features/discover/ItemGrid';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Button, Notice, Skeleton, SpatialButton, SpatialGroup, SpatialPage } from '@/ui';
import { LibraryGrid } from './LibraryGrid';
import type { DrawnRange } from './libraryPages';
import { picksPageQuery } from './queries';
import styles from '@/features/discover/Discover.module.css';

const GRID_KEY = 'picks-item';
const BACK_KEY = 'picks-back';

/**
 * Every user pick the active profile sees (concept 18 part B, E-2): each title once, newest pick
 * first, without a note; the start page row shows the first 20. No «Mehr laden» (concept 18,
 * 16.4): the grid of the library pages draws a place for every title and the next page is read
 * once the drawn places reach the end of what is there. A title that two pages both hold (a new
 * pick moved the list meanwhile) stands once; a page with another total reads the list again.
 */
export function PicksPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('picks.page.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const queryClient = useQueryClient();
  const list = useInfiniteQuery(picksPageQuery);
  const pages = list.data?.pages;
  const total = pages?.[0]?.total ?? 0;
  const loaded = useMemo(() => {
    const seen = new Set<string>();
    const out: ItemRef[] = [];
    for (const item of pages?.flatMap((page) => page.items) ?? []) {
      if (seen.has(item.id)) continue;
      seen.add(item.id);
      out.push(item);
    }
    return out;
  }, [pages]);
  const slots = useMemo(
    () =>
      Array.from({ length: Math.max(total, loaded.length) }, (_, index): ItemRef | undefined =>
        index < loaded.length ? loaded[index] : undefined,
      ),
    [loaded, total],
  );

  // The list changed between two pages (a new pick, one that expired): read it again from the top.
  const shifted = pages?.some((page) => page.total !== total) ?? false;
  useEffect(() => {
    if (shifted) void queryClient.resetQueries({ queryKey: picksPageQuery.queryKey, exact: true });
  }, [queryClient, shifted]);

  const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = list;
  const onRange = useCallback(
    (range: DrawnRange) => {
      if (
        range.end >= loaded.length &&
        hasNextPage &&
        !isFetchingNextPage &&
        !isFetchNextPageError
      ) {
        void fetchNextPage();
      }
    },
    [loaded.length, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage],
  );

  const first = loaded[0];
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
        {isFetchNextPageError && (
          <Notice
            tone="danger"
            title={t('library.page.pageFailed')}
            actions={
              <SpatialGroup>
                <SpatialButton
                  focusKey="picks-retry"
                  icon="refresh"
                  onPress={() => void fetchNextPage()}
                >
                  {t('common.retry')}
                </SpatialButton>
              </SpatialGroup>
            }
          />
        )}

        {slots.length > 0 && (
          <section className={styles.section}>
            <LibraryGrid
              slots={slots}
              focusKey={GRID_KEY}
              label={t('picks.page.title')}
              square={false}
              onRange={onRange}
            />
          </section>
        )}
      </main>
    </SpatialPage>
  );
}
