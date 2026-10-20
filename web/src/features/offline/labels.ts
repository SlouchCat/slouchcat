import type { ItemRef } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { episodeCode } from '@/features/catalog/labels';
import type { DownloadQuality, StoredDownload } from './types';

export type StatusTone = 'success' | 'info' | 'warning' | 'danger' | 'neutral';

export interface DownloadStatus {
  text: string;
  tone: StatusTone;
  /** 0 … 100 while the server prepares or the device fetches. */
  percent?: number;
}

const percentOf = (done: number, total: number | undefined) =>
  total && total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;

/** Bytes of the finished file (manifest, server size) or the server's estimate. */
export const expectedBytes = (download: StoredDownload) =>
  download.manifest?.sizeBytes ?? download.sizeBytes ?? download.estimatedBytes;

/**
 * What a download is doing, from the server's state and the device's fetch: preparing on the
 * server, waiting, coming in, on the device, or why not.
 */
export function downloadStatus(
  t: TFunction,
  download: StoredDownload,
  waitsForWifi = false,
): DownloadStatus {
  if (download.pendingDelete) return { text: t('offline.state.deleting'), tone: 'neutral' };
  if (download.complete) return { text: t('offline.state.complete'), tone: 'success' };
  switch (download.fetchError) {
    case 'quota':
      return { text: t('offline.state.quota'), tone: 'danger' };
    case 'storage':
      return { text: t('offline.state.storage'), tone: 'danger' };
    case 'network':
      return { text: t('offline.state.network'), tone: 'warning' };
    case 'range':
      return { text: t('offline.state.range'), tone: 'warning' };
    case 'gone':
    case 'unauthorized':
      return { text: t('offline.state.gone'), tone: 'warning' };
    case 'evicted':
    case 'size':
    case 'changed':
      return { text: t('offline.state.evicted'), tone: 'info' };
    default:
  }
  switch (download.state) {
    case 'ready': {
      if (waitsForWifi) return { text: t('offline.state.wifi'), tone: 'info' };
      const percent = percentOf(download.bytesWritten, expectedBytes(download));
      return { text: t('offline.state.fetching', { percent }), tone: 'info', percent };
    }
    case 'preparing': {
      const percent = percentOf(download.segmentsDone, download.segmentsTotal);
      return { text: t('offline.state.preparing', { percent }), tone: 'info', percent };
    }
    case 'queued':
      return download.error === 'budget_full'
        ? { text: t('offline.state.budgetFull'), tone: 'info' }
        : { text: t('offline.state.queued'), tone: 'info' };
    case 'paused':
      return { text: t('offline.state.paused'), tone: 'info' };
    case 'failed':
      return download.error === 'not_visible'
        ? { text: t('offline.state.gone'), tone: 'warning' }
        : { text: t('offline.state.failed'), tone: 'danger' };
    case 'evicted':
      return { text: t('offline.state.evicted'), tone: 'info' };
  }
}

export const qualityLabel = (t: TFunction, quality: DownloadQuality) =>
  t(`offline.quality.${quality}`);

/** "S01E02 · Episode · Series" for an episode, the title otherwise; a fallback without data. */
export function downloadTitle(t: TFunction, item: ItemRef | undefined): string {
  if (!item) return t('offline.unknownTitle');
  if (item.kind !== 'episode') return item.title;
  const code = episodeCode(t, item);
  const series = item.seriesTitle;
  if (code && series) return t('offline.episodeTitle', { code, title: item.title, series });
  return code ? t('item.nextEpisodeLabel', { code, title: item.title }) : item.title;
}
