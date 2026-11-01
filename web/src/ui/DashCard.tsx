import { useId, type ReactNode } from 'react';
import styles from './DashCard.module.css';
import { Icon, type IconName } from './Icon';
import { Panel } from './Panel';
import { VisuallyHidden } from './VisuallyHidden';

interface DashCardProps {
  icon: IconName;
  title: string;
  /** Number in the head ("3", "5 / 8"), formatted by the page. */
  count?: string;
  /** Spoken form of the number ("3 Hinweise"); without it the number is read as shown. */
  countLabel?: string;
  /** The card's way on: a link or buttons, at the bottom of the card. */
  footer?: ReactNode;
  children: ReactNode;
}

/**
 * A card of the administration's overview (concept 11, 11.1): a Panel that fills its grid cell,
 * so the cards of a row are equally high. Head with icon, title and an optional number; the footer
 * sits at the bottom whatever the body's height.
 */
export function DashCard({ icon, title, count, countLabel, footer, children }: DashCardProps) {
  const id = useId();
  return (
    <Panel as="section" aria-labelledby={id}>
      <div className={styles.head}>
        <span className={styles.icon}>
          <Icon name={icon} />
        </span>
        <h2 id={id} className={styles.title}>
          {title}
        </h2>
        {count !== undefined && (
          <span className={styles.count}>
            <span aria-hidden={countLabel ? true : undefined}>{count}</span>
            {countLabel && <VisuallyHidden>{countLabel}</VisuallyHidden>}
          </span>
        )}
      </div>
      {children}
      {footer && <div className={styles.footer}>{footer}</div>}
    </Panel>
  );
}
