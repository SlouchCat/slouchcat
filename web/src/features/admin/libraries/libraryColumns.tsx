import type { Library, RatingSystem } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { formatDateTime } from '@/lib/format';
import { Chip } from '@/ui';
import { type DataColumn } from '@/ui/aria';
import styles from './Libraries.module.css';
import { ratingLabel } from '../../shared/choices';
import { intlLocale } from '@/i18n';

/** The scan state as a badge in the tone of its state (redesign, build plan 2.7). */
const toneOf: Record<
  Library['scan']['state'],
  'outline' | 'info' | 'success' | 'danger' | 'warning'
> = {
  never: 'outline',
  queued: 'info',
  running: 'info',
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
      <span>
        <Chip tone={toneOf[scan.state]}>{t(`admin.libraries.state.${scan.state}`)}</Chip>
      </span>
      {scan.finishedAt && (
        <span className={styles.muted}>
          {t('admin.libraries.finishedAt', {
            date: formatDateTime(scan.finishedAt, intlLocale(i18n.language)),
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
      {library.roots.length === 0 && library.kind === 'podcasts' ? (
        // A podcast library takes its titles from feeds; «0 Ordner» would look like a fault.
        <span className={styles.muted}>{t('admin.libraries.feedsOnly')}</span>
      ) : library.roots.length === 1 && first ? (
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

/** The columns of a library table: the row folds out on the name (Bib-C). */
export function libraryColumns(t: TFunction, ratingSystem?: RatingSystem): DataColumn<Library>[] {
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
                  rating: ratingLabel(t, library.defaultRating, ratingSystem),
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
  ];
}
