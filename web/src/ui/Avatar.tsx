import { useState } from 'react';
import styles from './Avatar.module.css';

interface AvatarProps {
  /** Name of the person: its first letter is shown without a portrait. */
  name: string;
  /** Portrait URL; absent or failing to load, the letter shows instead. */
  imageUrl?: string;
  size?: 'sm' | 'lg';
}

/**
 * Round portrait of a person, or the first letter of the name on a neutral ground. Decorative: the
 * name is always written next to it.
 */
export function Avatar({ name, imageUrl, size = 'sm' }: AvatarProps) {
  // The URL that failed, so a new URL (another profile, a refreshed hash) gets its own try.
  const [failed, setFailed] = useState<string>();
  const shown = imageUrl !== undefined && imageUrl !== failed;
  return (
    <span className={`${styles.avatar} ${styles[size]}`} aria-hidden="true">
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
  );
}
