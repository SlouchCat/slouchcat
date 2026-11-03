import row from './DashRow.module.css';
import { ProgressBar } from './ProgressBar';
import styles from './StreamProgress.module.css';

interface StreamProgressProps {
  /** 0 … 100; without it only the text shows (a stream whose length is unknown). */
  value?: number;
  /** Accessible name of the progress bar ("Fortschritt Oppenheimer"). */
  label: string;
  /** Position and duration as shown next to the bar ("0:42:10 / 3:00:00"). */
  text?: string;
}

/**
 * The progress of a running stream, the bar and the position next to it: the part the overview's
 * StreamRow and the streams page's StreamTile share (concept 12, 11.2).
 */
export function StreamProgress({ value, label, text }: StreamProgressProps) {
  return (
    <span className={row.progress}>
      {value !== undefined && <ProgressBar value={value} label={label} />}
      {text && (
        <span className={value === undefined ? `${row.figure} ${styles.alone}` : row.figure}>
          {text}
        </span>
      )}
    </span>
  );
}
