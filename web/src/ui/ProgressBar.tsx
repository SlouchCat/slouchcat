import { useTranslation } from 'react-i18next';
import styles from './ProgressBar.module.css';

interface ProgressBarProps {
  /** 0 … 100. */
  value: number;
  /** Accessible description, e.g. "42 % gesehen". */
  label: string;
  size?: 'thin' | 'regular';
}

/** Thin progress indicator for watch progress and scan progress (role progressbar, 0 … 100). */
export function ProgressBar({ value, label, size = 'thin' }: ProgressBarProps) {
  const { i18n } = useTranslation();
  const percent = Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0));
  const valueText = new Intl.NumberFormat(i18n.language, {
    style: 'percent',
    maximumFractionDigits: 0,
  }).format(percent / 100);
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={percent}
      aria-valuetext={valueText}
      className={`${styles.track} ${styles[size]}`}
    >
      <span className={styles.fill} style={{ width: `${percent}%` }} />
    </div>
  );
}
