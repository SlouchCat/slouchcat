import type { Collection, ItemRef } from '@slouchcat/api-client';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { BACK_KEY, DetailBody, DetailHeader, DetailPage } from '@/features/catalog/DetailHeader';
import { EDITABLE_KINDS, useCoverPens } from '@/features/catalog/EditMetadata';
import { isAudioKind, isFinished } from '@/features/catalog/labels';
import { CoverWatched, useCollectionWatchedToggle } from '@/features/catalog/watched';
import { formatRuntime } from '@/lib/format';
import { usePagedList, wholeRange } from '@/lib/pagedList';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { useOnline } from '@/lib/useOnline';
import {
  Button,
  ButtonLink,
  Notice,
  ProgressBar,
  RoundAction,
  Skeleton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  VolumeRow,
} from '@/ui';
import { collectionYears } from './collectionFacts';
import { PageRetry } from './ItemGrid';
import { collectionSource, useWatchlistMutation } from './queries';
import itemStyles from '@/features/catalog/ItemPage.module.css';
import styles from './Discover.module.css';

const route = getRouteApi('/_authed/_active/_chrome/collections/$collectionId');

const PLAY_KEY = 'collection-play';
const WATCHLIST_KEY = 'collection-watchlist';
const WATCHED_KEY = 'collection-watched';
const PART_KEY = 'collection-part';
const WATCHED_PRESSED = { none: false, half: 'mixed', full: true } as const;

/** A series or podcast in a collection: it plays from its own page (its next episode). */
const isGroup = (item: ItemRef) => item.kind === 'series' || item.kind === 'podcast';

/** A part begun and not finished: a running position (concept 20 section 6.3). */
const isRunning = (item: ItemRef) =>
  isGroup(item)
    ? (item.watchedCount ?? 0) > 0 && item.watched !== true
    : item.progress !== undefined && !item.progress.completed && item.progress.positionMs > 0;

/**
 * The big button (concept 21 P-10): the part the server names next (`next`, the first one not
 * finished), «Weiterschauen · Teil N» when it is begun, «Nochmals ansehen · Teil 1» when every
 * part is finished, else «Abspielen · Teil N». A series plays from its own page.
 */
export function playTarget(
  t: TFunction,
  collection: Pick<Collection, 'next' | 'nextPart'>,
): { label: string; href: string; resume: boolean; again: boolean } | null {
  const { next, nextPart: part } = collection;
  if (!next || part === undefined) return null;
  const again = isGroup(next) ? next.watched === true : next.progress?.completed === true;
  const resume = !again && isRunning(next);
  const label = t(again ? 'collection.rewatch' : resume ? 'collection.resume' : 'collection.play', {
    part,
  });
  if (isGroup(next)) return { label, href: paths.item(next.id), resume, again };
  return { label, href: again ? paths.playAt(next.id, 0) : paths.play(next.id), resume, again };
}

/**
 * A collection (E-6, concept 21 P-10 variant A): the head of a detail page with the first part's
 * backdrop and the collection's poster, name, «8 Filme · 2001–2011 · zusammen 19 h 38 min», «x von
 * n gesehen», the big button for the next part and the round buttons «Alle merken» and «Gesehen»
 * (every part, O-19), the TMDB description (O-18); then the parts 1…N in collection order with
 * poster, title, year, length and check mark, the next one marked.
 */
export function CollectionPage() {
  const { collectionId } = route.useParams();
  return <CollectionView key={collectionId} id={collectionId} />;
}

function CollectionView({ id }: { id: string }) {
  const { t } = useTranslation();
  const pens = useCoverPens();
  // Every part at once: the list is short, the next part and «Alle merken» need them all.
  const paged = usePagedList(collectionSource(id), wholeRange);
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

  const parts = paged.slots.filter((slot): slot is ItemRef => slot !== undefined);
  return (
    <SpatialPage defaultFocusKey={collection.next ? PLAY_KEY : BACK_KEY}>
      <DetailPage>
        <CollectionHead collection={collection} parts={parts} onBack={goBack} />
        <DetailBody>
          <section className={styles.members} aria-labelledby="collection-parts">
            <h2 id="collection-parts" className={styles.sectionTitle}>
              {t('collection.parts')}
            </h2>
            <SpatialGroup as="ul" className={styles.parts} aria-label={t('collection.parts')}>
              {paged.slots.map((part, index) =>
                part ? (
                  <li key={part.id}>
                    <PartRow
                      part={part}
                      number={index + 1}
                      next={collection.next?.id === part.id && collection.nextPart === index + 1}
                      tool={EDITABLE_KINDS.has(part.kind) ? pens.tool?.(part) : undefined}
                    />
                  </li>
                ) : null,
              )}
            </SpatialGroup>
            {pens.editor}
            {paged.failed && <PageRetry onRetry={paged.retry} />}
          </section>
        </DetailBody>
      </DetailPage>
    </SpatialPage>
  );
}

interface CollectionHeadProps {
  collection: Collection;
  /** The parts read so far (every one once the list is complete). */
  parts: readonly ItemRef[];
  onBack: () => void;
}

