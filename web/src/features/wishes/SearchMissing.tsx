import type { ItemRef } from '@slouchcat/api-client';
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useOnline } from '@/lib/useOnline';
import { Notice, PosterCard, RequestMark, setSpatialFocus, SpatialGroup } from '@/ui';
import discover from '@/features/discover/Discover.module.css';
import {
  externalSearchQuery,
  type ExternalMedia,
  type ExternalTitlePage,
  type ExternalTitleRef,
  type Wish,
} from './queries';
import { externalPoster, wishProblemMessage, wishStateText } from './state';

const RequestDialog = lazy(() =>
  import('./RequestDialog').then((module) => ({ default: module.RequestDialog })),
);

const cardKey = (title: ExternalTitleRef) => `search-missing-${title.media}-${title.tmdbId}`;

/**
 * The query of the search section «Noch nicht in der Bibliothek» (concept 25, 5.2): only while the
 * session says the profile can request (the server filters each hit by right, age and switch,
 * rule 12), for the media the search's kind filter keeps (`undefined` = both, `null` = none).
 */
export function useSearchMissing(
  q: string | undefined,
  media: ExternalMedia | null | undefined,
): UseQueryResult<ExternalTitlePage> {
  const session = useActiveSession();
  const can = session.wishes?.enabled === true && session.wishes.canRequest;
  return useQuery({
    ...externalSearchQuery(q ?? '', media ?? undefined),
    enabled: can && media !== null && q !== undefined && q.trim().length >= 2,
  });
}

/**
 * The section «Noch nicht in der Bibliothek» under the library's hits: poster cards to the preview
 * page with «2026 · Noch nicht da» or the state of the own wish, and the round plus on the poster
 * that opens the request dialog. Its error stays inside the section, the library's hits stay.
 */
export function SearchMissing({ query }: { query: UseQueryResult<ExternalTitlePage> }) {
  const { t } = useTranslation();
  const online = useOnline();
  const navigate = useNavigate();
  const [requesting, setRequesting] = useState<ExternalTitleRef | null>(null);
  const [done, setDone] = useState<Wish | null>(null);
  // The card whose plus went with its request: it takes the focus once the plus is gone.
  const refocus = useRef<string | null>(null);
  const hits = query.data?.items ?? [];
  useEffect(() => {
    const card = refocus.current;
    if (card === null) return;
    const active = document.activeElement;
    if (active && active !== document.body && active.isConnected) return;
    refocus.current = null;
    setSpatialFocus(card);
  }, [query.data]);
  if (!query.isError && hits.length === 0) return null;

  const caption = (title: ExternalTitleRef) =>
    [
      ...(title.year ? [String(title.year)] : []),
      title.wish
        ? wishStateText(t, title.wish.state, title.wish.stateDetail)
        : t('search.missing.notHere'),
    ].join(t('format.separator'));

  return (
    <section className={discover.section} aria-labelledby="search-missing">
      <h2 id="search-missing" className={discover.sectionTitle}>
        {t('search.missing.title')}
      </h2>
      {query.isError && <Notice tone="danger" title={wishProblemMessage(t, query.error)} />}
      {hits.length > 0 && (
        <SpatialGroup as="ul" className={discover.grid} aria-label={t('search.missing.title')}>
          {hits.map((title) => (
            <li key={cardKey(title)} className={discover.cell}>
              <PosterCard
                focusKey={cardKey(title)}
                href={paths.title(title.media, title.tmdbId)}
                title={title.title}
                caption={caption(title)}
                {...externalPoster(title.images)}
                mark={
                  !title.wish && title.canRequest ? (
                    <RequestMark
                      label={t('search.missing.request', { title: title.title })}
                      isDisabled={!online}
                      onPress={() => {
                        setDone(null);
                        setRequesting(title);
                      }}
                    />
                  ) : undefined
                }
              />
            </li>
          ))}
        </SpatialGroup>
      )}
      {done && (
        <Notice
          title={t(done.role === 'joined' ? 'titles.done.joined' : 'titles.done.requested')}
        />
      )}
      {requesting && (
        <Suspense fallback={null}>
          <RequestDialog
            title={requesting}
            source="search"
            onClose={() => setRequesting(null)}
            onDone={(wish) => {
              refocus.current = cardKey(requesting);
              setRequesting(null);
              setDone(wish);
            }}
            onAvailable={(item: ItemRef) => void navigate({ href: paths.item(item.id) })}
          />
        </Suspense>
      )}
    </section>
  );
}
