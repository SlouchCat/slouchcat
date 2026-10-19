import { AppLink } from './AppLink';
import styles from './CastCard.module.css';
import { useSpatialItem } from './spatial';

interface CastCardProps {
  href: string;
  name: string;
  /** Character or role, translated by the page ("als Ellen Ripley", "Regie"). */
  caption?: string;
  focusKey?: string;
}

/**
 * A person in the cast row of a title: initial, name and role, one link to the person's page.
 * There are no portraits in M3 (the image endpoint is per title).
 */
export function CastCard({ href, name, caption, focusKey }: CastCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink ref={ref} href={href} className={styles.card} onFocus={onFocus}>
      <span className={styles.avatar} aria-hidden="true">
        {name.slice(0, 1).toUpperCase()}
      </span>
      <span className={styles.name}>{name}</span>
      {caption && <span className={styles.caption}>{caption}</span>}
    </AppLink>
  );
}
