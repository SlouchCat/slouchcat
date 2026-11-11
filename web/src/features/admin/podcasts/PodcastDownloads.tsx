import type { PodcastDownload, PodcastDownloadPage } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { formatBytes } from '@/lib/format';
import { Notice, ProgressBar, Text } from '@/ui';
import { AriaButton, BandPanel, DataTable, StatusRegion, useStatusMessage } from '@/ui/aria';
import { PODCAST_DOWNLOAD_JOB, pauseJobType, resumeJobType } from '../queries';
import styles from './Podcasts.module.css';

/** The share of a running download in per cent, when the provider named its size. */
export function downloadShare(progress?: number, total?: number): number | undefined {
  if (progress === undefined || total === undefined || total <= 0) return undefined;
  return Math.min(100, Math.round((100 * progress) / total));
}

/**
 * The section «Downloads» of the podcasts page (concept 19 part A, 12.3): the queue of every
 * podcast library as the server sends it (running first, then in the order a worker takes them),
 * «Anhalten» / «Fortsetzen» for the job type and «Abbrechen» per episode. The page polls the
 * queue while it holds something (`queue` comes from there); a cancelled row is gone at the next
 * answer, so the focus goes to the title of the section.
 */
export function PodcastDownloads({ queue }: { queue: PodcastDownloadPage }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = new Intl.NumberFormat(i18n.language);
  const [message, say] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  const pauseButton = useRef<HTMLButtonElement>(null);

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'podcasts'] });
  const pause = useMutation({
    mutationFn: (paused: boolean) =>
      paused ? resumeJobType(PODCAST_DOWNLOAD_JOB) : pauseJobType(PODCAST_DOWNLOAD_JOB),
    onSuccess: async (_info, paused) => {
      setFailure(null);
      say(paused ? t('admin.podcasts.queue.resumed') : t('admin.podcasts.queue.paused'));
      await refresh();
    },
    onError: (error) => {
      say(null);
      setFailure(error);
    },
  });
  const cancel = useMutation({
    mutationFn: (d: PodcastDownload) =>
      request(
        api.DELETE('/admin/podcasts/{id}/episodes/{episodeId}/download', {
          params: { path: { id: d.podcastId, episodeId: d.episodeId } },
        }),
      ),
    onSuccess: async (_data, d) => {
      setFailure(null);
      say(t('admin.podcasts.queue.cancelled', { title: d.episodeTitle }));
      await refresh();
      // The row and its button are gone; «Anhalten» stays and takes the focus.
      pauseButton.current?.focus();
    },
    onError: (error) => {
      say(null);
      setFailure(error);
    },
  });

  const progressOf = (d: PodcastDownload) => {
    const share = downloadShare(d.progressBytes, d.totalBytes);
    if (d.state === 'running' && queue.paused) return t('admin.podcasts.queue.state.paused');
    if (d.state === 'queued') {
      return queue.paused
        ? t('admin.podcasts.queue.state.paused')
        : t('admin.podcasts.queue.state.queued');
    }
    if (share !== undefined) return t('admin.podcasts.queue.state.runningShare', { share });
    return d.progressBytes !== undefined
      ? t('admin.podcasts.queue.state.runningBytes', {
          size: formatBytes(d.progressBytes, i18n.language),
        })
      : t('admin.podcasts.queue.state.running');
  };

  const summary =
    queue.total === 0
      ? t('admin.podcasts.queue.empty')
      : t('admin.podcasts.queue.summary', {
          count: queue.total,
          number: number.format(queue.total),
          running: number.format(queue.running),
        });
  return (
    <BandPanel
      title={t('admin.podcasts.queue.title')}
      icon="download"
      meta={<span className={styles.summary}>{summary}</span>}
    >
      <div className={styles.band}>
        <div className={styles.actions}>
          <AriaButton
            ref={pauseButton}
            variant="secondary"
            icon={queue.paused ? 'play' : 'pause'}
            isPending={pause.isPending}
            onPress={() => pause.mutate(queue.paused)}
          >
            {queue.paused ? t('admin.podcasts.queue.resume') : t('admin.podcasts.queue.pause')}
          </AriaButton>
          {queue.paused && <Text tone="secondary">{t('admin.podcasts.queue.pausedHint')}</Text>}
        </div>
        {queue.downloads.length > 0 ? (
          <DataTable
            caption={t('admin.podcasts.queue.title')}
            columns={[
              {
                key: 'episode',
                header: t('admin.podcasts.queue.columns.episode'),
                rowHeader: true,
                cell: (d) => (
                  <span className={styles.stack}>
                    <span>{d.episodeTitle}</span>
                    <span className={`${styles.muted} ${styles.small}`}>
                      {d.sizeBytes !== undefined
                        ? t('admin.podcasts.queue.ofPodcastSize', {
                            podcast: d.podcastTitle,
                            size: formatBytes(d.sizeBytes, i18n.language),
                          })
                        : d.podcastTitle}
                    </span>
                  </span>
                ),
              },
              {
                key: 'progress',
                header: t('admin.podcasts.queue.columns.progress'),
                cell: (d) => {
                  const share = downloadShare(d.progressBytes, d.totalBytes);
                  const text = progressOf(d);
                  return (
                    <span className={`${styles.stack} ${styles.episodes}`}>
                      {d.state === 'running' && <ProgressBar value={share ?? 0} label={text} />}
                      <span className={`${styles.muted} ${styles.small}`}>{text}</span>
                    </span>
                  );
                },
              },
              {
                key: 'cancel',
                header: t('admin.podcasts.queue.columns.cancel'),
                hideHeader: true,
                align: 'end',
                cell: (d) => (
                  <AriaButton
                    variant="ghost"
                    icon="close"
                    aria-label={t('admin.podcasts.queue.cancel', { title: d.episodeTitle })}
                    isPending={cancel.isPending && cancel.variables.episodeId === d.episodeId}
                    onPress={() => cancel.mutate(d)}
                  >
                    {t('common.cancel')}
                  </AriaButton>
                ),
              },
            ]}
            rows={queue.downloads}
            rowKey={(d) => d.episodeId}
          />
        ) : (
          <Text tone="secondary">{t('admin.podcasts.queue.none')}</Text>
        )}
        {queue.total > queue.downloads.length && (
          <Text tone="secondary">
            {t('admin.podcasts.queue.more', {
              shown: number.format(queue.downloads.length),
              total: number.format(queue.total),
            })}
          </Text>
        )}
        {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
        <StatusRegion message={failure === null ? message : null} />
      </div>
    </BandPanel>
  );
}
