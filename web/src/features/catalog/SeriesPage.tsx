import type { components } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatDuration } from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ButtonLink,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  VolumeRow,
} from '@/ui';
import { BACK_KEY, DetailBody, DetailHeader, DetailPage } from './DetailHeader';
import { CoverPen, EDITABLE_KINDS, EditMetadata } from './EditMetadata';
import { isFinished } from './labels';
import { CoverWatched } from './watched';
import { audioSeriesQuery } from './queries';
import { useBackFocus } from './useBackFocus';
import itemStyles from './ItemPage.module.css';
import styles from './SeriesPage.module.css';

type AudioSeries = components['schemas']['AudioSeries'];
type AudioSeriesVolume = components['schemas']['AudioSeriesVolume'];

const route = getRouteApi('/_authed/_active/_chrome/libraries/$libraryId/series/$seriesId');

// «Reihe bearbeiten» needs react-aria, which the browsing pages do not load up front (P-10).
const SeriesEditDialog = lazy(() =>
  import('./SeriesEditDialog').then((module) => ({ default: module.SeriesEditDialog })),
);

const PLAY_KEY = 'series-play';
const EDIT_KEY = 'series-edit';
const VOLUME_KEY = 'series-volume';

/** Where the dismissal of the admin hint is kept: per series, for one set of findings. */
const hintStorageKey = (seriesId: string) => `slouchcat.series-hint.${seriesId}`;

const readHint = (seriesId: string): string | null => {
  try {
    return window.localStorage.getItem(hintStorageKey(seriesId));
  } catch {
    return null;
  }
};

const storeHint = (seriesId: string, signature: string) => {
  try {
    window.localStorage.setItem(hintStorageKey(seriesId), signature);
  } catch {
    // Without storage the hint comes back with the next visit.
  }
};

/** The numbers that more than one visible volume carries (OD-5), in the server's order. */
const duplicateNumbers = (volumes: readonly AudioSeriesVolume[]): number[] => {
  const seen = new Set<number>();
  const twice: number[] = [];
  for (const { seriesIndex } of volumes) {
    if (seriesIndex === undefined) continue;
    if (seen.has(seriesIndex) && !twice.includes(seriesIndex)) twice.push(seriesIndex);
    seen.add(seriesIndex);
  }
  return twice;
};

/** «Weiterhören: Band n» with `next` (the server's choice); «Abspielen» before any progress. */
function playTarget(t: TFunction, series: AudioSeries): { label: string; href: string } | null {
  const next = series.next;
  if (!next) return null;
  const started = series.volumes.some(
    ({ item }) => item.progress && (item.progress.completed || item.progress.positionMs > 0),
  );
  if (next.progress?.completed) {
    return { label: t('item.action.againListening'), href: paths.playAt(next.id, 0) };
  }
  if (!started) return { label: t('item.play'), href: paths.play(next.id) };
  const volume = series.volumes.find(({ item }) => item.id === next.id)?.seriesIndex;
  return {
    label:
      volume !== undefined
        ? t('audioSeries.continue', { volume })
        : t('audioSeries.continueTitle', { title: next.title }),
    href: paths.play(next.id),
  };
}

/**
 * The page of an audiobook series (concept 19 D4.3): the head with the stacked cover, name,
 * author, narrator (when every volume has the same), number of volumes and total length,
 * «Weiterhören» and for the administrator «Reihe bearbeiten»; then the volumes in the server's
 * order with their progress, a play button and the pen. Back returns to the library.
 */
export function SeriesPage() {
  const { libraryId, seriesId } = route.useParams();
  const id = Number(libraryId);
  return (
    <SeriesView
      key={`${libraryId}-${seriesId}`}
      libraryId={Number.isSafeInteger(id) && id > 0 ? id : null}
      seriesId={seriesId}
    />
  );
}

