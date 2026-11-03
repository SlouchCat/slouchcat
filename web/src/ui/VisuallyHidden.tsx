import type { ReactNode } from 'react';
import styles from './VisuallyHidden.module.css';

/** Content for screen readers only (page headings, extra context, a live region). */
export function VisuallyHidden({
  children,
  as: Tag = 'span',
  id,
  role,
}: {
  children?: ReactNode;
  as?: 'span' | 'div' | 'h1' | 'h2';
  /** For a section labelled by this heading (`aria-labelledby`). */
  id?: string;
  /** `status`: a live region that is always there, so even its first message is announced. */
  role?: 'status';
}) {
  return (
    <Tag id={id} role={role} className={styles.hidden}>
      {children}
    </Tag>
  );
}
