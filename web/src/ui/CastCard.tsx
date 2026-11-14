import { useState } from 'react';
import { AppLink } from './AppLink';
import styles from './CastCard.module.css';
import { useSpatialItem } from './spatial';

interface CastCardProps {
  href: string;
  name: string;
  /** Character or role, translated by the page ("als Ellen Ripley", "Regie"). */
  caption?: string;
  /** Portrait of the person; absent or failing to load, the initial shows instead. */
  imageUrl?: string;
  focusKey?: string;
}

/**
 * A person in the cast row of a title, or among the search results (concept 21 P-7): a 2:3
 * portrait cut towards the face (the initial on a neutral ground without one), the name and at
 * most two lines of the role under it, left aligned; one link to the person's page. Round
 * portraits (Avatar) stay for profiles.
 */
export function CastCard({ href, name, caption, imageUrl, focusKey }: CastCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  // The URL that failed, so a new URL (a refreshed hash) gets its own try.
  const [failed, setFailed] = useState<string>();
  const shown = imageUrl !== undefined && imageUrl !== failed;
  return (
    <AppLink ref={ref} href={href} className={styles.card} onFocus={onFocus}>
      <span className={styles.portrait} aria-hidden="true">
        {shown ? (
          <img
            className={styles.image}
            src={imageUrl}
            alt=""
            loading="lazy"
            decoding="async"
            draggable={false}
            onError={() => setFailed(imageUrl)}
          />
        ) : (
          Array.from(name)[0]?.toUpperCase()
        )}
      </span>
      <span className={styles.name}>{name}</span>
      {caption && <span className={styles.caption}>{caption}</span>}
    </AppLink>
  );
}
