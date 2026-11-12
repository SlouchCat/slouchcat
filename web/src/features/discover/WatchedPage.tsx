import type { ItemRef, RootContent, WatchedEntry, WatchedSort } from '@slouchcat/api-client';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { CoverPen, EDITABLE_KINDS, EditMetadata } from '@/features/catalog/EditMetadata';
import { formatDayMonth } from '@/lib/format';
import { rangeBySize, usePagedList, type PlaceRange } from '@/lib/pagedList';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ChoiceChips,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
  setSpatialFocus,
} from '@/ui';
import { cardKey, PageRetry, type CardFacts } from './ItemGrid';
import { PagedItemGrid } from './PagedItemGrid';
import { watchedSource } from './queries';
import styles from './Discover.module.css';

const GRID_KEY = 'watched-item';
const BACK_KEY = 'watched-back';
const CONTENT_KEY = 'watched-content';
const SORT_KEY = 'watched-sort';

/** The chips «Alle», «Filme», «Serien», «Hörbücher», «Podcasts» (listWatched `content`). */
const CONTENTS = ['', 'movie', 'series', 'audiobook', 'podcast'] as const;
const SORTS: readonly WatchedSort[] = ['watched', 'title'];

/**
 * The page «Gesehen» (concept 20 P-14): every film and audiobook the profile has finished and
 * every series and podcast with a watched episode, newest first or by title, as poster cards with
 * the date or «19 von 96 Folgen · zuletzt 9. Nov.» and a bar. Each card has its pen (server
 * administrator) and its check mark; removing a full check takes the card off the list (the focus
 * goes to the neighbour first). The list comes from the server only: offline the page says so.
 */
export function WatchedPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('watched.page.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const [content, setContent] = useState<RootContent | ''>('');
  const [sort, setSort] = useState<WatchedSort>('watched');
  // The pen on the covers (concept 15 7.4), for the one who manages the library.
  const session = useActiveSession();
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  const [editing, setEditing] = useState<string | null>(null);
  const source = useMemo(() => watchedSource(content || undefined, sort), [content, sort]);
  // Every title, read page by page as the grid asks (concept 18 C-3).
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(source, rangeBySize(span));
  const list = paged.first;

  const entries = useMemo(() => {
    const byId = new Map<string, WatchedEntry>();
    for (const entry of paged.slots) if (entry) byId.set(entry.item.id, entry);
    return byId;
  }, [paged.slots]);
  // A title whose check was removed leaves when the list is read again (useWatchedMutation), after
  // the focus moved on (onUnwatched): the card stands until then, so its button keeps the focus.
  const slots = useMemo(() => paged.slots.map((entry) => entry?.item), [paged.slots]);
  const items = slots.filter((item): item is ItemRef => item !== undefined);

  const describe = (item: ItemRef): CardFacts => {
    const entry = entries.get(item.id);
    if (!entry) return {};
    const date = formatDayMonth(entry.lastWatchedAt, i18n.language);
    if (entry.episodeCount === undefined) {
      return {
        caption: t(item.kind === 'audiobook' ? 'watched.page.heardOn' : 'watched.page.watchedOn', {
          date,
        }),
      };
    }
    const count = item.watchedCount ?? 0;
    const total = entry.episodeCount;
    return {
      caption: t('watched.page.episodes', { count, total, date }),
      progress:
        total > 0
          ? {
              percent: Math.min(100, (count / total) * 100),
              label: t('item.season.progress', { count, total }),
            }
          : undefined,
    };
  };

  // The check of a card removed (concept 20 P-14): the focus goes to the neighbouring card (or
  // back) before the card leaves.
  const onUnwatched = (item: ItemRef) => {
    const index = items.findIndex((entry) => entry.id === item.id);
    const neighbour = items[index + 1] ?? items[index - 1];
    setSpatialFocus(neighbour ? cardKey(GRID_KEY, neighbour) : BACK_KEY);
  };

  const first = items[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first) : BACK_KEY}>
      <main className={styles.page} aria-busy={list.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('watched.page.title')}</h1>
          {list.isSuccess && (
            <p className={styles.overview}>{t('watched.page.count', { count: paged.total })}</p>
          )}
          <SpatialGroup className={styles.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
        </div>

        <section className={styles.filters} aria-label={t('watched.page.filters')}>
          <div className={styles.filterRow}>
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('watched.page.content')}
              </span>
              <ChoiceChips
                label={t('watched.page.content')}
                focusKey={CONTENT_KEY}
                value={content}
                onChange={(value) => setContent(value as RootContent | '')}
                options={CONTENTS.map((value) => ({
                  value,
                  label: t(`watched.page.contents.${value || 'all'}`),
                }))}
              />
            </div>
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('watched.page.sort')}
              </span>
              <ChoiceChips
                label={t('watched.page.sort')}
                focusKey={SORT_KEY}
                value={sort}
                onChange={(value) => setSort(value as WatchedSort)}
                options={SORTS.map((value) => ({
                  value,
                  label: t(`watched.page.sorts.${value}`),
                }))}
              />
            </div>
          </div>
        </section>

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

        {list.isSuccess && paged.total === 0 && (
          <Notice title={t('watched.page.empty.title')}>{t('watched.page.empty.body')}</Notice>
        )}

        {paged.total > 0 && (
          <section className={styles.section}>
            <PagedItemGrid
              slots={slots}
              onRange={setSpan}
              focusKey={GRID_KEY}
              label={t('watched.page.title')}
              describe={describe}
              onUnwatched={onUnwatched}
              tool={
                canManage
                  ? (item) =>
                      EDITABLE_KINDS.has(item.kind) ? (
                        <CoverPen item={item} onEdit={setEditing} />
                      ) : undefined
                  : undefined
              }
            />
            {paged.failed && <PageRetry onRetry={paged.retry} />}
          </section>
        )}
        {editing && <EditMetadata itemId={editing} onClose={() => setEditing(null)} />}
      </main>
    </SpatialPage>
  );
}
