import { useState, type ReactNode } from 'react';
import { apiUrl } from '@/api/client';
import { PosterFallback } from '@/ui';
import styles from './PosterTitle.module.css';

/**
 * A title cell of an admin table with its poster in front (mockups H and I): the `w200` poster
 * through `getAdminItemImage`, system-wide. The poster is decorative (`alt=""`), so the row header
 * keeps the title as its only text. Without a poster, or when it fails to load, the placeholder.
 */
export function PosterTitle({
  posterUrl,
  className,
  children,
}: {
  posterUrl?: string;
  className?: string;
  children: ReactNode;
}) {
  // The address that failed: a new content hash gives the image another try.
  const [failed, setFailed] = useState<string | null>(null);
  return (
    <span className={`${styles.row} ${className ?? ''}`}>
      <span className={styles.poster}>
        {posterUrl && failed !== posterUrl ? (
          <img
            className={styles.image}
            src={apiUrl(posterUrl)}
            alt=""
            width={40}
            height={60}
            loading="lazy"
            decoding="async"
            onError={() => setFailed(posterUrl)}
          />
        ) : (
          <PosterFallback />
        )}
      </span>
      {children}
    </span>
  );
}
