import { createContext, useContext, type ReactNode } from 'react';
import styles from './Page.module.css';

const PageNavContext = createContext<ReactNode>(null);

/**
 * Puts `nav` (a section navigation such as the administration's) at the top of every Page below
 * it, inside the page's column, so it lines up with the page's heading.
 */
export function PageNav({ nav, children }: { nav: ReactNode; children: ReactNode }) {
  return <PageNavContext.Provider value={nav}>{children}</PageNavContext.Provider>;
}

interface PageProps {
  children: ReactNode;
  /** `form`: a centred narrow column (setup, login); `content`: page under the app header. */
  layout?: 'form' | 'content';
}

/** Main landmark that every route renders into. */
export function Page({ children, layout = 'content' }: PageProps) {
  const nav = useContext(PageNavContext);
  return (
    <main className={`${styles.page} ${styles[layout]}`}>
      <div className={styles.column}>
        {nav}
        {children}
      </div>
    </main>
  );
}
