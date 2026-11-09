import { AppLink } from './AppLink';
import { Avatar } from './Avatar';
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
 * A person in the cast row of a title, or among the search results: round portrait (the initial
 * without one), name and role, one link to the person's page.
 */
export function CastCard({ href, name, caption, imageUrl, focusKey }: CastCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink ref={ref} href={href} className={styles.card} onFocus={onFocus}>
      <span className={styles.portrait}>
        <Avatar name={name} imageUrl={imageUrl} />
      </span>
      <span className={styles.name}>{name}</span>
      {caption && <span className={styles.caption}>{caption}</span>}
    </AppLink>
  );
}
