import type { Library } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { formatDateTime } from '@/lib/format';
import { Chip, Notice, Panel, ProgressBar, StatusDot, type StatusDotState } from '@/ui';
import { providerChain } from '@/ui/aria';
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

const watchKeys = {
  inotify: 'admin.libraries.watch.inotify',
  poll: 'admin.libraries.watch.poll',
  inotify_poll: 'admin.libraries.watch.inotify_poll',
} as const;

function errorText(t: TFunction, code: string): string {
  return code in knownErrors
    ? t(knownErrors[code as keyof typeof knownErrors])
    : t('admin.libraries.error.other', { message: code });
}

interface LibraryCardProps {
  library: Library;
  actions: ReactNode;
}

export function LibraryCard({ library, actions }: LibraryCardProps) {
  const { t, i18n } = useTranslation();
  const { scan } = library;
  const headingId = `library-${library.id}`;
  const showProgress = scan.state === 'running' && (scan.filesTotal ?? 0) > 0;
  const progress = showProgress
    ? Math.min(100, ((scan.filesSeen ?? 0) / (scan.filesTotal ?? 1)) * 100)
    : 0;

  return (
    <Panel as="article" aria-labelledby={headingId}>
      <div className={styles.cardHead}>
        <h2 id={headingId} className={styles.cardTitle}>
          {library.name}
        </h2>
        <Chip tone="info">{t(`library.kind.${library.kind}`)}</Chip>
        {library.defaultRating !== undefined && (
          <Chip tone="outline">
            {t('admin.libraries.defaultRating', {
              rating: t('rating.fsk', { age: library.defaultRating }),
            })}
          </Chip>
        )}
      </div>

      <div className={styles.state}>
        <span className={styles.stateLine}>
          <StatusDot state={dotOf[scan.state]} />
          <span className={styles.stateLabel}>{t(`admin.libraries.state.${scan.state}`)}</span>
          <span className={styles.muted}>
            {t('admin.libraries.counts', { items: scan.itemCount, files: scan.fileCount })}
          </span>
        </span>
        {showProgress && (
          <div className={styles.scanProgress}>
            <ProgressBar
              value={progress}
              size="regular"
              label={t('admin.libraries.progress', {
                seen: scan.filesSeen ?? 0,
                total: scan.filesTotal ?? 0,
              })}
            />
            <span className={styles.muted}>
              {t('admin.libraries.progress', {
                seen: scan.filesSeen ?? 0,
                total: scan.filesTotal ?? 0,
              })}
            </span>
          </div>
        )}
        {scan.finishedAt && (
          <span className={styles.muted}>
            {t('admin.libraries.finishedAt', {
              date: formatDateTime(scan.finishedAt, i18n.language),
            })}
          </span>
        )}
      </div>

      {scan.lastError && (scan.state === 'failed' || scan.state === 'aborted') && (
        <Notice
          tone={
            scan.lastError === 'cancelled'
              ? 'info'
              : scan.state === 'aborted'
                ? 'warning'
                : 'danger'
          }
        >
          {errorText(t, scan.lastError)}
        </Notice>
      )}

      <Folders library={library} />

      <div className={styles.actions}>{actions}</div>
    </Panel>
  );
}

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

/**
 * The folders of a library in one line: the path with its content and provider list when there
 * is one, a summary per content («1 Filme, 1 Serien») when there are more (idea 02, E16; ADR
 * 0015). The list itself and the way in and out live in the folders dialog, so a library with
 * five folders does not push everything else off the card.
 */
function Folders({ library }: { library: Library }) {
  const { t } = useTranslation();
  const [first] = library.roots;

  if (library.roots.length === 0) {
    // A podcast library never had a folder: its titles come from feeds (M6). A video or audio
    // library can lose its last one (idea 02, E13), and then it needs its own sentence -- the
    // podcast text would be wrong, and silence would look like a bug.
    return (
      <p className={styles.muted}>
        {library.kind === 'podcasts' ? t('admin.libraries.noRoots') : t('admin.libraries.noFolder')}
      </p>
    );
  }
  return (
    <div className={styles.roots}>
      <h3 className={styles.rootsTitle}>{t('admin.libraries.roots')}</h3>
      {library.roots.length === 1 && first ? (
        <div className={styles.root}>
          <code className={styles.path}>{first.path}</code>
          <span className={styles.rootMeta}>
            {/* Audio libraries have one content, named by the kind chip above already. */}
            {library.kind === 'video' && (
              <Chip tone="outline">{t(`library.content.${first.content}`)}</Chip>
            )}
            <span className={styles.muted}>{providerChain(t, first.providers)}</span>
            <Chip tone="info">{t('admin.libraries.fsType', { type: first.fsType })}</Chip>
            {first.warning === 'network_share' && (
              <Chip tone="warning" icon="warning">
                {t('admin.libraries.warning.network_share')}
              </Chip>
            )}
            <span className={styles.muted}>{t(watchKeys[first.watch])}</span>
          </span>
        </div>
      ) : (
        <div className={styles.root}>
          <span>
            {t('admin.libraries.rootCount', { count: library.roots.length })}
            <span className={styles.muted}>{t('format.separator') + rootSummary(t, library)}</span>
          </span>
          <span className={styles.rootMeta}>
            {library.roots.some((root) => root.warning === 'network_share') && (
              <Chip tone="warning" icon="warning">
                {t('admin.libraries.warning.network_share')}
              </Chip>
            )}
          </span>
        </div>
      )}
    </div>
  );
}
