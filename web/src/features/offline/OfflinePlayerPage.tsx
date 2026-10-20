import type { SessionInfo } from '@slouchcat/api-client';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { sessionQuery } from '@/app/session';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { ulid } from '@/lib/ulid';
import { Notice } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { downloadTitle } from './labels';
import { useOfflineChoice, type OfflineChoice } from './offlineSession';
import { MEDIA_FILE, readFile } from './opfs';
import { OfflinePlayer } from './OfflinePlayer';
import { getDownload, getMeta, getProfile } from './store';
import type { StoredDownload, StoredWatchTime } from './types';
import styles from '@/features/player/Player.module.css';

const route = getRouteApi('/offline/play/$downloadId');

interface Loaded {
  download: StoredDownload & { manifest: NonNullable<StoredDownload['manifest']> };
  media: File;
  watch: StoredWatchTime | undefined;
  serverOffsetMs: number;
}

type LoadState = { status: 'loading' } | { status: 'missing' } | ({ status: 'ready' } & Loaded);

/**
 * Who plays: the profile chosen in the offline library, else the session's active profile (the
 * route's guard sends anyone without either to the library).
 */
function usePlayer(): OfflineChoice | null {
  const choice = useOfflineChoice();
  const queryClient = useQueryClient();
  const session = queryClient.getQueryData<SessionInfo | null>(sessionQuery.queryKey);
  if (choice) return choice;
  return session?.profile ? { accountId: session.account.id, profileId: session.profile.id } : null;
}

async function load(id: string, who: OfflineChoice): Promise<LoadState> {
  const download = await getDownload(id);
  // Only the server's decision counts: the profile must be in allowedProfileIds (O-7).
  if (
    !download ||
    download.pendingDelete ||
    !download.complete ||
    !download.manifest ||
    download.accountId !== who.accountId ||
    !download.allowedProfileIds.includes(who.profileId)
  ) {
    return { status: 'missing' };
  }
  const [media, profile, offset] = await Promise.all([
    readFile(id, MEDIA_FILE),
    getProfile(who.profileId),
    getMeta('serverOffsetMs'),
  ]);
  if (!media || media.size !== download.manifest.sizeBytes) return { status: 'missing' };
  return {
    status: 'ready',
    download: { ...download, manifest: download.manifest },
    media,
    watch: profile?.accountId === who.accountId ? profile.watch : undefined,
    serverOffsetMs: offset ?? 0,
  };
}

/** Route /offline/play/$downloadId: a download played from this device, online or offline. */
export function OfflinePlayerPage() {
  const { downloadId } = route.useParams();
  const who = usePlayer();
  const { t } = useTranslation();
  const choice = useOfflineChoice();
  const goBack = useGoBack(choice ? paths.offline : paths.downloads);
  // One playback per mount (OfflineEvent.playbackId).
  const [playbackId] = useState(() => ulid());
  const accountId = who?.accountId;
  const profileId = who?.profileId;
  const key = `${downloadId}:${accountId ?? ''}:${profileId ?? ''}`;
  const [loaded, setLoaded] = useState<{ key: string; state: LoadState } | null>(null);

  useEffect(() => {
    if (accountId === undefined || profileId === undefined) return;
    let active = true;
    load(downloadId, { accountId, profileId }).then(
      (next) => {
        if (active) setLoaded({ key, state: next });
      },
      () => {
        if (active) setLoaded({ key, state: { status: 'missing' } });
      },
    );
    return () => {
      active = false;
    };
  }, [accountId, downloadId, key, profileId]);

  const state: LoadState = !who
    ? { status: 'missing' }
    : loaded?.key === key
      ? loaded.state
      : { status: 'loading' };

  const title = state.status === 'ready' ? downloadTitle(t, state.download.item) : undefined;
  useDocumentTitle(title);

  if (state.status === 'ready' && accountId !== undefined && profileId !== undefined) {
    const { download } = state;
    const resume = download.positions?.[profileId]?.positionMs ?? 0;
    return (
      <OfflinePlayer
        title={title ?? ''}
        manifest={download.manifest}
        media={state.media}
        playback={{
          accountId,
          profileId,
          downloadId: download.id,
          itemId: download.manifest.itemId,
          playbackId,
        }}
        subtitleIds={download.subtitlesStored}
        resumeMs={resume}
        watch={state.watch}
        serverOffsetMs={state.serverOffsetMs}
        onBack={goBack}
      />
    );
  }
  return <Message loading={state.status === 'loading'} onBack={goBack} />;
}

function Message({ loading, onBack }: { loading: boolean; onBack: () => void }) {
  const { t } = useTranslation();
  useBackKeys(onBack);
  return (
    <main className={styles.messagePage}>
      <div className={styles.messageBox}>
        {loading ? (
          <p className={styles.loading}>{t('player.loading')}</p>
        ) : (
          <Notice tone="warning" title={t('offline.player.missing.title')}>
            {t('offline.player.missing.body')}
          </Notice>
        )}
        <div>
          <AriaButton variant="secondary" icon="back" onPress={onBack} autoFocus>
            {t('common.back')}
          </AriaButton>
        </div>
      </div>
    </main>
  );
}
