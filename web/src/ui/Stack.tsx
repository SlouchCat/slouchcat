import type { ReactNode } from 'react';
import styles from './Stack.module.css';

const gapClass = {
  1: styles.gap1,
  2: styles.gap2,
  4: styles.gap4,
  6: styles.gap6,
  8: styles.gap8,
} as const;

interface StackProps {
  children: ReactNode;
  /** Step on the 4 px spacing scale (`space.<gap>` token). */
  gap?: keyof typeof gapClass;
  direction?: 'column' | 'row';
}

/** Lays out children in a column (default) or row with a token-based gap. */
export function Stack({ children, gap = 4, direction = 'column' }: StackProps) {
  const className = [styles.stack, direction === 'row' && styles.row, gapClass[gap]]
    .filter(Boolean)
    .join(' ');
  return <div className={className}>{children}</div>;
}
