import type { ReactNode } from 'react';
import styles from './Chip.module.css';
import { Icon, type IconName } from './Icon';

interface ChipProps {
  children: ReactNode;
  tone?: 'neutral' | 'outline' | 'warning' | 'danger' | 'success' | 'info';
  icon?: IconName;
}

/** Small label for metadata (year, FSK, codec) and states (network share, scan state). */
export function Chip({ children, tone = 'neutral', icon }: ChipProps) {
  return (
    <span className={`${styles.chip} ${styles[tone]}`}>
      {icon && <Icon name={icon} size="sm" />}
      {children}
    </span>
  );
}
