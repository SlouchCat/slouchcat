import { useTranslation } from 'react-i18next';
import row from './DashRow.module.css';
import { PosterFallback } from './PosterFallback';
import { StreamProgress } from './StreamProgress';
import styles from './StreamRow.module.css';

interface StreamRowProps {
  title: string;
  /** Poster of the title, or undefined for the fallback tile. */
  posterUrl?: string;
  profile: string;
  device: string;
  /** How it plays, formatted by the page ("Transcodiert (NVENC) · 1080p", "Direkt"). */
  mode: string;
  /** 0 … 100. */
  progress: number;
  /** Accessible name of the progress bar ("Fortschritt Oppenheimer"). */
  progressLabel: string;
  /** Position and duration as shown next to the bar ("0:42:10 / 3:00:00"). */
  progressText?: string;
}

/** A running stream on the overview (concept 11, card Jetzt läuft). */
export function StreamRow({
  title,
  posterUrl,
  profile,
  device,
  mode,
  progress,
  progressLabel,
  progressText,
}: StreamRowProps) {
  const { t } = useTranslation();
  return (
    <div className={row.row}>
      <span className={styles.poster}>
        {posterUrl ? (
          <img
            className={styles.image}
            src={posterUrl}
            alt=""
            width={200}
            height={300}
            loading="lazy"
            decoding="async"
            draggable={false}
          />
        ) : (
          <PosterFallback />
        )}
      </span>
      <span className={row.text}>
        <span className={row.title}>{title}</span>
        <span className={row.line}>{[profile, device].join(t('format.separator'))}</span>
        <span className={row.line}>{mode}</span>
        <StreamProgress value={progress} label={progressLabel} text={progressText} />
      </span>
    </div>
  );
}
