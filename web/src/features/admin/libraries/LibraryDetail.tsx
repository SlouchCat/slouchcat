import type { Library, LibraryKeyframeIndex, LibraryTrickplay } from '@slouchcat/api-client';
import { memo, useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { Chip, Notice, ProgressBar, TextLink } from '@/ui';
import { ActionMenu, AriaButton, providerChain, type ActionMenuItem } from '@/ui/aria';
import { useFolderMetadataText } from '../metadata/languages';
import { canScan, isScanning } from '../queries';
import { canAcceptLoss, errorText, hasScanError } from './libraryColumns';
import { RootFacts } from './RootFacts';
import styles from './Libraries.module.css';
import { intlLocale } from '@/i18n';

/**
 * How long after «Erneut scannen» the same button ignores a press: it turns into «Scan abbrechen»
 * under the pointer, and a double press must not cancel the scan it just started.
 */
const CANCEL_GRACE_MS = 1000;

/** The job type whose row on the jobs page the coverage links (`?item=` opens it). */
const BACKFILL_TYPE = 'keyframes.backfill';

/** The handlers of a fold-out, stable across renders so the fold-outs need not render again. */
export interface LibraryHandlers {
  onScan: (id: number) => void;
  onCancelScan: (id: number) => void;
  /** «Bearbeiten»: the library form (concept 24 4.4). */
  onEdit: (id: number) => void;
  onAcceptLoss: (library: Library) => void;
  /** «Bibliothek löschen …» (concept 20 P-8): the typed confirmation of the page. */
  onDelete: (library: Library) => void;
}

/**
 * The fold-out of a library (Bib-C, concept 24 4.4): «Erneut scannen» or, while a scan runs,
 * «Scan abbrechen», and «Bearbeiten» as buttons; «Verlust akzeptieren» and «Bibliothek löschen …»
 * (danger) in «Weitere Aktionen» (S-6; concept 20 P-8); of an audiobook library the link to its
 * sources; the progress of a running scan, the error band of the last one, the keyframe index,
 * the trickplay setting, and every folder with its whole path, read-only (BIB-1 to BIB-4).
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
  // When this button started a scan; a cancel within the grace period is the same double press.
  const startedAt = useRef(0);
  const onScanButton = () => {
    if (!scanning) {
      startedAt.current = Date.now();
      handlers.onScan(library.id);
      return;
    }
    if (Date.now() - startedAt.current < CANCEL_GRACE_MS) return;
    handlers.onCancelScan(library.id);
  };
  const showProgress = scan.state === 'running' && (scan.filesTotal ?? 0) > 0;
  const progress = showProgress
    ? Math.min(100, ((scan.filesSeen ?? 0) / (scan.filesTotal ?? 1)) * 100)
    : 0;
  const progressText = t('admin.libraries.progress', {
    seen: scan.filesSeen ?? 0,
    total: scan.filesTotal ?? 0,
  });

  const menu: ActionMenuItem[] = [
    ...(canAcceptLoss(library)
      ? [{ id: 'acceptLoss', label: t('admin.libraries.acceptLoss.action'), danger: true }]
      : []),
    { id: 'delete', label: t('admin.libraries.delete.action'), danger: true },
  ];
  const onMenu = (id: string) => {
    if (id === 'acceptLoss') handlers.onAcceptLoss(library);
    else if (id === 'delete') handlers.onDelete(library);
  };

  return (
    <section className={styles.detail} aria-label={library.name}>
      <div className={styles.actions}>
        {/* One button that changes its part: the focus stays on it when a scan starts or ends. */}
        <AriaButton
          variant="secondary"
          icon={scanning ? 'close' : 'refresh'}
          isDisabled={!scanning && !canScan(library)}
          isPending={scanning ? cancelPending : scanPending}
          onPress={onScanButton}
        >
          {scanning ? t('admin.libraries.cancel') : t('admin.libraries.rescan')}
        </AriaButton>
        <AriaButton
          icon="pen"
          aria-label={t('admin.libraries.editLabel', { name: library.name })}
          onPress={() => handlers.onEdit(library.id)}
        >
          {t('admin.libraries.edit')}
        </AriaButton>
        <ActionMenu rowName={library.name} items={menu} onAction={onMenu} />
        {library.kind === 'audiobooks' && (
          <TextLink href={paths.adminAudiobookSources}>
            {t('admin.libraries.metadata.audiobooksLink')}
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

      {library.keyframeIndex && <KeyframeCoverage coverage={library.keyframeIndex} />}
      {library.trickplay && <TrickplaySummary setting={library.trickplay} />}

      <Folders library={library} />
    </section>
  );
});

/**
 * The keyframe index of a video library (concept 14, 7.2): how many of its files can be remuxed
 * right away, and while some still wait, the way to the backfill on the jobs page (its button
 * stays there, F-4). Audio libraries have no coverage; one without video files says nothing.
 */
function KeyframeCoverage({ coverage }: { coverage: LibraryKeyframeIndex }) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(intlLocale(i18n.language)), [i18n.language]);
  if (coverage.videoFiles === 0) return null;
  const complete = coverage.indexed === coverage.videoFiles;
  return (
    <p className={styles.keyframes}>
      <span className={styles.muted}>
        {complete
          ? t('admin.libraries.keyframes.complete')
          : t('admin.libraries.keyframes.summary', {
              indexed: number.format(coverage.indexed),
              videoFiles: number.format(coverage.videoFiles),
              pending: number.format(coverage.pending),
              unavailable: number.format(coverage.unavailable),
            })}
      </span>
      {coverage.pending > 0 && (
        <TextLink href={`${paths.adminJobs}?item=${BACKFILL_TYPE}`}>
          {t('admin.libraries.keyframes.backfill')}
        </TextLink>
      )}
    </p>
  );
}

/** The setting «Vorschau beim Spulen» of a video library in one line, films and series. */
function TrickplaySummary({ setting }: { setting: LibraryTrickplay }) {
  const { t } = useTranslation();
  const label = (value: { mode: LibraryTrickplay['series']['mode']; keep: boolean }) =>
    value.mode === 'on_start' && !value.keep
      ? t('admin.libraries.trickplay.summaryNoKeep')
      : t(`admin.libraries.trickplay.mode.${value.mode}`);
  return (
    <p className={styles.keyframes}>
      <span className={styles.muted}>
        {t('admin.libraries.trickplay.summary', {
          movies: label(setting.movies),
          series: label(setting.series),
        })}
      </span>
    </p>
  );
}

/**
 * Every folder of the library with its whole path, content, provider list, the languages and
 * images of a video folder (the default or its own, concept 24 4.4) and the technical facts
 * (RootFacts, BIB-2), read-only: adding, changing and removing are «Bearbeiten». A library without
 * a folder says why: podcasts take their titles from feeds, a video or audio library that lost
 * its last folder scans nothing until one is added under «Bearbeiten» (idea 02, E13).
 */
function Folders({ library }: { library: Library }) {
  const { t } = useTranslation();
  const metadata = useFolderMetadataText();
  return (
    <div className={styles.roots}>
      <h3 className={styles.rootsTitle}>{t('admin.libraries.roots')}</h3>
      {library.roots.length === 0 ? (
        <p className={styles.muted}>
          {library.kind === 'podcasts'
            ? t('admin.libraries.noRoots')
            : t('admin.libraries.noFolderEdit')}
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
                {library.kind === 'video' && (
                  <>
                    <span className={styles.muted}>{metadata.langs(root)}</span>
                    <span className={styles.muted}>{metadata.images(root)}</span>
                  </>
                )}
                <RootFacts root={root} />
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
