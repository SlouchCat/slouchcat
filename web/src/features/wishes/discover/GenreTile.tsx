import { useState } from 'react';
import { AppLink, useSpatialItem } from '@/ui';
import styles from './WishesArea.module.css';

interface GenreTileProps {
  href: string;
  name: string;
  /** The genre's backdrop (getWishGenreBackdrop); without one the tile shows its name alone. */
  imageUrl?: string;
  focusKey?: string;
}

/**
 * A genre of the rows «Film-Genres» and «Serien-Genres» (concept 25, M-31): a wide tile with the
 * backdrop Seerr names for it and the name on it, a link to «Filme» or «Serien» with that genre.
 * Kept in the feature: no other page has genres with a picture (the start page's are GenreChip).
 */
export function GenreTile({ href, name, imageUrl, focusKey }: GenreTileProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  // A backdrop that does not come (404, TMDB out of reach) leaves the tile with its name alone.
  const [failed, setFailed] = useState(false);
  return (
    <AppLink ref={ref} href={href} className={styles.genre} onFocus={onFocus}>
      {imageUrl && !failed && (
        <img
          className={styles.genreImage}
          src={imageUrl}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailed(true)}
        />
      )}
      <span className={styles.genreName}>{name}</span>
    </AppLink>
  );
}
