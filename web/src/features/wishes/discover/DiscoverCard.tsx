import type { ItemRef } from '@slouchcat/api-client';
import { useNavigate } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { useOnline } from '@/lib/useOnline';
import { Notice, PosterCard, RequestMark, setSpatialFocus, Skeleton } from '@/ui';
import discover from '@/features/discover/Discover.module.css';
import type { ExternalTitleRef, Wish } from '../queries';
import { externalPoster, REFOCUS_MS, WishStateChip } from '../state';
import type { DiscoverEntry } from './queries';
import styles from './WishesArea.module.css';

/** Placeholder cards while a grid's first page loads: about two rows on a wide screen. */
const SKELETON_CARDS = 12;

// Dialogs need react-aria, which the browsing pages do not load up front (P-10).
const RequestDialog = lazy(() =>
  import('../RequestDialog').then((module) => ({ default: module.RequestDialog })),
);

/** «2026 · In der Bibliothek», «2026 · Noch nicht da», «2026 · Bereits angefordert». */
function caption(t: TFunction, entry: DiscoverEntry): string {
  const { title } = entry;
  // A library title has its mark, an own wish its state chip on the poster.
  const state =
    entry.item || title.wish
      ? undefined
      : t(entry.seerrRequested ? 'wishes.discover.requested' : 'search.missing.notHere');
  return [...(title.year ? [String(title.year)] : []), ...(state ? [state] : [])].join(
    t('format.separator'),
  );
}

interface DiscoverCardProps {
  entry: DiscoverEntry;
  focusKey: string;
  /** The plus on the poster: opens the request dialog for the title of the card `focusKey`. */
  onRequest: (title: ExternalTitleRef, focusKey: string) => void;
  eager?: boolean;
}

/**
 * One tile of the wishes area (concept 25, 5.4): a title of the library links to its page with
 * the mark «In der Bibliothek»; a missing title links to the preview page and carries the plus
 * that opens the request dialog; the profile's own wish shows its state on the poster; a title
 * Seerr knows a request for carries the bell, which leads to the preview page and its «Benachrichtige
 * mich». A title without stored images shows the lettered placeholder. Every mark is the
 * server's answer (rule 12): `item`, `title.wish`, `title.canRequest`, `seerrRequested`.
 */
export function DiscoverCard({ entry, focusKey, onRequest, eager }: DiscoverCardProps) {
  const { t } = useTranslation();
  const online = useOnline();
  const navigate = useNavigate();
  const { title, item } = entry;
  if (item) {
    return (
      <PosterCard
        focusKey={focusKey}
        href={paths.item(item.id)}
        title={item.title}
        caption={caption(t, entry)}
        badge={t('wishes.discover.inLibrary')}
        posterUrl={imageUrl(item.images.poster)}
        posterSmallUrl={imageUrl(item.images.posterSmall)}
        accent={item.accent}
        eager={eager}
      />
    );
  }
  const preview = paths.title(title.media, title.tmdbId);
  let mark: ReactNode;
  if (!title.wish && title.canRequest) {
    mark = entry.seerrRequested ? (
      <RequestMark
        icon="bell"
        label={t('wishes.discover.join', { title: title.title })}
        isDisabled={!online}
        onPress={() => void navigate({ href: preview })}
      />
    ) : (
      <RequestMark
        label={t('search.missing.request', { title: title.title })}
        isDisabled={!online}
        onPress={() => onRequest(title, focusKey)}
      />
    );
  }
  return (
    <PosterCard
      focusKey={focusKey}
      href={preview}
      title={title.title}
      caption={caption(t, entry)}
      {...externalPoster(title.images)}
      eager={eager}
      mark={mark}
    >
      {/* Placed by hand: PosterCard's `badge` takes a neutral text only (and holds «In der
          Bibliothek» on library tiles), `mark` the plus; the state needs its tone (wishTone). */}
      {title.wish && (
        <span className={styles.state}>
          <WishStateChip state={title.wish.state} detail={title.wish.stateDetail} />
        </span>
      )}
    </PosterCard>
  );
}

/**
 * The request dialog of a page of tiles and its answer: `request(title, focusKey)` opens it for
 * the title of a card; after the request the focus goes back to that card, whose plus has gone
 * with the new state (`refreshed`: the page's data), and a short note says what happened.
 */
export function useWishRequest(refreshed: unknown) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [requesting, setRequesting] = useState<{ title: ExternalTitleRef; card: string } | null>(
    null,
  );
  const [done, setDone] = useState<Wish | null>(null);
  const refocus = useRef<string | null>(null);
  const expire = useRef<number | undefined>(undefined);
  // Once the list came back with the new state: the tile takes the focus the plus held.
  useEffect(() => {
    const card = refocus.current;
    if (card === null) return;
    const active = document.activeElement;
    if (active && active !== document.body && active.isConnected) return;
    refocus.current = null;
    setSpatialFocus(card);
  }, [refreshed]);
  // A refocus the list never answered is dropped, so it cannot move the focus at a later,
  // unrelated change of the data.
  useEffect(() => () => window.clearTimeout(expire.current), []);
  const element = (
    <>
      {done && (
        <Notice
          title={t(done.role === 'joined' ? 'titles.done.joined' : 'titles.done.requested')}
        />
      )}
      {requesting && (
        <Suspense fallback={null}>
          <RequestDialog
            title={requesting.title}
            source="list"
            onClose={() => setRequesting(null)}
            onDone={(wish) => {
              refocus.current = requesting.card;
              window.clearTimeout(expire.current);
              expire.current = window.setTimeout(() => {
                refocus.current = null;
              }, REFOCUS_MS);
              setRequesting(null);
              setDone(wish);
            }}
            onAvailable={(item: ItemRef) => void navigate({ href: paths.item(item.id) })}
          />
        </Suspense>
      )}
    </>
  );
  return {
    request: (title: ExternalTitleRef, card: string) => {
      setDone(null);
      setRequesting({ title, card });
    },
    element,
  };
}

/** The grid while its first page loads: poster placeholders in the grid's cells (as LibraryGrid). */
export function GridSkeleton() {
  return (
    <ul className={discover.grid} aria-hidden="true">
      {Array.from({ length: SKELETON_CARDS }, (_, index) => (
        <li key={index} className={discover.cell}>
          <Skeleton shape="poster" />
        </li>
      ))}
    </ul>
  );
}
