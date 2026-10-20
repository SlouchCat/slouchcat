import type { PlaybackInfo } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { formatResolution } from '@/lib/format';
import { FactList } from '@/ui';
import styles from './Player.module.css';

const reasons = [
  'container',
  'video_codec',
  'audio_codec',
  'hdr',
  'pixel_format',
  'keyframes_pending',
] as const;
type Reason = (typeof reasons)[number];

/** The server's reason for not playing directly in words; an unknown code shows as it is. */
const reasonText = (t: TFunction, reason: string): string =>
  (reasons as readonly string[]).includes(reason)
    ? t(`player.streamInfo.reasons.${reason as Reason}`)
    : reason;

/**
 * "Stream-Info" overlay of the player (B-5): how the server delivers this playback and which file
 * it plays. Shows the server's answer only; the client decides nothing here.
 */
export function StreamInfo({ info }: { info: PlaybackInfo }) {
  const { t } = useTranslation();
  const file = info.mediaFile;
  const none = t('player.streamInfo.none');
  const rows: [string, string][] = [
    [
      t('player.streamInfo.mode'),
      info.mode === 'direct' ? t('player.streamInfo.modeDirect') : t('player.streamInfo.modeHls'),
    ],
  ];
  if (info.transcoding) {
    rows.push(
      [t('player.streamInfo.transcoding'), t(`player.streamInfo.${info.transcoding.mode}`)],
      [t('player.streamInfo.backend'), info.transcoding.backend ?? none],
      [t('player.streamInfo.reason'), reasonText(t, info.transcoding.reason)],
    );
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
