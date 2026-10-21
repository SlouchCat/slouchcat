import { AppLink } from './AppLink';
import styles from './GenreChip.module.css';
import { useSpatialItem } from './spatial';

interface GenreChipProps {
  href: string;
  /** Genre name in the profile language. */
  name: string;
  /** Number of visible titles, shown muted after the name. */
  count?: number;
  focusKey?: string;
}

/** A genre as a pill-shaped link (start page row "Genres"): opens the search filtered by it. */
export function GenreChip({ href, name, count, focusKey }: GenreChipProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink ref={ref} href={href} className={styles.chip} onFocus={onFocus}>
      <span>{name}</span>
      {count !== undefined && (
        <span className={styles.count} aria-hidden="true">
          {count}
        </span>
      )}
    </AppLink>
  );
}
