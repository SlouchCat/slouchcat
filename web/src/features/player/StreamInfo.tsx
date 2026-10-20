import type { PlaybackInfo } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { formatBitrate, formatResolution } from '@/lib/format';
import { FactList } from '@/ui';
import { selectedAudio, selectedSubtitle, trackLabel } from './labels';
import styles from './Player.module.css';

const reasons = [
  'container',
  'video_codec',
  'audio_codec',
  'hdr',
  'pixel_format',
  'keyframes_pending',
  'dolby_vision',
  'bandwidth',
  'max_height',
  'quality',
  'audio_track',
  'subtitle_burn',
  'budget_lowered',
] as const;
type Reason = (typeof reasons)[number];

/** The server's reason for not playing directly in words; an unknown code shows as it is. */
const reasonText = (t: TFunction, reason: string): string =>
  (reasons as readonly string[]).includes(reason)
    ? t(`player.streamInfo.reasons.${reason as Reason}`)
    : reason;

/**
 * "Stream-Info" overlay of the player (B-5, W-4, W-8): how the server delivers this playback, in
 * which quality and with which tracks, and which file it plays. Shows the server's answer only;
 * the client decides nothing here.
 */
export function StreamInfo({ info }: { info: PlaybackInfo }) {
  const { t, i18n } = useTranslation();
  const file = info.mediaFile;
  const none = t('player.streamInfo.none');
  const rows: [string, string][] = [
    [
      t('player.streamInfo.mode'),
      info.mode === 'direct' ? t('player.streamInfo.modeDirect') : t('player.streamInfo.modeHls'),
    ],
  ];
  const transcoding = info.transcoding;
  if (transcoding) {
    rows.push(
      [t('player.streamInfo.transcoding'), t(`player.streamInfo.${transcoding.mode}`)],
      [t('player.streamInfo.reason'), reasonText(t, transcoding.reason)],
    );
    if (transcoding.mode === 'transcode') {
      rows.push(
        [
          t('player.streamInfo.encoder'),
          transcoding.hardware
            ? t('player.streamInfo.hardware', { backend: transcoding.backend ?? none })
            : t('player.streamInfo.software'),
        ],
        [
          t('player.streamInfo.tonemap'),
          transcoding.tonemap ? t('player.streamInfo.yes') : t('player.streamInfo.no'),
        ],
      );
    }
    rows.push([t('player.streamInfo.quality'), transcoding.quality ?? t('player.quality.auto')]);
    if (transcoding.width && transcoding.height) {
      rows.push([
        t('player.streamInfo.output'),
        formatResolution(t, transcoding.width, transcoding.height),
      ]);
    }
    if (transcoding.bitrate) {
      rows.push([
        t('player.streamInfo.bitrate'),
        formatBitrate(transcoding.bitrate, i18n.language),
      ]);
    }
  } else {
    rows.push([t('player.streamInfo.quality'), t('player.streamInfo.original')]);
  }

  const audioIndex = info.tracks.audio.findIndex((track) => track === selectedAudio(info));
  const audio = info.tracks.audio[audioIndex];
  const subtitle = selectedSubtitle(info);
  rows.push(
    [
      t('player.streamInfo.audioTrack'),
      audio ? trackLabel(t, audio, audioIndex, i18n.language) : none,
    ],
    [
      t('player.streamInfo.subtitle'),
      subtitle
        ? trackLabel(t, subtitle, info.tracks.subtitles.indexOf(subtitle), i18n.language)
        : t('player.tracks.off'),
    ],
  );
  if (subtitle && info.selection.subtitleDelivery) {
    rows.push([
      t('player.streamInfo.delivery'),
      info.selection.subtitleDelivery === 'burn'
        ? t('player.streamInfo.deliveryBurn')
        : t('player.streamInfo.deliveryText'),
    ]);
  }

  rows.push(
    [t('player.streamInfo.mediaFile'), file.id],
    [t('player.streamInfo.version'), file.versionLabel ?? none],
    [t('player.streamInfo.container'), file.container.toUpperCase()],
    [t('player.streamInfo.video'), file.videoCodec?.toUpperCase() ?? none],
    [t('player.streamInfo.audio'), file.audioCodec?.toUpperCase() ?? none],
    [
      t('player.streamInfo.resolution'),
      file.width && file.height ? formatResolution(t, file.width, file.height) : none,
    ],
  );

  return (
    <section
      className={styles.info}
      aria-labelledby="stream-info-title"
      data-media-file-id={file.id}
    >
      <h2 id="stream-info-title" className={styles.infoTitle}>
        {t('player.streamInfo.title')}
      </h2>
      <FactList facts={rows} layout="rows" />
    </section>
  );
}
