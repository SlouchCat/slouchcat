import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { cardKey, PageRetry } from '@/features/discover/ItemGrid';
import { PagedItemGrid } from '@/features/discover/PagedItemGrid';
import { rangeBySize, usePagedList, type PlaceRange } from '@/lib/pagedList';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Button, Notice, Skeleton, SpatialButton, SpatialGroup, SpatialPage } from '@/ui';
import { picksSource } from './queries';
import styles from '@/features/discover/Discover.module.css';

const GRID_KEY = 'picks-item';
const BACK_KEY = 'picks-back';

/**
 * Every user pick the active profile sees (concept 18 part B, E-2): each title once, newest pick
 * first, without a note; the start page row shows the first 20. No «Mehr laden» (concept 18,
 * 16.4, C-3): the pages are read as the grid asks (usePagedList). A page with another total, or
 * one that holds a title of another place (a pick made again moved it to the front meanwhile),
 * reads the list again, so every title stands once.
 */
export function PicksPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('picks.page.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(picksSource, rangeBySize(span));
  const list = paged.first;

  const first = paged.slots[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first) : BACK_KEY}>
      <main className={styles.page} aria-busy={list.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('picks.page.title')}</h1>
          {list.isSuccess && (
            <p className={styles.muted}>{t('picks.page.count', { count: paged.total })}</p>
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

        {paged.total > 0 && (
          <section className={styles.section}>
            <PagedItemGrid
              slots={paged.slots}
              onRange={setSpan}
              focusKey={GRID_KEY}
              label={t('picks.page.title')}
              showWatched
            />
            {paged.failed && <PageRetry onRetry={paged.retry} />}
          </section>
        )}
      </main>
    </SpatialPage>
  );
}
