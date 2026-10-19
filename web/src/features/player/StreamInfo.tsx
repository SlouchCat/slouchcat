import type { PlaybackInfo } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { resolutionClass } from '@/lib/format';
import styles from './Player.module.css';

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
      [t('player.streamInfo.reason'), info.transcoding.reason],
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
      file.width && file.height
        ? `${t('format.resolution', { width: file.width, height: file.height })} · ${resolutionClass(t, file.width, file.height)}`
        : none,
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
      <dl className={styles.infoList}>
        {rows.map(([term, value]) => (
          <div key={term} className={styles.infoRow}>
            <dt>{term}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
