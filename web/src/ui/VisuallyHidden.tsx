import type { ReactNode } from 'react';
import styles from './VisuallyHidden.module.css';

/** Content for screen readers only (page headings, extra context). */
export function VisuallyHidden({
  children,
  as: Tag = 'span',
}: {
  children: ReactNode;
  as?: 'span' | 'h1' | 'h2';
}) {
  return <Tag className={styles.hidden}>{children}</Tag>;
}
