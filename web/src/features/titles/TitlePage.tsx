import type { ItemRef } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { lazy, Suspense, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { paths } from '@/app/paths';
import { BACK_KEY, DetailBody, DetailHeader, DetailPage } from '@/features/catalog/DetailHeader';
import { TRAILER_KEY } from '@/features/catalog/DetailActions';
import {
  externalTitleQuery,
  useCreateWish,
  useWithdrawWish,
  wishesSource,
  type ExternalMedia,
  type ExternalTitle,
  type Wish,
} from '@/features/wishes/queries';
import {
  availableItem,
  externalImages,
  failText,
  reasonText,
  wishProblemMessage,
  wishStateText,
} from '@/features/wishes/state';
import { WishCard } from '@/features/wishes/WishCard';
import { intlLocale } from '@/i18n';
import { formatDate, formatRuntime } from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { useOnline } from '@/lib/useOnline';
import {
  Button,
  ButtonLink,
  Chip,
  Notice,
  RoundAction,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  setSpatialFocus,
} from '@/ui';
import itemStyles from '@/features/catalog/ItemPage.module.css';
import styles from '@/features/wishes/Wishes.module.css';

const route = getRouteApi('/_authed/_active/_chrome/titles/$media/$tmdbId');

// Dialogs need react-aria, which the browsing pages do not load up front (P-10).
const RequestDialog = lazy(() =>
  import('@/features/wishes/RequestDialog').then((module) => ({ default: module.RequestDialog })),
);
const WithdrawDialog = lazy(() =>
  import('@/features/wishes/WithdrawDialog').then((module) => ({
    default: module.WithdrawDialog,
  })),
);
const TitleTrailer = lazy(() =>
  import('./TitleTrailer').then((module) => ({ default: module.TitleTrailer })),
);

/** The big button: «Film anfordern», «Serie anfordern» or the state of the own wish. */
export const REQUEST_KEY = 'title-request';

const MEDIA: readonly ExternalMedia[] = ['movie', 'show'];

/**
 * The preview page of a title outside the library (concept 25, 5.1, M-03/M-04): the head of a
 * detail page with the chip «Noch nicht in der Bibliothek», the big button «Film anfordern» or
 * «Serie anfordern» (or the state of the profile's wish, which leads to «Meine Wünsche»; on a film
 * another profile wished, its state and «Benachrichtige mich», F-7), the
 * trailer, the wish card, the last closed wish and the seasons of a series. Whether the profile
 * may see and request the title is the server's answer (rule 12): 404 is «not there», 409 a title
 * of the library, which the page opens instead.
 */
export function TitlePage() {
  const { media, tmdbId } = route.useParams();
  const id = Number(tmdbId);
  const known = MEDIA.find((entry) => entry === media);
  if (!known || !Number.isSafeInteger(id) || id < 1) return <TitleMissing />;
  // One state (dialogs, notices) per title.
  return <TitleView key={`${known}-${id}`} media={known} tmdbId={id} />;
}

/** «Nicht gefunden»: the title is unknown or the profile may not see it (the server says which). */
function TitleMissing() {
  const { t } = useTranslation();
  useDocumentTitle(t('titles.notFound.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  return (
    <main className={`${itemStyles.detail} ${itemStyles.message}`}>
      <Notice
        tone="info"
        title={t('titles.notFound.title')}
        actions={
          <ButtonLink href={paths.home} variant="primary">
            {t('item.notFound.action')}
          </ButtonLink>
        }
      >
        {t('titles.notFound.body')}
      </Notice>
    </main>
  );
}

/** The pages of «Meine Wünsche» (100 each) read at most to find the open wish. */
const FIND_PAGES = 5;
const FIND_LIMIT = 100;

/**
 * The open wish of the profile with its history: «Meine Wünsche» holds it (listWishes has no
 * filter by id or title), read page by page until it is found, at most FIND_PAGES pages; null when
 * it is not among them.
 */
function useOpenWish(wishId: number | undefined) {
  const source = wishesSource('open');
  return useQuery({
    queryKey: [...source.key, 'find', wishId],
    queryFn: async ({ signal }): Promise<Wish | null> => {
      for (let page = 0; page < FIND_PAGES; page++) {
        const read = await source.fetch(page * FIND_LIMIT, FIND_LIMIT, signal);
        const found = read.items.find((wish) => wish.id === wishId);
        if (found) return found;
        if ((page + 1) * FIND_LIMIT >= read.total) break;
      }
      return null;
    },
    enabled: wishId !== undefined,
  });
}

function TitleView({ media, tmdbId }: { media: ExternalMedia; tmdbId: number }) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const query = useQuery(externalTitleQuery(media, tmdbId));
  const title = query.data;
  const navigate = useNavigate();
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  useDocumentTitle(title?.title ?? t('titles.loading'));
  const online = useOnline();
  const [requesting, setRequesting] = useState(false);
  const [withdrawAsk, setWithdrawAsk] = useState(false);
  const [trailerOpen, setTrailerOpen] = useState(false);
  const [done, setDone] = useState<Wish | null>(null);
  const withdraw = useWithdrawWish();
  const join = useCreateWish();
  const openWish = useOpenWish(title?.wish?.id);

  // A title of the library the profile sees (409 `already_available`): its own page instead.
  const available = availableItem(query.error);
  useEffect(() => {
    if (available) void navigate({ href: paths.item(available.id), replace: true });
  }, [available, navigate]);
  const toLibrary = (item: ItemRef) => void navigate({ href: paths.item(item.id), replace: true });

  if (!online && trailerOpen) setTrailerOpen(false);

  if (query.isPending || available) {
    return (
      <main className={itemStyles.detail} aria-busy="true">
        <div className={itemStyles.loading}>
          <Skeleton shape="title" />
          <Skeleton shape="line" />
          <Skeleton shape="block" />
        </div>
      </main>
    );
  }

  if (query.isError || !title) {
    // An id the server cannot take (422) is as unknown as one it does not have.
    if (isApiError(query.error, 404) || isApiError(query.error, 422)) return <TitleMissing />;
    return (
      <main className={`${itemStyles.detail} ${itemStyles.message}`}>
        <Notice
          tone="danger"
          title={wishProblemMessage(t, query.error)}
          actions={
            <Button icon="refresh" onPress={() => void query.refetch()}>
              {t('common.retry')}
            </Button>
          }
        />
      </main>
    );
  }

  const wish = title.wish;
  // Another profile's open wish on the film (F-7): its state, never who; join it or only see it.
  const shared = wish ? undefined : title.request.shared;
  const canRequest = title.request.canRequest && !wish;
  const startKey = wish || canRequest ? REQUEST_KEY : BACK_KEY;
  const hasTrailer = title.trailers.length > 0 && online;
  const facts = [
    ...(title.year ? [String(title.year)] : []),
    ...(media === 'movie' && title.runtimeMinutes ? [formatRuntime(t, title.runtimeMinutes)] : []),
    ...(media === 'show' && title.seasons.length > 0
      ? [t('titles.seasonCount', { count: title.seasons.length })]
      : []),
  ];

  let action = null;
  if (wish) {
    action = (
      <SpatialButtonLink
        focusKey={REQUEST_KEY}
        href={paths.wishes}
        variant="secondary"
        size="lg"
        icon="heart"
      >
        {wishStateText(t, wish.state, wish.stateDetail)}
      </SpatialButtonLink>
    );
  } else if (shared?.canJoin) {
    // «Benachrichtige mich» (concept 25, 4.3): one press joins, no dialog, no limit used.
    action = (
      <SpatialButton
        focusKey={REQUEST_KEY}
        variant="primary"
        size="lg"
        icon="bell"
        aria-disabled={join.isPending || undefined}
        onPress={() => {
          if (join.isPending) return;
          setDone(null);
          withdraw.reset();
          join.mutate(
            { media, tmdbId, source: 'detail' },
            {
              // The big button turns into the state of the joined wish and keeps the focus.
              onSuccess: (made) => {
                setDone(made);
                setSpatialFocus(REQUEST_KEY);
              },
              onError: (error) => {
                const item = availableItem(error);
                if (item) toLibrary(item);
              },
            },
          );
        }}
      >
        {t('titles.join')}
      </SpatialButton>
    );
  } else if (canRequest) {
    action = (
      <SpatialButton
        focusKey={REQUEST_KEY}
        variant="primary"
        size="lg"
        icon="plus"
        onPress={() => {
          setDone(null);
          join.reset();
          withdraw.reset();
          setRequesting(true);
        }}
      >
        {t(media === 'movie' ? 'titles.requestMovie' : 'titles.requestShow')}
      </SpatialButton>
    );
  }

  // Why the profile cannot request it: the server's reason; the own wish speaks for itself.
  const reason = title.request.canRequest ? undefined : title.request.reason;
  const last = !wish ? title.lastWish : undefined;
  const nextAir = title.nextEpisode?.airDate;

  return (
    <SpatialPage defaultFocusKey={startKey}>
      <DetailPage>
        <DetailHeader
          images={externalImages(title.images)}
          cover="poster"
          onBack={goBack}
          belowBack={startKey === REQUEST_KEY ? REQUEST_KEY : undefined}
          title={title.title}
          logoUrl={imageUrl(externalImages(title.images).logo)}
          lines={
            title.originalTitle && title.originalTitle !== title.title ? [title.originalTitle] : []
          }
          facts={
            <p className={`${itemStyles.muted} ${styles.facts}`}>
              {facts.length > 0 && <span>{facts.join(t('format.separator'))}</span>}
              <Chip tone="outline">{t('titles.missing')}</Chip>
            </p>
          }
          genres={title.genres.join(', ')}
          tagline={title.tagline}
          actions={
            <>
              {action && <SpatialGroup className={itemStyles.play}>{action}</SpatialGroup>}
              <SpatialGroup ordered className={itemStyles.roundActions}>
                {hasTrailer && (
                  <RoundAction
                    focusKey={TRAILER_KEY}
                    icon="play"
                    label={t('item.trailer')}
                    onPress={() => setTrailerOpen(true)}
                  />
                )}
              </SpatialGroup>
              {done && (
                <Notice
                  title={t(done.role === 'joined' ? 'titles.done.joined' : 'titles.done.requested')}
                />
              )}
              {join.isError && !availableItem(join.error) && (
                <Notice tone="danger" title={wishProblemMessage(t, join.error)} />
              )}
              {withdraw.isError && (
                <Notice tone="danger" title={wishProblemMessage(t, withdraw.error)} />
              )}
              {withdraw.isSuccess && !wish && <Notice title={t('titles.done.withdrawn')} />}
            </>
          }
          overview={title.overview}
        />

        <DetailBody>
          {wish && openWish.isPending && (
            <div className={itemStyles.block}>
              <Skeleton shape="block" />
            </div>
          )}
          {wish && openWish.isError && (
            <div className={itemStyles.block}>
              <Notice
                tone="danger"
                title={wishProblemMessage(t, openWish.error)}
                actions={
                  <Button icon="refresh" onPress={() => void openWish.refetch()}>
                    {t('common.retry')}
                  </Button>
                }
              />
            </div>
          )}
          {wish && openWish.data && (
            <div className={itemStyles.block}>
              <WishCard
                // The progress of the preview's own answer: read for this page (WishMark).
                wish={{ ...openWish.data, progress: wish.progress ?? openWish.data.progress }}
                withdrawing={withdraw.isPending}
                onWithdraw={() => setWithdrawAsk(true)}
              />
            </div>
          )}
          {shared && (
            <div className={itemStyles.block}>
              <Notice
                tone="info"
                title={t('titles.shared.title', {
                  state: wishStateText(t, shared.state, shared.stateDetail),
                })}
              >
                {t(shared.canJoin ? 'titles.shared.join' : 'titles.shared.statusOnly')}
              </Notice>
            </div>
          )}
          {!shared && reason && reason !== 'already_requested' && (
            <div className={itemStyles.block}>
              <Notice
                tone={reason === 'status_only' ? 'info' : 'warning'}
                title={t(
                  reason === 'status_only' ? 'titles.alreadyRequested' : 'titles.cannotRequest',
                )}
              >
                {reasonText(t, reason)}
              </Notice>
            </div>
          )}
          {last && <LastWish last={last} language={language} />}
          {media === 'show' && title.seasons.length > 0 && (
            <section className={itemStyles.block} aria-labelledby="title-seasons">
              <h2 id="title-seasons" className={itemStyles.sectionTitle}>
                {t('titles.seasons')}
              </h2>
              {nextAir && (
                <p className={itemStyles.muted}>
                  {t('titles.nextEpisode', { date: formatDate(nextAir, language) })}
                </p>
              )}
              <ul className={styles.seasons}>
                {title.seasons.map((season) => (
                  <li key={season.number} className={styles.season}>
                    <span className={styles.seasonName}>
                      {t('wishes.dialog.season', { number: season.number })}
                    </span>
                    <span>
                      {[
                        t('wishes.dialog.episodes', { count: season.episodeCount }),
                        ...(season.airDate ? [season.airDate.slice(0, 4)] : []),
                      ].join(t('format.separator'))}
                    </span>
                    {season.state === 'requested' && (
                      <Chip tone="neutral">{t('wishes.dialog.seasonState.requested')}</Chip>
                    )}
                  </li>
                ))}
              </ul>
            </section>
          )}
        </DetailBody>

        {requesting && (
          <Suspense fallback={null}>
            <RequestDialog
              title={title}
              source="detail"
              autoApprove={title.request.autoApprove}
              onClose={() => setRequesting(false)}
              onDone={(made) => {
                setRequesting(false);
                setDone(made);
                // The big button turns into the state and keeps the focus (concept 25, 4.1).
                setSpatialFocus(REQUEST_KEY);
              }}
              onAvailable={toLibrary}
            />
          </Suspense>
        )}
        {withdrawAsk && openWish.data && (
          <Suspense fallback={null}>
            <WithdrawDialog
              title={title.title}
              joined={openWish.data.role === 'joined'}
              onClose={() => setWithdrawAsk(false)}
              onConfirm={() => {
                const wishId = openWish.data?.id;
                setWithdrawAsk(false);
                setDone(null);
                join.reset();
                // The card goes with the wish: the focus waits on the big button.
                setSpatialFocus(REQUEST_KEY);
                if (wishId !== undefined) withdraw.mutate(wishId);
              }}
            />
          </Suspense>
        )}
        {trailerOpen && (
          <Suspense fallback={null}>
            <TitleTrailer
              title={title.title}
              trailers={title.trailers}
              onClose={() => setTrailerOpen(false)}
            />
          </Suspense>
        )}
      </DetailPage>
    </SpatialPage>
  );
}

/** «Dein letzter Wunsch: Abgelehnt» with its date and reason, under the head (concept 25, 2). */
function LastWish({
  last,
  language,
}: {
  last: NonNullable<ExternalTitle['lastWish']>;
  language: string;
}) {
  const { t } = useTranslation();
  const why = last.reason
    ? failText(t, last.reason)
    : last.state === 'declined' || last.state === 'withdrawn'
      ? t(`titles.last.why.${last.state}`)
      : '';
  return (
    <div className={itemStyles.block}>
      <Notice
        tone={last.state === 'withdrawn' || last.state === 'available' ? 'info' : 'warning'}
        title={t('titles.last.title', { state: t(`wishes.state.${last.state}`) })}
      >
        {[formatDate(last.at, language), why].filter(Boolean).join(t('format.separator'))}
      </Notice>
    </div>
  );
}
