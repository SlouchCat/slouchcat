import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ChoiceChips,
  CollectionCard,
  Notice,
  SearchField,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
} from '@/ui';
import { arrangeCollections, collectionCaption, collectionProgress } from './collectionFacts';
import {
  COLLECTION_SIZES,
  COLLECTION_SORTS,
  validateCollectionsParams,
  type CollectionSize,
  type CollectionsParams,
} from './collectionsParams';
import { collectionsQuery } from './queries';
import styles from './Discover.module.css';

const GRID_KEY = 'collections-card';
const BACK_KEY = 'collections-back';
const SEARCH_KEY = 'collections-search';
const SIZE_KEY = 'collections-size';
const SORT_KEY = 'collections-sort';

const route = getRouteApi('/_authed/_active/_chrome/collections');

/**
 * «Sammlungen» (concept 20 O-3, concept 21 P-10 variant A): every collection the active profile
 * sees, searched, filtered by size («Alle» first) and sorted by name, by count or started first,
 * all in the client over the one answer of the server; search, size and order live in the URL.
 * Each poster carries the stack with the count and, when started, a bar of the parts seen; under
 * it «n Titel · Jahr–Jahr». A card opens the collection.
 */
export function CollectionsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('collection.all.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const params = route.useSearch();
  const navigate = route.useNavigate();
  const size: CollectionSize =
    params.min === undefined ? 'all' : (String(params.min) as CollectionSize);
  const sort = params.sort ?? 'name';
  const update = (change: Partial<CollectionsParams>) =>
    void navigate({ search: (previous) => changed(previous, change), replace: true });

  // The text field follows the URL when it changes from elsewhere (Back); typing filters at once
  // and replaces the address a moment later.
  const [text, setText] = useState(params.q ?? '');
  const [seenQ, setSeenQ] = useState(params.q);
  if (params.q !== seenQ) {
    setSeenQ(params.q);
    if ((params.q ?? '') !== text.trim()) setText(params.q ?? '');
  }
  const typed = text.trim();
  useEffect(() => {
    if (typed === (params.q ?? '')) return;
    const timer = window.setTimeout(() => {
      void navigate({
        search: (previous) => changed(previous, { q: typed || undefined }),
        replace: true,
      });
    }, SEARCH_SETTLE_MS);
    return () => window.clearTimeout(timer);
  }, [navigate, typed, params.q]);

  const list = useQuery(collectionsQuery);
  const all = useMemo(() => list.data ?? [], [list.data]);
  const collections = useMemo(
    () => arrangeCollections(all, { q: typed, size, sort }, i18n.language),
    [all, typed, size, sort, i18n.language],
  );
  const filtered = typed !== '' || size !== 'all';
  const first = collections[0];

  return (
    <SpatialPage defaultFocusKey={first ? `${GRID_KEY}-${first.id}` : BACK_KEY}>
      <main className={styles.page} aria-busy={list.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('collection.all.title')}</h1>
          {list.isSuccess && (
            <p className={styles.overview}>
              {filtered
                ? t('collection.all.shown', { count: collections.length, total: all.length })
                : t('collection.all.count', { count: all.length })}
            </p>
          )}
          <SpatialGroup className={styles.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
        </div>

        <section className={styles.filters} aria-label={t('collection.all.filters')}>
          <div className={styles.searchBox}>
            <SearchField
              label={t('collection.all.search')}
              placeholder={t('collection.all.search')}
              value={text}
              onChange={setText}
              focusKey={SEARCH_KEY}
            />
          </div>
          <div className={styles.filterRow}>
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('collection.all.size')}
              </span>
              <ChoiceChips
                label={t('collection.all.size')}
                focusKey={SIZE_KEY}
                value={size}
                onChange={(value) =>
                  update({
                    min: value === 'all' ? undefined : (Number(value) as CollectionsParams['min']),
                  })
                }
                options={COLLECTION_SIZES.map((value) => ({
                  value,
                  label: t(`collection.all.sizes.${value}`),
                }))}
              />
            </div>
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('collection.all.sort')}
              </span>
              <ChoiceChips
                label={t('collection.all.sort')}
                focusKey={SORT_KEY}
                value={sort}
                onChange={(value) =>
                  update({
                    sort: value === 'name' ? undefined : (value as CollectionsParams['sort']),
                  })
                }
                options={COLLECTION_SORTS.map((value) => ({
                  value,
                  label: t(`collection.all.sorts.${value}`),
                }))}
              />
            </div>
          </div>
        </section>

        {list.isPending && <Skeleton shape="block" />}
        {list.isError && (
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
        {list.isSuccess && all.length === 0 && <Notice title={t('collection.all.empty')} />}
        {list.isSuccess && all.length > 0 && collections.length === 0 && (
          <Notice title={t('collection.all.noMatch')} />
        )}

        {collections.length > 0 && (
          <section className={styles.section}>
            <SpatialGroup
              as="ul"
              focusKey={GRID_KEY}
              className={styles.grid}
              aria-label={t('collection.all.title')}
            >
              {collections.map((collection, index) => (
                <li key={collection.id} className={styles.cell}>
                  <CollectionCard
                    focusKey={`${GRID_KEY}-${collection.id}`}
                    href={paths.collection(collection.id)}
                    name={collection.name}
                    caption={collectionCaption(t, collection)}
                    badge={String(collection.count)}
                    badgeIcon="stack"
                    progress={collectionProgress(t, collection)}
                    imageUrl={imageUrl(collection.image)}
                    posterUrl={imageUrl(collection.cover?.images.poster)}
                    posterSmallUrl={imageUrl(collection.cover?.images.posterSmall)}
                    accent={collection.cover?.accent}
                    eager={index < 12}
                  />
                </li>
              ))}
            </SpatialGroup>
          </section>
        )}
      </main>
    </SpatialPage>
  );
}

/** The URL's parameters with a change; a parameter set to undefined goes. */
function changed(
  previous: Record<string, unknown>,
  change: Partial<CollectionsParams>,
): CollectionsParams {
  const next: CollectionsParams = { ...validateCollectionsParams(previous), ...change };
  for (const key of ['q', 'min', 'sort'] as const) if (next[key] === undefined) delete next[key];
  return next;
}

/** How long the typed search waits before it replaces the address. */
const SEARCH_SETTLE_MS = 300;
