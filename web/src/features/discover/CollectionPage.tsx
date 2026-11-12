import { getRouteApi } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { rangeBySize, usePagedList, type PlaceRange } from '@/lib/pagedList';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { BACK_KEY, DetailBody, DetailHeader, DetailPage } from '@/features/catalog/DetailHeader';
import { useCoverPens } from '@/features/catalog/EditMetadata';
import { isAudioKind } from '@/features/catalog/labels';
import { Button, ButtonLink, Notice, Skeleton, SpatialPage } from '@/ui';
import { cardKey, PageRetry } from './ItemGrid';
import { PagedItemGrid } from './PagedItemGrid';
import { collectionSource } from './queries';
import styles from './Discover.module.css';

const route = getRouteApi('/_authed/_active/_chrome/collections/$collectionId');

const GRID_KEY = 'collection-item';

/**
 * A collection (E-6): the head of a detail page (concept 17 E-16; the first member's cover and
 * backdrop, name, "N Titel", description, no play button) and the members the active profile may
 * see, with their watch state.
 */
export function CollectionPage() {
  const { collectionId } = route.useParams();
  return <CollectionView key={collectionId} id={collectionId} />;
}

function CollectionView({ id }: { id: string }) {
  const { t } = useTranslation();
  const pens = useCoverPens();
  // Every member, read page by page as the grid asks (concept 18 C-3).
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(collectionSource(id), rangeBySize(span));
  const query = paged.first;
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  // A refetch that finds the collection gone (deleted, no longer visible) wins over the cache.
  const gone = isApiError(query.error, 404);
  const collection = gone ? undefined : query.data;
  useDocumentTitle(
    collection?.name ?? (query.isError ? t('collection.notFound.title') : undefined),
  );

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

  const first = paged.slots[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first) : BACK_KEY}>
      <DetailPage>
        <DetailHeader
          images={first?.images ?? {}}
          ownPoster={collection.image}
          accent={first?.accent}
          cover={first && isAudioKind(first.kind) ? 'square' : 'poster'}
          onBack={goBack}
          title={collection.name}
          facts={
            <p className={styles.muted}>{t('collection.count', { count: collection.total })}</p>
          }
          overview={collection.overview}
        />
        <DetailBody>
          <section className={styles.members} aria-labelledby="collection-members">
            <h2 id="collection-members" className={styles.sectionTitle}>
              {t('collection.members')}
            </h2>
            <PagedItemGrid
              slots={paged.slots}
              onRange={setSpan}
              focusKey={GRID_KEY}
              label={t('collection.members')}
              showWatched
              tool={pens.tool}
            />
            {pens.editor}
            {paged.failed && <PageRetry onRetry={paged.retry} />}
          </section>
        </DetailBody>
      </DetailPage>
    </SpatialPage>
  );
}
