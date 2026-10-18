import type { ReactNode } from 'react';
import styles from './Page.module.css';

interface PageProps {
  children: ReactNode;
  /** `form`: a centred narrow column (setup, login); `content`: page under the app header. */
  layout?: 'form' | 'content';
}

/** Main landmark that every route renders into. */
export function Page({ children, layout = 'content' }: PageProps) {
  return (
    <main className={`${styles.page} ${styles[layout]}`}>
      <div className={styles.column}>{children}</div>
    </main>
  );
}