function SeriesView({ libraryId, seriesId }: { libraryId: number | null; seriesId: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const query = useQuery({
    ...audioSeriesQuery(libraryId ?? 0, seriesId),
    enabled: libraryId !== null,
  });
  const goBack = useGoBack(libraryId === null ? paths.home : paths.library(libraryId));
  useBackKeys(goBack);
  // The administrator's primary profile edits; a shortcut, not a rule (the server decides).
  const session = useActiveSession();
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  const [editing, setEditing] = useState<string | null>(null);
  const [renaming, setRenaming] = useState(false);
  const [dismissed, setDismissed] = useState(() => readHint(seriesId));
  const series = isApiError(query.error, 404) ? undefined : query.data;
  const volumes = series?.volumes ?? [];
  useBackFocus(
    VOLUME_KEY,
    volumes.map(({ item }) => `${VOLUME_KEY}-${item.id}`),
  );
  const notFound = libraryId === null || isApiError(query.error, 404);
  useDocumentTitle(series?.name ?? (notFound ? t('audioSeries.notFound.title') : undefined));

  if (notFound || (!series && query.isError)) {
    return (
      <main className={`${itemStyles.detail} ${itemStyles.message}`}>
        <Notice
          tone={notFound ? 'info' : 'danger'}
          title={notFound ? t('audioSeries.notFound.title') : problemMessage(t, query.error)}
          actions={
            notFound ? (
              <ButtonLink
                href={libraryId === null ? paths.home : paths.library(libraryId)}
                variant="primary"
              >
                {t('audioSeries.notFound.action')}
              </ButtonLink>
            ) : (
              <Button icon="refresh" onPress={() => void query.refetch()}>
                {t('common.retry')}
              </Button>
            )
          }
        >
          {notFound && t('audioSeries.notFound.body')}
        </Notice>
      </main>
    );
  }

  if (!series || libraryId === null) {
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

  const target = playTarget(t, series);
  const facts = [
    series.author,
    series.narrator ? t('audioSeries.narratedBy', { narrator: series.narrator }) : undefined,
    t('audioSeries.volumes', { count: volumes.length }),
    series.durationMs > 0 ? formatDuration(t, series.durationMs) : undefined,
  ]
    .filter(Boolean)
    .join(t('format.separator'));

  // OD-5, for the administrator only: put together from several spellings, or a number twice.
  const twice = duplicateNumbers(volumes);
  const findings: string[] = [];
  if (series.spellings.length > 1) {
    findings.push(
      t('audioSeries.hint.spellings', {
        names: series.spellings
          .map((spelling) =>
            t('audioSeries.hint.spelling', { name: spelling.name, count: spelling.count }),
          )
          .join(t('format.listSeparator')),
      }),
    );
  }
  if (twice.length > 0) {
    findings.push(
      t('audioSeries.hint.duplicates', {
        volumes: twice
          .map((value) => t('audioSeries.number', { value }))
          .join(t('format.listSeparator')),
      }),
    );
  }
  const signature = JSON.stringify([series.spellings.map((spelling) => spelling.name), twice]);
  const showHint = canManage && findings.length > 0 && dismissed !== signature;

  return (
    <SpatialPage defaultFocusKey={target ? PLAY_KEY : BACK_KEY}>
      <DetailPage>
        <DetailHeader
          images={series.cover?.images ?? {}}
          accent={series.cover?.accent}
          cover="square"
          stack
          onBack={goBack}
          belowBack={target ? PLAY_KEY : undefined}
          kicker={t('audioSeries.kicker')}
          title={series.name}
          facts={<p className={itemStyles.muted}>{facts}</p>}
          actions={
            (target || canManage) && (
              <SpatialGroup className={styles.actions}>
                {target && (
                  <SpatialButtonLink
                    focusKey={PLAY_KEY}
                    href={target.href}
                    variant="primary"
                    size="lg"
                    icon="play"
                  >
                    {target.label}
                  </SpatialButtonLink>
                )}
                {canManage && (
                  <SpatialButton
                    focusKey={EDIT_KEY}
                    variant="secondary"
                    size="lg"
                    icon="pen"
                    onPress={() => setRenaming(true)}
                  >
                    {t('audioSeries.edit')}
                  </SpatialButton>
                )}
              </SpatialGroup>
            )
          }
        />

        <DetailBody>
          {showHint && (
            <div className={itemStyles.gutter}>
              <Notice
                tone="warning"
                title={t('audioSeries.hint.title')}
                actions={
                  <SpatialGroup>
                    <SpatialButton
                      variant="secondary"
                      onPress={() => {
                        storeHint(series.id, signature);
                        setDismissed(signature);
                      }}
                    >
                      {t('audioSeries.hint.dismiss')}
                    </SpatialButton>
                  </SpatialGroup>
                }
              >
                {findings.map((finding) => (
                  <p key={finding}>{finding}</p>
                ))}
              </Notice>
            </div>
          )}

          <section className={itemStyles.block} aria-labelledby="series-volumes-heading">
            <h2 id="series-volumes-heading" className={itemStyles.sectionTitle}>
              {t('audioSeries.list')}
            </h2>
            {/* The server's order (D4.3): by number, 0.5 before 1, without a number last. */}
            <SpatialGroup as="ul" className={styles.volumes} aria-label={t('audioSeries.list')}>
              {volumes.map(({ item, seriesIndex, narrator }) => {
                // Heard (the check, `watchedAt`) and a running position (what plays next,
                // `completed`) apart: a volume heard once and started again is both (concept 20
                // section 6.3).
                const finished = isFinished(item);
                const progress = item.progress;
                const started = progress && !progress.completed && progress.positionMs > 0;
                const caption = [
                  // The narrator where the volumes differ (a reading and a radio play).
                  series.narrator === undefined ? narrator : undefined,
                  finished
                    ? t('library.status.heard')
                    : started
                      ? t('audioSeries.left', {
                          left: formatDuration(
                            t,
                            Math.max(0, progress.durationMs - progress.positionMs),
                          ),
                        })
                      : undefined,
                ]
                  .filter(Boolean)
                  .join(t('format.separator'));
                return (
                  <li key={item.id}>
                    <VolumeRow
                      focusKey={`${VOLUME_KEY}-${item.id}`}
                      href={paths.item(item.id)}
                      number={
                        seriesIndex !== undefined
                          ? t('audioSeries.number', { value: seriesIndex })
                          : undefined
                      }
                      label={
                        seriesIndex !== undefined
                          ? t('audioSeries.volumeLabel', { volume: seriesIndex, title: item.title })
                          : undefined
                      }
                      title={item.title}
                      caption={caption || undefined}
                      coverUrl={imageUrl(item.images.posterSmall ?? item.images.poster)}
                      accent={item.accent}
                      watchedLabel={finished ? t('library.status.heard') : undefined}
                      progress={
                        started
                          ? {
                              percent: progress.percent,
                              label: t('item.listened', { percent: Math.round(progress.percent) }),
                            }
                          : undefined
                      }
                      duration={item.durationMs ? formatDuration(t, item.durationMs) : undefined}
                      playHref={
                        progress?.completed ? paths.playAt(item.id, 0) : paths.play(item.id)
                      }
                      playLabel={t('audioSeries.play', { title: item.title })}
                      tool={
                        canManage && EDITABLE_KINDS.has(item.kind) ? (
                          <CoverPen item={item} onEdit={setEditing} />
                        ) : undefined
                      }
                      // The check mark of the volume (concept 20 P-5); the volume itself never
                      // asks whether to resume (O-9).
                      watchedControl={<CoverWatched item={item} appearance="row" />}
                    />
                  </li>
                );
              })}
            </SpatialGroup>
          </section>
        </DetailBody>

        {editing && <EditMetadata itemId={editing} onClose={() => setEditing(null)} />}
        {renaming && (
          <Suspense fallback={null}>
            <SeriesEditDialog
              series={series}
              onClose={() => setRenaming(false)}
              onSaved={(saved) => {
                setRenaming(false);
                // Merged into another series: its address from now on.
                if (saved.id !== series.id) {
                  void navigate({ href: paths.audioSeries(libraryId, saved.id), replace: true });
                }
              }}
            />
          </Suspense>
        )}
      </DetailPage>
    </SpatialPage>
  );
}
