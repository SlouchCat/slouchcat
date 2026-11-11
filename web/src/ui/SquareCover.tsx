import { useState } from 'react';
import { swatchStyle } from './accent';
import { stationAccent, stationInitials } from './StationLogo';
import styles from './SquareCover.module.css';

interface SquareCoverProps {
  /** The title the cover belongs to; gives the initials and their colour without a picture. */
  title: string;
  /** URL of the cover; without one (or when it fails to load) the initials show. */
  src?: string;
  /** `sm` 3 rem (a row of an admin list), `md` 4.5 rem, `lg` 7.5 rem (the head of a page). */
  size?: 'sm' | 'md' | 'lg';
}

/**
 * The square cover of a podcast or an audiobook in the administration's lists and pages (concept
 * 19 part A, 12.4): the picture fills the square (`object-fit: cover`), the initials of the title
 * on its accent colour stand in for a missing one. Decorative: the row or heading next to it
 * names the title.
 */
export function SquareCover({ title, src, size = 'sm' }: SquareCoverProps) {
  // The address that failed: a new content hash gives the picture another try.
  const [failed, setFailed] = useState<string | null>(null);
  const image = src !== undefined && src !== failed ? src : undefined;
  return (
    <span
      className={`${styles.cover} ${styles[size]}`}
      style={image ? undefined : swatchStyle(stationAccent(title))}
      data-initials={image ? undefined : ''}
      aria-hidden="true"
    >
      {image ? (
        <img
          className={styles.image}
          src={image}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailed(image)}
        />
      ) : (
        // Drawn by CSS from the attribute, so the initials stay out of the text of a row or a
        // button the cover sits in (its name and textContent are the title's).
        <span className={styles.initials} data-text={stationInitials(title)} />
      )}
    </span>
  );
}
