import type { ReactNode } from 'react';
import { Icon } from './Icon';
import styles from './Notice.module.css';

interface NoticeProps {
  title?: string;
  children?: ReactNode;
  tone?: 'info' | 'warning' | 'danger';
  /** Buttons or links below the text. */
  actions?: ReactNode;
}

/**
 * Inline message. Danger notices are announced immediately (role alert), the others politely.
 */
export function Notice({ title, children, tone = 'info', actions }: NoticeProps) {
  return (
    <div
      className={`${styles.notice} ${styles[tone]}`}
      role={tone === 'danger' ? 'alert' : 'status'}
    >
      <span className={styles.icon}>
        <Icon name={tone === 'info' ? 'info' : 'warning'} />
      </span>
      <div className={styles.body}>
        {title && <p className={styles.title}>{title}</p>}
        {children && <div className={styles.text}>{children}</div>}
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
    </div>
  );
}
