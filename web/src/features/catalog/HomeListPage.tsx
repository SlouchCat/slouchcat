import type { HomeListKey, RootContent } from '@slouchcat/api-client';
import { getRouteApi } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { cardKey, PageRetry } from '@/features/discover/ItemGrid';
import { PagedItemGrid } from '@/features/discover/PagedItemGrid';
import { rangeBySize, usePagedList, type PlaceRange } from '@/lib/pagedList';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ButtonLink,
  ChoiceChips,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
} from '@/ui';
import { useCoverPens } from './EditMetadata';
import { HOME_LIST_CONTENTS, HOME_LIST_KEYS } from './homeListParams';
import { homeRowTitle } from './labels';
import { homeListSource } from './queries';
import styles from '@/features/discover/Discover.module.css';

const route = getRouteApi('/_authed/_active/_chrome/lists/$key');

const GRID_KEY = 'home-list-item';
const BACK_KEY = 'home-list-back';
const FILTER_KEY = 'home-list-content';

/**
 * The whole list behind a row title of the start page (concept 20 P-7): «Spotlight»,
 * «Weiterschauen», «Neu hinzugefügt» or «Kürzlich veröffentlicht», in the row's order and by its
 * rules, only without the cut to 20. Heading, count, the filter by the kind of the top level
 * («Serien» keeps the next episodes of a series), the grid read page by page as it asks (concept 18
 * C-3) and the pen on every card for the one who manages the library.
 */
export function HomeListPage() {
  const { key } = route.useParams();
  const known = (HOME_LIST_KEYS as readonly string[]).includes(key);
  return known ? <HomeList key={key} listKey={key as HomeListKey} /> : <ListNotFound />;
}

function HomeList({ listKey }: { listKey: HomeListKey }) {
  const { t } = useTranslation();
  const pens = useCoverPens();
  const { content } = route.useSearch();
  const navigate = route.useNavigate();
  const title = homeRowTitle(t, { key: listKey });
  useDocumentTitle(title);
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(homeListSource(listKey, content), rangeBySize(span));
  const list = paged.first;

  if (isApiError(list.error, 404)) return <ListNotFound />;

  const first = paged.slots[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first) : BACK_KEY}>
      <main className={styles.page} aria-busy={list.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{title}</h1>
          {list.isSuccess && (
            <p className={styles.muted} role="status">
              {t('home.list.count', { count: paged.total })}
            </p>
          )}
          <SpatialGroup className={styles.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
          <ChoiceChips
            label={t('library.content.title')}
            focusKey={FILTER_KEY}
            value={content ?? ''}
            onChange={(value) =>
              void navigate({
                search: value === '' ? {} : { content: value as RootContent },
                replace: true,
              })
            }
            options={[
              { value: '', label: t('library.content.all') },
              ...HOME_LIST_CONTENTS.map((value) => ({
                value,
                label: t(`library.content.${value}`),
              })),
            ]}
          />
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
        {list.isSuccess && paged.total === 0 && <Notice title={t('home.list.empty')} />}

        {paged.total > 0 && (
          <section className={styles.section}>
            <PagedItemGrid
              slots={paged.slots}
              onRange={setSpan}
              focusKey={GRID_KEY}
              label={title}
              showWatched
              tool={pens.tool}
            />
            {paged.failed && <PageRetry onRetry={paged.retry} />}
          </section>
        )}
        {pens.editor}
      </main>
    </SpatialPage>
  );
}

/** An unknown list, or the spotlight while it has no row (switched off, too few hits). */
function ListNotFound() {
  const { t } = useTranslation();
  useDocumentTitle(t('home.list.notFound.title'));
  return (
    <main className={styles.page}>
      <Notice
        title={t('home.list.notFound.title')}
        actions={
          <ButtonLink href={paths.home} variant="primary">
            {t('item.notFound.action')}
          </ButtonLink>
        }
      >
        {t('home.list.notFound.body')}
      </Notice>
    </main>
  );
}