/** The head: facts, progress, the big button, «Alle merken», «Gesehen» and the description. */
function CollectionHead({ collection, parts, onBack }: CollectionHeadProps) {
  const { t } = useTranslation();
  const online = useOnline();
  const first = parts[0];
  const watched = useCollectionWatchedToggle({
    id: collection.id,
    name: collection.name,
    total: collection.total,
    seenCount: collection.seenCount,
  });
  const watchlist = useWatchlistMutation();
  // «Alle merken»: pressed while every part is on the watchlist; a press adds the others, or
  // takes every part off when all are on it.
  const allListed = parts.length > 0 && parts.every((part) => part.inWatchlist);
  const listAll = () => {
    for (const part of parts) {
      if (part.inWatchlist !== !allListed) watchlist.mutate({ id: part.id, on: !allListed });
    }
  };
  const target = playTarget(t, collection);
  const films = parts.length === collection.total && parts.every((part) => part.kind === 'movie');
  const facts = [
    t(films ? 'collection.films' : 'collection.count', { count: collection.total }),
    collectionYears(t, collection),
    collection.runtimeMinutes
      ? t('collection.together', { duration: formatRuntime(t, collection.runtimeMinutes) })
      : undefined,
  ]
    .filter(Boolean)
    .join(t('format.separator'));
  const seen = collection.seenCount ?? 0;

  return (
    <DetailHeader
      images={first?.images ?? {}}
      ownPoster={collection.image}
      accent={first?.accent}
      cover={first && isAudioKind(first.kind) ? 'square' : 'poster'}
      stack
      onBack={onBack}
      belowBack={target ? PLAY_KEY : undefined}
      title={collection.name}
      facts={
        <>
          <p className={styles.muted}>{facts}</p>
          {collection.started && (
            <div className={styles.seenLine}>
              <span className={styles.seenBar}>
                <ProgressBar
                  value={collection.total > 0 ? (seen / collection.total) * 100 : 0}
                  label={t('collection.seen', { count: seen, total: collection.total })}
                />
              </span>
              <span aria-hidden="true">
                {t('collection.seen', { count: seen, total: collection.total })}
              </span>
            </div>
          )}
        </>
      }
      actions={
        <>
          {target && (
            <SpatialGroup className={itemStyles.play}>
              <SpatialButtonLink
                focusKey={PLAY_KEY}
                href={target.href}
                variant="primary"
                size="lg"
                icon="play"
              >
                {target.label}
              </SpatialButtonLink>
            </SpatialGroup>
          )}
          <SpatialGroup ordered className={itemStyles.roundActions}>
            <RoundAction
              focusKey={WATCHLIST_KEY}
              icon="plus"
              label={t(allListed ? 'collection.watchlist.added' : 'collection.watchlist.add')}
              pressed={allListed}
              isDisabled={!online || parts.length === 0}
              busy={watchlist.isPending}
              onPress={listAll}
            />
            <RoundAction
              focusKey={WATCHED_KEY}
              icon="check"
              label={watched.buttonLabel}
              pressed={WATCHED_PRESSED[watched.state]}
              description={watched.hint}
              isDisabled={watched.readOnly}
              busy={watched.busy}
              onPress={watched.press}
            />
          </SpatialGroup>
          {watchlist.error && <Notice tone="danger" title={problemMessage(t, watchlist.error)} />}
          {watched.error && <Notice tone="danger" title={watched.error} />}
          {watched.dialog}
        </>
      }
      overview={collection.overview}
      overviewNote={collection.overviewFallback ? t('collection.overviewFallback') : undefined}
    />
  );
}

interface PartRowProps {
  part: ItemRef;
  /** The place in collection order, from 1. */
  number: number;
  /** The part the big button plays. */
  next: boolean;
  tool?: ReactNode;
}

/** One part: number, poster, the marker of the next one, title, year, length and check mark. */
function PartRow({ part, number, next, tool }: PartRowProps) {
  const { t } = useTranslation();
  const finished = isFinished(part);
  const running = isRunning(part);
  const progress = part.progress;
  return (
    <VolumeRow
      focusKey={`${PART_KEY}-${part.id}`}
      href={paths.item(part.id)}
      number={String(number)}
      label={t('collection.part', { part: number, title: part.title })}
      title={part.title}
      marker={
        next && !finished ? t(running ? 'item.nextUp.resume' : 'item.nextUp.markNext') : undefined
      }
      caption={part.year !== undefined ? String(part.year) : undefined}
      coverUrl={imageUrl(part.images.posterSmall ?? part.images.poster)}
      shape={isAudioKind(part.kind) ? 'square' : 'poster'}
      accent={part.accent}
      watchedLabel={finished ? t('watched.state.full') : undefined}
      progress={
        running && progress && !isGroup(part)
          ? {
              percent: progress.percent,
              label: t('item.watched', { percent: Math.round(progress.percent) }),
            }
          : undefined
      }
      duration={
        part.runtimeMinutes && !isGroup(part) ? formatRuntime(t, part.runtimeMinutes) : undefined
      }
      playHref={
        isGroup(part)
          ? paths.item(part.id)
          : progress?.completed
            ? paths.playAt(part.id, 0)
            : paths.play(part.id)
      }
      playLabel={t('collection.playPart', { title: part.title })}
      tool={tool}
      watchedControl={<CoverWatched item={part} appearance="row" />}
    />
  );
}
