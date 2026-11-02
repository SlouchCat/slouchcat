import type { Library } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { formatDateTime } from '@/lib/format';
import { Chip, StatusDot, type StatusDotState } from '@/ui';
import { type DataColumn } from '@/ui/aria';
import { libraryMetadataQuery } from '../queries';
import styles from './LibrariesPage.module.css';

const dotOf: Record<Library['scan']['state'], StatusDotState> = {
  never: 'pending',
  queued: 'active',
  running: 'active',
  done: 'success',
  failed: 'danger',
  aborted: 'warning',
};

const knownErrors = {
  root_missing: 'admin.libraries.error.root_missing',
  root_empty: 'admin.libraries.error.root_empty',
  loss_over_threshold: 'admin.libraries.error.loss_over_threshold',
  cancelled: 'admin.libraries.error.cancelled',
} as const;

export const watchKeys = {
  inotify: 'admin.libraries.watch.inotify',
  poll: 'admin.libraries.watch.poll',
  inotify_poll: 'admin.libraries.watch.inotify_poll',
} as const;

/** The error of the last scan as a sentence; an unknown code is quoted. */
export function errorText(t: TFunction, code: string): string {
  return code in knownErrors
    ? t(knownErrors[code as keyof typeof knownErrors])
    : t('admin.libraries.error.other', { message: code });
}

/** Whether the scan ended with an error the fold-out shows in its band. */
export const hasScanError = (library: Library) =>
  Boolean(library.scan.lastError) &&
  (library.scan.state === 'failed' || library.scan.state === 'aborted');

/**
 * An aborted scan whose loss nobody has accepted yet (A2-N12): a cancelled scan needs none, and
 * an accepted one is left out like on the overview (concept 11).
 */
export const canAcceptLoss = (library: Library) =>
  library.scan.state === 'aborted' &&
  library.scan.lastError !== 'cancelled' &&
  !library.scan.lossAcceptedAt;

/** «1 Filme, 1 Serien»: how many folders of each content a library has (ADR 0015). */
export function rootSummary(t: TFunction, library: Library): string {
  const counts = new Map<string, number>();
  for (const root of library.roots) counts.set(root.content, (counts.get(root.content) ?? 0) + 1);
  return [...counts]
    .map(([content, count]) =>
      t('admin.libraries.rootSummary', {
        count,
        content: t(`library.content.${content}` as 'library.content.movie'),
      }),
    )
    .join(t('format.listSeparator'));
}

/** Scan state with the end of the last scan (BIB-1). */
function StateCell({ library }: { library: Library }) {
  const { t, i18n } = useTranslation();
  const { scan } = library;
  return (
    <span className={styles.cellStack}>
      <span className={styles.stateLine}>
        <StatusDot state={dotOf[scan.state]} />
        {t(`admin.libraries.state.${scan.state}`)}
      </span>
      {scan.finishedAt && (
        <span className={styles.muted}>
          {t('admin.libraries.finishedAt', {
            date: formatDateTime(scan.finishedAt, i18n.language),
          })}
        </span>
      )}
    </span>
  );
}

/**
 * The folders in one line (BIB-2, BIB-2a): the path of a single folder, cut to the column with
 * the whole path in the fold-out; the count and a summary per content for several; the network
 * warning when any of them is a share.
 */
function FoldersCell({ library }: { library: Library }) {
  const { t } = useTranslation();
  const [first] = library.roots;
  const share = library.roots.some((root) => root.warning === 'network_share');
  return (
    <span className={styles.cellStack}>
      {library.roots.length === 1 && first ? (
        <code className={`${styles.path} ${styles.oneLine}`} title={first.path}>
          {first.path}
        </code>
      ) : (
        <span>
          {t('admin.libraries.rootCount', { count: library.roots.length })}
          {library.roots.length > 1 && (
            <span className={styles.muted}>{t('format.separator') + rootSummary(t, library)}</span>
          )}
        </span>
      )}
      {share && (
        <span>
          <Chip tone="warning" icon="warning">
            {t('admin.libraries.warning.network_share')}
          </Chip>
        </span>
      )}
    </span>
  );
}

/**
 * The metadata deviation of a video library (concept 10, 7.2; BIB-3): «wie Server» or which part
 * deviates. The link to the page "Metadaten" sits in the fold-out. It asks while the page asks.
 */
function MetadataCell({ library, hold }: { library: Library; hold: boolean }) {
  const { t } = useTranslation();
  const metadata = useQuery({
    ...libraryMetadataQuery(library.id),
    enabled: library.kind === 'video',
    refetchOnWindowFocus: !hold,
  });
  if (library.kind !== 'video') return <>{t('admin.libraries.none')}</>;
  if (!metadata.data) return null;
  const texts = metadata.data.textLangs != null;
  const images = metadata.data.images != null;
  const key = texts && images ? 'both' : texts ? 'texts' : images ? 'images' : 'like';
  return <>{t(`admin.libraries.metadata.${key}`)}</>;
}

/** The columns of a library table: the row folds out on the name (Bib-C). */
export function libraryColumns(t: TFunction, hold: boolean): DataColumn<Library>[] {
  return [
    {
      key: 'name',
      header: t('admin.libraries.columns.library'),
      rowHeader: true,
      cell: (library) => library.name,
    },
    {
      key: 'kind',
      header: t('admin.libraries.columns.kind'),
      cell: (library) => (
        <span className={styles.cellStack}>
          <span>{t(`library.kind.${library.kind}`)}</span>
          {library.defaultRating !== undefined && (
            <span>
              <Chip tone="outline">
                {t('admin.libraries.defaultRating', {
                  rating: t('rating.fsk', { age: library.defaultRating }),
                })}
              </Chip>
            </span>
          )}
        </span>
      ),
    },
    {
      key: 'state',
      header: t('admin.libraries.columns.state'),
      cell: (library) => <StateCell library={library} />,
    },
    {
      key: 'counts',
      header: t('admin.libraries.columns.counts'),
      cell: (library) => (
        <span className={styles.nowrap}>
          {t('admin.libraries.counts', {
            items: library.scan.itemCount,
            files: library.scan.fileCount,
          })}
        </span>
      ),
    },
    {
      key: 'folders',
      header: t('admin.libraries.columns.folders'),
      cell: (library) => <FoldersCell library={library} />,
    },
    {
      key: 'metadata',
      header: t('admin.libraries.columns.metadata'),
      cell: (library) => <MetadataCell library={library} hold={hold} />,
    },
  ];
}
