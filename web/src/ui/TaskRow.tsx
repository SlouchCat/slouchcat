import { useTranslation } from 'react-i18next';
import styles from './DashRow.module.css';
import { ProgressBar } from './ProgressBar';

interface TaskRowProps {
  /** The job type's name ("Keyframe-Index"). */
  type: string;
  /** What it works on ("Oppenheimer (2023)", "Filme"). */
  subject?: string;
  /** 0 … 100. */
  progress: number;
  /** Accessible name of the progress bar ("Fortschritt Keyframe-Index"). */
  progressLabel: string;
  /** The progress as shown next to the bar ("12 / 40", "30 %"). */
  progressText?: string;
  /** Remaining time, formatted by the page ("noch 3 min"). */
  remaining?: string;
}

/** A running background job on the overview (concept 11, card Aufgaben). */
export function TaskRow({
  type,
  subject,
  progress,
  progressLabel,
  progressText,
  remaining,
}: TaskRowProps) {
  const { t } = useTranslation();
  return (
    <div className={styles.row}>
      <span className={styles.text}>
        <span className={styles.title}>
          {subject ? [type, subject].join(t('format.separator')) : type}
        </span>
        <span className={styles.progress}>
          <ProgressBar value={progress} label={progressLabel} />
          {progressText && <span className={styles.figure}>{progressText}</span>}
        </span>
        {remaining && <span className={styles.line}>{remaining}</span>}
      </span>
    </div>
  );
}
