import type { Library } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { formatDateTime } from '@/lib/format';
import { Chip, Notice, Panel, ProgressBar, StatusDot, type StatusDotState } from '@/ui';
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
        <Notice tone={scan.state === 'aborted' ? 'warning' : 'danger'}>
          {errorText(t, scan.lastError)}
        </Notice>
      )}

      <div className={styles.roots}>
        <h3 className={styles.rootsTitle}>{t('admin.libraries.roots')}</h3>
        <ul className={styles.rootList}>
          {library.roots.map((root) => (
            <li key={root.id} className={styles.root}>
              <code className={styles.path}>{root.path}</code>
              <span className={styles.rootMeta}>
                <Chip tone="info">{t('admin.libraries.fsType', { type: root.fsType })}</Chip>
                {root.warning === 'network_share' && (
                  <Chip tone="warning" icon="warning">
                    {t('admin.libraries.warning.network_share')}
                  </Chip>
                )}
                <span className={styles.muted}>{t(watchKeys[root.watch])}</span>
              </span>
            </li>
          ))}
        </ul>
      </div>

      <div className={styles.actions}>{actions}</div>
    </Panel>
  );
}
