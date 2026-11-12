import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  CollectionCard,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
} from '@/ui';
import { collectionsQuery } from './queries';
import styles from './Discover.module.css';

const GRID_KEY = 'collections-card';
const BACK_KEY = 'collections-back';

/**
 * «Sammlungen» (concept 20 O-3): every collection the active profile sees, as on the start page
 * row (by name, each with the number of its visible members), only without the row's cut to 20;
 * the target of that row's title. A card opens the collection.
 */
export function CollectionsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('collection.all.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const list = useQuery(collectionsQuery);
  const collections = list.data ?? [];
  const first = collections[0];

  return (
    <SpatialPage defaultFocusKey={first ? `${GRID_KEY}-${first.id}` : BACK_KEY}>
      <main className={styles.page} aria-busy={list.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('collection.all.title')}</h1>
          {list.isSuccess && (
            <p className={styles.muted}>
              {t('collection.all.count', { count: collections.length })}
            </p>
          )}
          <SpatialGroup className={styles.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
        </div>

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
        {list.isSuccess && collections.length === 0 && <Notice title={t('collection.all.empty')} />}

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
                    caption={t('collection.count', { count: collection.count })}
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
