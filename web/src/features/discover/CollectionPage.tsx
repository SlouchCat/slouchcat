import { useInfiniteQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ButtonLink,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
} from '@/ui';
import { cardKey, ItemGrid, MoreButton } from './ItemGrid';
import { collectionQuery } from './queries';
import styles from './Discover.module.css';

const route = getRouteApi('/_authed/_active/_chrome/collections/$collectionId');

const GRID_KEY = 'collection-item';
const BACK_KEY = 'collection-back';

/** A collection (E-6): name, description and the members the active profile may see. */
export function CollectionPage() {
  const { collectionId } = route.useParams();
  return <CollectionView key={collectionId} id={collectionId} />;
}

function CollectionView({ id }: { id: string }) {
  const { t } = useTranslation();
  const query = useInfiniteQuery(collectionQuery(id));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  // A refetch that finds the collection gone (deleted, no longer visible) wins over the cache.
  const gone = isApiError(query.error, 404);
  const collection = gone ? undefined : query.data?.pages[0];
  useDocumentTitle(
    collection?.name ?? (query.isError ? t('collection.notFound.title') : undefined),
  );
  const items = useMemo(() => query.data?.pages.flatMap((page) => page.items) ?? [], [query.data]);

  if (query.isPending) {
    return (
      <main className={styles.page} aria-busy="true">
        <Skeleton shape="title" />
        <Skeleton shape="block" />
      </main>
    );
  }

  if (!collection) {
    return (
      <main className={styles.page}>
        <Notice
          tone={gone ? 'info' : 'danger'}
          title={gone ? t('collection.notFound.title') : problemMessage(t, query.error)}
          actions={
            gone ? (
              <ButtonLink href={paths.home} variant="primary">
                {t('item.notFound.action')}
              </ButtonLink>
            ) : (
              <Button icon="refresh" onPress={() => void query.refetch()}>
                {t('common.retry')}
              </Button>
            )
          }
        >
          {gone && t('collection.notFound.body')}
        </Notice>
      </main>
    );
  }

  const first = items[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first) : BACK_KEY}>
      <main className={styles.page}>
        <div className={styles.head}>
          <h1 className={styles.title}>{collection.name}</h1>
          <p className={styles.muted}>{t('collection.count', { count: collection.total })}</p>
          {collection.overview && <p className={styles.overview}>{collection.overview}</p>}
          <SpatialGroup className={styles.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
        </div>
        <section className={styles.section} aria-labelledby="collection-members">
          <h2 id="collection-members" className={styles.sectionTitle}>
            {t('collection.members')}
          </h2>
          <ItemGrid items={items} focusKey={GRID_KEY} label={t('collection.members')} />
          <MoreButton
            shown={items.length}
            keys={items.map((item) => cardKey(GRID_KEY, item))}
            hasMore={query.hasNextPage}
            loading={query.isFetchingNextPage}
            failed={query.isFetchNextPageError}
            label={t('collection.more')}
            focusKey="collection-more"
            onMore={() => void query.fetchNextPage()}
          />
        </section>
      </main>
    </SpatialPage>
  );
}
