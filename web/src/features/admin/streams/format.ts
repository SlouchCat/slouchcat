import type { AdminStream } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { formatBitrate, formatPlayerTime } from '@/lib/format';

// What the streams page says about a running stream; the overview's card «Jetzt läuft» uses only
// streamMode from here.

/** What a stream costs the server: the four counters of «Serverlast» (concept 12, 11.3). */
export type StreamLoad = 'hw' | 'sw' | 'remux' | 'direct';

export const streamLoads: readonly StreamLoad[] = ['hw', 'sw', 'remux', 'direct'];

export function streamLoad(stream: AdminStream): StreamLoad {
  if (stream.mode !== 'transcode') return stream.mode;
  return stream.hardware ? 'hw' : 'sw';
}

/** The tone of a load: Direct costs nothing, Remux little, Transcode HW more, SW the most. */
export const loadTone = {
  hw: 'warning',
  sw: 'danger',
  remux: 'neutral',
  direct: 'success',
} as const satisfies Record<StreamLoad, 'neutral' | 'success' | 'warning' | 'danger'>;

/** "Direct", "Remux" or "Transcode (HW)" / "Transcode (SW)" (W-6). */
export function streamMode(t: TFunction, stream: AdminStream): string {
  if (stream.mode !== 'transcode') return t(`admin.streams.modes.${stream.mode}`);
  return t('admin.streams.modes.transcode', {
    encoder: stream.hardware ? t('admin.streams.hw') : t('admin.streams.sw'),
  });
}

/** The output of the stream: rung or height plus bit rate; the source as it is for direct play. */
export function streamQuality(t: TFunction, stream: AdminStream, language: string): string {
  const rung =
    stream.quality && stream.quality !== 'auto'
      ? stream.quality
      : stream.height
        ? t('format.height', { height: stream.height })
        : undefined;
  if (!rung) return t('admin.streams.original');
  return stream.bitrate
    ? t('admin.streams.qualityValue', {
        quality: rung,
        bitrate: formatBitrate(stream.bitrate, language),
      })
    : rung;
}

/**
 * Where the stream stands: "1:05 von 10:00" with the share for the bar; the position alone when
 * the length is unknown, and "–" before the first report.
 */
export function streamPosition(
  t: TFunction,
  { positionMs, durationMs }: AdminStream,
): { percent?: number; text: string } {
  if (positionMs === undefined) return { text: t('admin.streams.noPosition') };
  if (!durationMs) return { text: formatPlayerTime(positionMs, positionMs) };
  return {
    percent: Math.min(100, Math.max(0, Math.round((positionMs * 100) / durationMs))),
    text: t('admin.streams.position', {
      position: formatPlayerTime(positionMs, durationMs),
      duration: formatPlayerTime(durationMs, durationMs),
    }),
  };
}

/** The bandwidth of all streams: the sum of the bit rates the server reports (bit/s). */
export const totalBitrate = (streams: readonly AdminStream[]) =>
  streams.reduce((sum, stream) => sum + (stream.bitrate ?? 0), 0);
