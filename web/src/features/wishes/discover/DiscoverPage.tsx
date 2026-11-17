import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { useBackFocus } from '@/features/catalog/useBackFocus';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Notice, Row, SearchField, Skeleton, SpatialButton, SpatialGroup, SpatialPage } from '@/ui';
import discover from '@/features/discover/Discover.module.css';
import type { ExternalTitleRef } from '../queries';
import { DiscoverCard, useWishRequest } from './DiscoverCard';
import { DiscoverProblem } from './DiscoverProblem';
import { GenreTile } from './GenreTile';
import { entryId, wishDiscoverQuery, type DiscoverRow, type DiscoverRowKey } from './queries';
import { searchHref, titlesHref } from './wishesAreaParams';
import { WishesNav } from './WishesNav';
import styles from './WishesArea.module.css';

const SEARCH_KEY = 'discover-search';
/** The first cards of the first row load at once; every other poster lazily. */
const EAGER_POSTERS = 6;

/** Where a row's title leads (M-31): the grid of its kind, «Demnächst» sorted newest first. */
const ROW_HREF: Partial<Record<DiscoverRowKey, string>> = {
  popular_movies: titlesHref('movie', {}),
  upcoming_movies: titlesHref('movie', { sort: 'newest' }),
  popular_shows: titlesHref('show', {}),
  upcoming_shows: titlesHref('show', { sort: 'newest' }),
};

const genreKey = (row: DiscoverRowKey, id: number) => `discover-${row}-genre-${id}`;
const cardKey = (row: DiscoverRowKey, title: Pick<ExternalTitleRef, 'media' | 'tmdbId'>) =>
  `discover-${row}-${title.media}-${title.tmdbId}`;

/**
 * «Entdecken» (concept 25, 5.4, M-31), modelled on Seerr's start page: the search field, which
 * leads to «Suche», and the seven fixed rows (F-28) «Im Trend», «Beliebte Filme», «Film-Genres»,
 * «Demnächst im Kino», «Beliebte Serien», «Serien-Genres», «Demnächst als Serie». A row Seerr
 * could not read is left out and the band says the page shows the last state; a row title leads
 * to «Filme» or «Serien», a genre tile to the grid with that genre.
 */
export function DiscoverPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('wishes.discover.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const navigate = useNavigate();
  const query = useQuery(wishDiscoverQuery);
  const [text, setText] = useState('');
  const requests = useWishRequest(query.data);

  const rows = query.data?.rows ?? [];
  const shown = rows.filter((row) => row.entries.length > 0 || row.genres.length > 0);
  const stale = rows.some((row) => row.stale);
  const firstRow = shown.find((row) => row.entries.length > 0);
  const firstEntry = firstRow?.entries[0];
  const failed = query.isError && !query.data;
  // Back from a title: the card it was opened from has the focus again.
  useBackFocus(
    'discover',
    shown.flatMap((row) => [
      ...row.entries.map((entry) => cardKey(row.key, entry.title)),
      ...row.genres.map((genre) => genreKey(row.key, genre.id)),
    ]),
  );

  const search = (event: FormEvent) => {
    event.preventDefault();
    void navigate({ href: searchHref(text.trim()) });
  };

  const rowOf = (row: DiscoverRow, index: number) => {
    const title = t(`wishes.discover.rows.${row.key}`);
    if (row.key === 'movie_genres' || row.key === 'show_genres') {
      const media = row.key === 'movie_genres' ? 'movie' : 'show';
      return (
        <Row key={row.key} title={title} focusKey={`discover-${row.key}`} variant="chips">
          {row.genres.map((genre) => (
            <GenreTile
              key={genre.id}
              focusKey={genreKey(row.key, genre.id)}
              href={titlesHref(media, { genre: genre.id })}
              name={genre.name}
              imageUrl={imageUrl(genre.backdrop)}
            />
          ))}
        </Row>
      );
    }
    return (
      <Row key={row.key} title={title} focusKey={`discover-${row.key}`} href={ROW_HREF[row.key]}>
        {row.entries.map((entry, position) => (
          <DiscoverCard
            key={entryId(entry)}
            entry={entry}
            focusKey={cardKey(row.key, entry.title)}
            onRequest={requests.request}
            eager={index === 0 && position < EAGER_POSTERS}
          />
        ))}
      </Row>
    );
  };

  return (
    <SpatialPage
      defaultFocusKey={
        firstRow && firstEntry ? cardKey(firstRow.key, firstEntry.title) : SEARCH_KEY
      }
    >
      <main className={discover.page} aria-busy={query.isPending || undefined}>
        <WishesNav current="discover" limit />
        <div className={discover.head}>
          <h1 className={discover.title}>{t('wishes.discover.title')}</h1>
          {!failed && <p className={discover.overview}>{t('wishes.discover.intro')}</p>}
        </div>
        {!failed && (
          <form role="search" className={styles.search} onSubmit={search}>
            <SpatialGroup focusKey="discover-search-box" className={styles.searchRow}>
              <SearchField
                label={t('wishes.discover.searchLabel')}
                placeholder={t('wishes.discover.searchLabel')}
                value={text}
                onChange={setText}
                focusKey={SEARCH_KEY}
              />
              <SpatialButton type="submit" icon="search" variant="secondary">
                {t('wishes.discover.searchAction')}
              </SpatialButton>
            </SpatialGroup>
          </form>
        )}

        {stale && <Notice tone="warning" title={t('wishes.discover.stale')} />}
        {requests.element}

        {query.isPending && <Skeleton shape="block" />}
        {failed && <DiscoverProblem error={query.error} onRetry={() => void query.refetch()} />}
        {query.isSuccess && shown.length === 0 && <Notice title={t('wishes.discover.empty')} />}
        {shown.length > 0 && (
          <div className={`${discover.bleed} ${styles.rows}`}>{shown.map(rowOf)}</div>
        )}
      </main>
    </SpatialPage>
  );
}
