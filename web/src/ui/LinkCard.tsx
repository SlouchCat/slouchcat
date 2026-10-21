import { useId } from 'react';
import { AppLink } from './AppLink';
import { Icon, type IconName } from './Icon';
import styles from './LinkCard.module.css';

interface LinkCardProps {
  href: string;
  /** The link's accessible name. */
  title: string;
  /** One line about the destination, read as the link's description. */
  description?: string;
  icon?: IconName;
}

/**
 * A card that is one link (the cards of the admin overview): icon, title and a line of text. The
 * title names the link, the line describes it, so the name stays short for screen readers.
 */
export function LinkCard({ href, title, description, icon }: LinkCardProps) {
  const id = useId();
  return (
    <AppLink
      href={href}
      className={styles.card}
      aria-labelledby={`${id}-title`}
      aria-describedby={description ? `${id}-description` : undefined}
    >
      {icon && (
        <span className={styles.icon}>
          <Icon name={icon} size="lg" />
        </span>
      )}
      <span className={styles.text}>
        <span id={`${id}-title`} className={styles.title}>
          {title}
        </span>
        {description && (
          <span id={`${id}-description`} className={styles.description}>
            {description}
          </span>
        )}
      </span>
    </AppLink>
  );
}
