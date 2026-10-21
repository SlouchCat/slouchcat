import { Children, useId, useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from './Button';
import styles from './Row.module.css';
import { SpatialGroup } from './spatial';

interface RowProps {
  title: string;
  children: ReactNode;
  focusKey?: string;
  /** `chips`: items as wide as their content (genre chips) instead of one poster wide. */
  variant?: 'cards' | 'chips';
}

/**
 * Titled horizontal row of cards. Arrow keys move inside the row (spatial navigation); pointer
 * users scroll with the paging buttons, touch users swipe.
 */
export function Row({ title, children, focusKey, variant = 'cards' }: RowProps) {
  const { t } = useTranslation();
  const headingId = useId();
  const scroller = useRef<HTMLDivElement>(null);

  const page = (direction: -1 | 1) => {
    const node = scroller.current;
    if (!node) return;
    node.scrollBy({ left: direction * node.clientWidth * 0.8, behavior: 'smooth' });
  };

  return (
    <section className={styles.row} aria-labelledby={headingId}>
      <div className={styles.header}>
        <h2 id={headingId} className={styles.title}>
          {title}
        </h2>
        <div className={styles.paging}>
          <Button
            variant="ghost"
            icon="chevronLeft"
            iconOnly
            excludeFromTabOrder
            aria-label={t('ui.row.previous')}
            onPress={() => page(-1)}
          />
          <Button
            variant="ghost"
            icon="chevronRight"
            iconOnly
            excludeFromTabOrder
            aria-label={t('ui.row.next')}
            onPress={() => page(1)}
          />
        </div>
      </div>
      <div ref={scroller} className={styles.scroller}>
        <SpatialGroup focusKey={focusKey} as="ul" className={styles.list}>
          {Children.map(children, (child) => (
            <li className={variant === 'chips' ? styles.chip : styles.item}>{child}</li>
          ))}
        </SpatialGroup>
      </div>
    </section>
  );
}
