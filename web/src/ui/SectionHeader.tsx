import type { ReactNode } from 'react';
import styles from './SectionHeader.module.css';

interface SectionHeaderProps {
  title: string;
  /** h2 for a section of the page, h3 for a thing inside it (a key, a library); deeper never (S-3). */
  level?: 2 | 3;
  /** Id of the heading, for `aria-labelledby` of the section it names. */
  id?: string;
  /** A short line under the title. */
  description?: string;
  /** Small state or actions at the end of the line ("3 Fehlschläge", «Neu»). */
  actions?: ReactNode;
}

/**
 * The title line of a section (concept 12, S-3): the heading with an optional line below it and
 * actions at its end. The size of an h2 is the token `--sc-section-title-size`, so the compact
 * density shrinks it with the rest.
 */
export function SectionHeader({ title, level = 2, id, description, actions }: SectionHeaderProps) {
  const Heading = level === 2 ? 'h2' : 'h3';
  return (
    <div className={styles.header}>
      <div className={styles.line}>
        <Heading id={id} className={`${styles.title} ${level === 2 ? styles.h2 : styles.h3}`}>
          {title}
        </Heading>
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
      {description && <p className={styles.description}>{description}</p>}
    </div>
  );
}
