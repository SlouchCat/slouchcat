import type { ReactNode } from 'react';
import styles from './Panel.module.css';

interface PanelProps {
  children: ReactNode;
  as?: 'div' | 'section' | 'article' | 'fieldset';
  'aria-labelledby'?: string;
}

/** Quiet surface that groups related content (form steps, library cards). */
export function Panel({ children, as: Tag = 'div', ...aria }: PanelProps) {
  return (
    <Tag className={styles.panel} {...aria}>
      {children}
    </Tag>
  );
}
