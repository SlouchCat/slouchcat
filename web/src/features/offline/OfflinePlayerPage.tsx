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
import { resumePosition, resumeRate } from './playback';
import { getDownload, getMeta, getProfile } from './store';
import type { StoredDownload, StoredWatchTime } from './types';
import { usePosterUrl } from './usePosterUrl';
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
 * Who plays: from /downloads (`as=session`) the session's active profile, never a profile chosen
 * in the offline library; from the library the profile chosen there (W1). The route's guard sends
 * anyone without one to the library.
 */
function usePlayer(fromSession: boolean): OfflineChoice | null {
  const choice = useOfflineChoice();
  const queryClient = useQueryClient();
  const session = queryClient.getQueryData<SessionInfo | null>(sessionQuery.queryKey);
  if (!fromSession) return choice;
  return session?.profile ? { accountId: session.account.id, profileId: session.profile.id } : null;
}

async function load(id: string, who: OfflineChoice, fromSession: boolean): Promise<LoadState> {
  if (!fromSession) {
    // The library's choice counts only for the device's account while it is signed in (W2).
    const [accountId, signedOut] = await Promise.all([getMeta('accountId'), getMeta('signedOut')]);
    if (accountId !== who.accountId || signedOut === true) return { status: 'missing' };
  }
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
  const { as } = route.useSearch();
  // Another download in the same route (history back and forth) is another playback (K10).
  return (
    <OfflinePlayback key={downloadId} downloadId={downloadId} fromSession={as === 'session'} />
  );
}

function OfflinePlayback({
  downloadId,
  fromSession,
}: {
  downloadId: string;
  fromSession: boolean;
}) {
  const who = usePlayer(fromSession);
  const { t } = useTranslation();
  const goBack = useGoBack(fromSession ? paths.downloads : paths.offline);
  // One playback per mount (OfflineEvent.playbackId).
  const [playbackId] = useState(() => ulid());
  const accountId = who?.accountId;
  const profileId = who?.profileId;
  const key = `${downloadId}:${accountId ?? ''}:${profileId ?? ''}`;
  const [loaded, setLoaded] = useState<{ key: string; state: LoadState } | null>(null);

  useEffect(() => {
    if (accountId === undefined || profileId === undefined) return;
    let active = true;
    load(downloadId, { accountId, profileId }, fromSession).then(
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
  }, [accountId, downloadId, fromSession, key, profileId]);

  const state: LoadState = !who
    ? { status: 'missing' }
    : loaded?.key === key
      ? loaded.state
      : { status: 'loading' };

  const title = state.status === 'ready' ? downloadTitle(t, state.download.item) : undefined;
  useDocumentTitle(title);
  // The cover of an audio download (M6) from the device, never from the network.
  const cover = usePosterUrl(state.status === 'ready' ? state.download : undefined);

  if (state.status === 'ready' && accountId !== undefined && profileId !== undefined) {
    const { download } = state;
    const resume = resumePosition(download, profileId);
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
        coverUrl={cover}
        byline={download.item?.author}
        accent={download.item?.accent}
        initialRate={resumeRate(download, profileId)}
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
