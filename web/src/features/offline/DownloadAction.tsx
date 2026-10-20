import type { ItemRef } from '@slouchcat/api-client';
import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useOnline } from '@/lib/useOnline';
import { Chip, setSpatialFocus, SpatialButton, SpatialButtonLink, VisuallyHidden } from '@/ui';
import { downloadStatus } from './labels';
import { isDownloadable } from './support';
import type { StoredDownload } from './types';
import { useStoredDownloads } from './useOfflineData';
import { usePreparingPoll } from './usePreparingPoll';
import styles from './DownloadAction.module.css';

// The dialog needs react-aria, which the detail page does not load up front (P-10).
const DownloadDialog = lazy(() =>
  import('./DownloadDialog').then((module) => ({ default: module.DownloadDialog })),
);

const itemIdOf = (download: StoredDownload) => download.item?.id ?? download.manifest?.itemId;

interface DownloadActionProps {
  /** A movie, an episode or a season. */
  target: Pick<ItemRef, 'id' | 'kind' | 'title' | 'childCount'>;
  /** Version of a single title (B-5). */
  mediaFileId?: string;
  /** The title has a playable file (single titles). */
  available?: boolean;
  focusKey?: string;
  size?: 'md' | 'lg';
}

/**
 * "Herunterladen" on the detail page (O-1, O-2) and the state of the title on this device: a
 * download of the active profile shows its state and leads to the downloads page; a season counts
 * its episodes here. The lookup goes by the server's `allowedProfileIds` only (O-7).
 */
export function DownloadAction({
  target,
  mediaFileId,
  available = true,
  focusKey,
  size = 'lg',
}: DownloadActionProps) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const online = useOnline();
  const downloads = useStoredDownloads();
  const [open, setOpen] = useState(false);
  const [announcement, setAnnouncement] = useState('');
  const mine = (downloads ?? []).filter(
    (download) =>
      download.accountId === session.account.id &&
      !download.pendingDelete &&
      download.allowedProfileIds.includes(session.profile.id),
  );
  const season = target.kind === 'season';
  const existing = season ? undefined : mine.find((download) => itemIdOf(download) === target.id);
  const episodes = season ? mine.filter((download) => download.item?.parentId === target.id) : [];
  const done = episodes.filter((download) => download.complete).length;
  usePreparingPoll(existing ? [existing] : episodes);
  // The new download replaces the button by the link to the downloads: the focus follows it.
  const refocus = useRef(false);
  const hasExisting = existing !== undefined;
  useEffect(() => {
    if (!refocus.current || !hasExisting || !focusKey) return;
    refocus.current = false;
    setSpatialFocus(focusKey);
  }, [focusKey, hasExisting]);
  if (!isDownloadable(target.kind) || !downloads) return null;

  let state: string | null = null;
  let tone: ReturnType<typeof downloadStatus>['tone'] = 'info';
  if (existing) {
    const status = downloadStatus(t, existing);
    state = status.text;
    tone = status.tone;
  } else if (episodes.length > 0) {
    state = t('offline.status.season', {
      done,
      total: Math.max(target.childCount ?? 0, episodes.length),
    });
    tone = done === episodes.length ? 'success' : 'info';
  }

  return (
    <>
      {existing ? (
        <SpatialButtonLink
          focusKey={focusKey}
          href={paths.downloads}
          variant="secondary"
          size={size}
          icon="download"
        >
          {t('offline.status.open')}
        </SpatialButtonLink>
      ) : (
        <SpatialButton
          focusKey={focusKey}
          variant="secondary"
          size={size}
          icon="download"
          isDisabled={!online || !available}
          onPress={() => setOpen(true)}
        >
          {season ? t('offline.dialog.seasonButton') : t('offline.dialog.button')}
        </SpatialButton>
      )}
      {state && (
        <span className={styles.state} data-download-state={existing?.state ?? 'season'}>
          <Chip tone={tone} icon="download">
            {state}
          </Chip>
        </span>
      )}
      <span aria-live="polite">
        <VisuallyHidden>{announcement}</VisuallyHidden>
      </span>
      {open && (
        <Suspense fallback={null}>
          <DownloadDialog
            target={target}
            mediaFileId={mediaFileId}
            onClose={() => setOpen(false)}
            onCreated={(created) => {
              refocus.current = true;
              setOpen(false);
              setAnnouncement(t('offline.dialog.started', { count: created.length }));
            }}
          />
        </Suspense>
      )}
    </>
  );
}
