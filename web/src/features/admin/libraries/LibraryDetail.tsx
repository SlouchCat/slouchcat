import type { Library } from '@slouchcat/api-client';
import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { Chip, Notice, ProgressBar, TextLink } from '@/ui';
import { ActionMenu, AriaButton, providerChain, type ActionMenuItem } from '@/ui/aria';
import { isAudioLibrary } from '../../shared/LibraryFields';
import { canScan, isScanning } from '../queries';
import { canAcceptLoss, errorText, hasScanError, watchKeys } from './libraryColumns';
import styles from './LibrariesPage.module.css';

/** The handlers of a fold-out, stable across renders so the fold-outs need not render again. */
export interface LibraryHandlers {
  onScan: (id: number) => void;
  onCancelScan: (id: number) => void;
  onFolders: (id: number) => void;
  onRating: (id: number) => void;
  onAcceptLoss: (library: Library) => void;
}

/**
 * The fold-out of a library (Bib-C): «Erneut scannen» or, while a scan runs, «Scan abbrechen»,
 * and «Ordner» as buttons; «Standardstufe ändern» (audio) and «Verlust akzeptieren» (danger) in
 * «Weitere Aktionen» (S-6); the link to its metadata (video); the progress of a running scan,
 * the error band of the last one, and every folder with its whole path (BIB-1 to BIB-4).
 */
export const LibraryDetail = memo(function LibraryDetail({
  library,
  scanPending,
  cancelPending,
  handlers,
}: {
  library: Library;
  scanPending: boolean;
  cancelPending: boolean;
  handlers: LibraryHandlers;
}) {
  const { t } = useTranslation();
  const { scan } = library;
  const scanning = isScanning(library);
  const showProgress = scan.state === 'running' && (scan.filesTotal ?? 0) > 0;
  const progress = showProgress
    ? Math.min(100, ((scan.filesSeen ?? 0) / (scan.filesTotal ?? 1)) * 100)
    : 0;
  const progressText = t('admin.libraries.progress', {
    seen: scan.filesSeen ?? 0,
    total: scan.filesTotal ?? 0,
  });

  const menu: ActionMenuItem[] = [
    ...(isAudioLibrary(library.kind)
      ? [{ id: 'rating', label: t('admin.libraries.changeRating.action') }]
      : []),
    ...(canAcceptLoss(library)
      ? [{ id: 'acceptLoss', label: t('admin.libraries.acceptLoss.action'), danger: true }]
      : []),
  ];
  const onMenu = (id: string) => {
    if (id === 'rating') handlers.onRating(library.id);
    else if (id === 'acceptLoss') handlers.onAcceptLoss(library);
  };

  return (
    <section className={styles.detail} aria-label={library.name}>
      <div className={styles.actions}>
        {/* One button that changes its part: the focus stays on it when a scan starts or ends. */}
        <AriaButton
          variant={scanning ? 'danger' : 'secondary'}
          icon={scanning ? 'close' : 'refresh'}
          isDisabled={!scanning && !canScan(library)}
          isPending={scanning ? cancelPending : scanPending}
          onPress={() =>
            scanning ? handlers.onCancelScan(library.id) : handlers.onScan(library.id)
          }
        >
          {scanning ? t('admin.libraries.cancel') : t('admin.libraries.rescan')}
        </AriaButton>
        <AriaButton
          icon="folder"
          aria-label={t('admin.libraries.openFoldersLabel', { name: library.name })}
          onPress={() => handlers.onFolders(library.id)}
        >
          {t('admin.libraries.openFolders')}
        </AriaButton>
        {menu.length > 0 && <ActionMenu rowName={library.name} items={menu} onAction={onMenu} />}
        {library.kind === 'video' && (
          <TextLink href={`${paths.adminMetadata}?library=${library.id}`}>
            {t('admin.libraries.metadata.link')}
          </TextLink>
        )}
      </div>

      {showProgress && (
        <div className={styles.scanProgress}>
          <ProgressBar value={progress} size="regular" label={progressText} />
          <span className={styles.muted}>{progressText}</span>
        </div>
      )}

      {hasScanError(library) && scan.lastError && (
        <Notice
          live={false}
          tone={
            scan.lastError === 'cancelled'
              ? 'info'
              : scan.state === 'aborted'
                ? 'warning'
                : 'danger'
          }
        >
          <span className={styles.wrap}>{errorText(t, scan.lastError)}</span>
        </Notice>
      )}

      <Folders library={library} />
    </section>
  );
});

/**
 * Every folder of the library with its whole path, content, provider list, file system, network
 * warning, watch mode and counts (BIB-2). Adding, changing and removing stay in the folders
 * dialog (BIB-5). A library without a folder says why: podcasts take their titles from feeds, a
 * video or audio library that lost its last folder scans nothing (idea 02, E13).
 */
function Folders({ library }: { library: Library }) {
  const { t } = useTranslation();
  return (
    <div className={styles.roots}>
      <h3 className={styles.rootsTitle}>{t('admin.libraries.roots')}</h3>
      {library.roots.length === 0 ? (
        <p className={styles.muted}>
          {library.kind === 'podcasts'
            ? t('admin.libraries.noRoots')
            : t('admin.libraries.noFolder')}
        </p>
      ) : (
        <ul className={styles.rootList}>
          {library.roots.map((root) => (
            <li key={root.id} className={styles.root}>
              <code className={styles.path}>{root.path}</code>
              <span className={styles.rootMeta}>
                {/* Audio libraries have one content, named by the kind already. */}
                {library.kind === 'video' && (
                  <Chip tone="outline">{t(`library.content.${root.content}`)}</Chip>
                )}
                <span className={styles.muted}>{providerChain(t, root.providers)}</span>
                <Chip tone="info">{t('admin.libraries.fsType', { type: root.fsType })}</Chip>
                {root.warning === 'network_share' && (
                  <Chip tone="warning" icon="warning">
                    {t('admin.libraries.warning.network_share')}
                  </Chip>
                )}
                <span className={styles.muted}>{t(watchKeys[root.watch])}</span>
                <span className={styles.muted}>
                  {t('admin.libraries.folders.content', {
                    items: root.itemCount,
                    files: root.fileCount,
                  })}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
