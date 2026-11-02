import type { ReactNode } from 'react';
import styles from './VisuallyHidden.module.css';

/** Content for screen readers only (page headings, extra context). */
export function VisuallyHidden({
  children,
  as: Tag = 'span',
  id,
}: {
  children: ReactNode;
  as?: 'span' | 'h1' | 'h2';
  /** For a section labelled by this heading (`aria-labelledby`). */
  id?: string;
}) {
  return (
    <Tag id={id} className={styles.hidden}>
      {children}
    </Tag>
  );
}
