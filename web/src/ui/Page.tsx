import type { ReactNode } from 'react';
import styles from './Page.module.css';

interface PageProps {
  children: ReactNode;
}

/** Full-height main landmark that every route renders into. */
export function Page({ children }: PageProps) {
  return <main className={styles.page}>{children}</main>;
}
