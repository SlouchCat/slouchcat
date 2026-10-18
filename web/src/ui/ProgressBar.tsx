import { ProgressBar as AriaProgressBar } from 'react-aria-components';
import styles from './ProgressBar.module.css';

interface ProgressBarProps {
  /** 0 … 100. */
  value: number;
  /** Accessible description, e.g. "42 % gesehen". */
  label: string;
  size?: 'thin' | 'regular';
}

/** Thin progress indicator for watch progress and scan progress. */
export function ProgressBar({ value, label, size = 'thin' }: ProgressBarProps) {
  return (
    <AriaProgressBar value={value} aria-label={label} className={`${styles.track} ${styles[size]}`}>
      {({ percentage }) => (
        <span className={styles.fill} style={{ width: `${percentage ?? 0}%` }} />
      )}
    </AriaProgressBar>
  );
}
