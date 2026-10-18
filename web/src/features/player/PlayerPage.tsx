import type { PlaybackInfo } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Button, Notice } from '@/ui';
import { VideoPlayer } from './VideoPlayer';
import styles from './Player.module.css';

const route = getRouteApi('/_authed/play/$itemId');

const containerMime: Record<string, string> = {
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  mov: 'video/quicktime',
  webm: 'video/webm',
  mkv: 'video/x-matroska',
};

/** MIME type for the capability check: the server's (with codecs) or one derived from the container. */
export function playbackMime(info: PlaybackInfo): string {
  return (
    info.mediaFile.mimeType ??
    containerMime[info.mediaFile.container.toLowerCase()] ??
    `video/${info.mediaFile.container.toLowerCase()}`
  );
}

/**
 * Whether this browser can direct-play the file. This is a device capability check (W-1), not a
 * visibility rule: the server already decided that the profile may watch the title.
 */
function canPlay(mime: string): boolean {
  return document.createElement('video').canPlayType(mime) !== '';
}

/** Video player route (W-1 direct play, W-5 progress and resume). */
export function PlayerPage() {
  const { t } = useTranslation();
  const { itemId } = route.useParams();
  const queryClient = useQueryClient();
  // Start of the measurement "navigation → first frame playing" (data-start-ms).
  const [openedAt] = useState(() => performance.now());
  const goBack = useGoBack(paths.item(itemId));

  const playback = useQuery({
    queryKey: ['playback', itemId],
    queryFn: ({ signal }): Promise<PlaybackInfo> =>
      request(api.POST('/items/{id}/play', { params: { path: { id: itemId } }, body: {}, signal })),
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  });
  useDocumentTitle(playback.data?.item.title);

  const supported = useMemo(
    () => (playback.data ? canPlay(playbackMime(playback.data)) : true),
    [playback.data],
  );

  // Progress changed while watching: the start page and the detail page must refetch.
  useEffect(
    () => () => {
      void queryClient.invalidateQueries({ queryKey: ['home'] });
      void queryClient.invalidateQueries({ queryKey: ['item', itemId] });
    },
    [itemId, queryClient],
  );

  if (playback.data && supported) {
    return <VideoPlayer info={playback.data} openedAt={openedAt} onBack={goBack} />;
  }

  return (
    <PlayerMessage onBack={goBack}>
      {playback.isPending && <p className={styles.loading}>{t('player.loading')}</p>}
      {playback.isError && (
        <Notice tone="danger" title={t('player.error.title')}>
          {problemMessage(t, playback.error)}
        </Notice>
      )}
      {playback.data && !supported && (
        <Notice tone="warning" title={t('player.unsupported.title')}>
          <p>{t('player.unsupported.body')}</p>
          <p className={styles.detailsLine}>
            {t('player.unsupported.details', {
              container: playback.data.mediaFile.container.toUpperCase(),
              video: playback.data.mediaFile.videoCodec?.toUpperCase() ?? '–',
              audio: playback.data.mediaFile.audioCodec?.toUpperCase() ?? '–',
            })}
          </p>
        </Notice>
      )}
    </PlayerMessage>
  );
}

function PlayerMessage({ children, onBack }: { children: ReactNode; onBack: () => void }) {
  const { t } = useTranslation();
  useBackKeys(onBack);
  return (
    <main className={styles.messagePage}>
      <div className={styles.messageBox}>
        {children}
        <div>
          <Button variant="secondary" icon="back" onPress={onBack} autoFocus>
            {t('common.back')}
          </Button>
        </div>
      </div>
    </main>
  );
}
