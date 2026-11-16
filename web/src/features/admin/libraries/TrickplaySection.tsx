import type { Library, LibraryTrickplay } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { formatBytes } from '@/lib/format';
import { Chip, Notice, Skeleton } from '@/ui';
import { AriaButton, DisclosureGroup } from '@/ui/aria';
import { intlLocale } from '@/i18n';
import { runJobType } from '../queries';
import { sameTrickplay, trickplayStores, TrickplayFields } from './TrickplayFields';
import styles from './Libraries.module.css';

/** The job type that makes the missing previews of every library «Im Voraus» (6.3, F-5). */
export const TRICKPLAY_BACKFILL_TYPE = 'trickplay.backfill';

/** The setting of one kind in a word: «Aus», «Im Voraus», «Beim Start, ohne Speichern» … */
export function trickplayModeText(
  t: TFunction,
  value: { mode: LibraryTrickplay['series']['mode']; keep: boolean },
): string {
  return value.mode === 'on_start' && !value.keep
    ? t('admin.libraries.trickplay.summaryNoKeep')
    : t(`admin.libraries.trickplay.mode.${value.mode}`);
}

interface TrickplaySectionProps {
  value: LibraryTrickplay;
  onChange: (value: LibraryTrickplay) => void;
  /** The stored library («Bearbeiten»): its estimate, its saved setting and «Jetzt erzeugen». */
  library?: Library;
  isExpanded: boolean;
  onExpandedChange: (expanded: boolean) => void;
}

/**
 * «Vorschau beim Spulen» of a video library in the library form (concept 18, 9; concept 24 4.1):
 * a section that starts closed, with «Filme: … · Serien: …» and «geändert» in its header. Of a
 * stored library it keeps what the former dialog had: the estimate of `getLibrary` (or the line
 * «Schätzung nicht verfügbar»), the warning when the change drops stored previews (F-12) and «Jetzt
 * erzeugen», which queues the backfill of every library «Im Voraus» at once. That button works on
 * the saved setting, so it is disabled while the section holds unsaved changes, with the reason
 * beside it (Z-5).
 */
export function TrickplaySection({
  value,
  onChange,
  library,
  isExpanded,
  onExpandedChange,
}: TrickplaySectionProps) {
  const { t, i18n } = useTranslation();
  const saved = library?.trickplay;
  const detail = useQuery({
    queryKey: ['admin', 'library', library?.id],
    queryFn: ({ signal }): Promise<Library> =>
      request(
        api.GET('/admin/libraries/{id}', { params: { path: { id: library?.id ?? 0 } }, signal }),
      ),
    enabled: library !== undefined,
  });
  const estimate = detail.data?.trickplayEstimate;
  const generate = useMutation({ mutationFn: () => runJobType(TRICKPLAY_BACKFILL_TYPE) });
  const changed = saved !== undefined && !sameTrickplay(value, saved);

  // Stored previews of a kind go when its setting stops storing (6.6).
  const drops = (kind: keyof LibraryTrickplay) =>
    saved !== undefined && trickplayStores(saved[kind]) && !trickplayStores(value[kind]);
  const size = (kind: keyof LibraryTrickplay) =>
    estimate ? formatBytes(estimate[kind].storedBytes, intlLocale(i18n.language)) : '–';
  const warning =
    drops('movies') && drops('series')
      ? t('admin.libraries.trickplay.drop.both', { movies: size('movies'), series: size('series') })
      : drops('movies')
        ? t('admin.libraries.trickplay.drop.movies', { size: size('movies') })
        : drops('series')
          ? t('admin.libraries.trickplay.drop.series', { size: size('series') })
          : null;
  // The backfill makes previews only for settings «Im Voraus» that are saved.
  const canGenerate =
    saved !== undefined && (saved.movies.mode === 'ahead' || saved.series.mode === 'ahead');

  return (
    <DisclosureGroup
      title={t('admin.libraries.trickplay.action')}
      isExpanded={isExpanded}
      onExpandedChange={onExpandedChange}
      meta={
        <>
          <span>
            {t('admin.libraries.trickplay.header', {
              movies: trickplayModeText(t, value.movies),
              series: trickplayModeText(t, value.series),
            })}
          </span>
          {changed && <Chip tone="warning">{t('admin.libraries.form.changed')}</Chip>}
        </>
      }
    >
      <div className={styles.trickplaySection}>
        <p className={styles.muted}>{t('admin.libraries.trickplay.description')}</p>
        <TrickplayFields value={value} onChange={onChange} estimate={estimate} />
        {library && detail.isPending && <Skeleton shape="line" />}
        {detail.isError && (
          <Notice tone="warning" live={false}>
            {t('admin.libraries.trickplay.estimate.unavailable')}
          </Notice>
        )}
        {warning && (
          <Notice tone="warning" live={false}>
            {warning}
          </Notice>
        )}
        {canGenerate && (
          <div className={styles.trickplayGenerate}>
            <AriaButton
              icon="refresh"
              isDisabled={changed}
              isPending={generate.isPending}
              onPress={() => generate.mutate()}
            >
              {t('admin.libraries.trickplay.generate.action')}
            </AriaButton>
            <span className={styles.muted}>
              {changed
                ? t('admin.libraries.trickplay.generate.unsaved')
                : t('admin.libraries.trickplay.generate.hint')}
            </span>
          </div>
        )}
        {generate.isSuccess && <Notice>{t('admin.libraries.trickplay.generate.queued')}</Notice>}
        {generate.isError && <Notice tone="danger">{problemMessage(t, generate.error)}</Notice>}
      </div>
    </DisclosureGroup>
  );
}
