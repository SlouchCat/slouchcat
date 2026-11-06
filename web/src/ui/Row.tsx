import { Children, useId, useRef, type ReactNode, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { AppLink } from './AppLink';
import { Button } from './Button';
import styles from './Row.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

interface RowProps {
  title: string;
  children: ReactNode;
  focusKey?: string;
  /** `chips`: items as wide as their content (genre chips) instead of one poster wide. */
  variant?: 'cards' | 'chips';
  /** A page with all of the row's entries ("Alle anzeigen ›"), linked right of the title. */
  moreHref?: string;
  moreLabel?: string;
}

interface RowListProps {
  children: ReactNode;
  focusKey?: string;
  variant?: 'cards' | 'chips';
  /** The scroll container, for a row that scrolls itself (the spotlight row of HeroCarousel). */
  scrollerRef?: Ref<HTMLDivElement>;
  /** Accessible name of the list, for a row without its own heading. */
  'aria-label'?: string;
  rememberLastChild?: boolean;
  preferredChildFocusKey?: string;
  /**
   * Visual place of the child at `position` (CSS `order`), for a row that turns without changing
   * its DOM order (the spotlight row, concept 15, 6.3). Each item then carries its fixed position
   * (`aria-posinset`, `aria-setsize`), which the turning never changes. Such a row does not snap:
   * a swipe scrolls it freely, and a snap would pull it back to its old first card after a turn.
   */
  order?: (position: number) => number;
}

/**
 * The scrolling list of a row: its cards (or chips) with the row's geometry and snapping. Row uses
 * it under its title; HeroCarousel uses it for its poster row, so the posters have the normal card
 * size.
 */
export function RowList({
  children,
  focusKey,
  variant = 'cards',
  scrollerRef,
  rememberLastChild,
  preferredChildFocusKey,
  order,
  ...aria
}: RowListProps) {
  const count = Children.count(children);
  return (
    // -1: Firefox makes a scroll box a Tab stop of its own; the cards are the stops.
    <div
      ref={scrollerRef}
      className={order ? `${styles.scroller} ${styles.free}` : styles.scroller}
      tabIndex={-1}
    >
      <SpatialGroup
        focusKey={focusKey}
        as="ul"
        className={styles.list}
        rememberLastChild={rememberLastChild}
        preferredChildFocusKey={preferredChildFocusKey}
        {...aria}
      >
        {Children.map(children, (child, position) => (
          <li
            className={variant === 'chips' ? styles.chip : styles.item}
            style={order && { order: order(position) }}
            aria-posinset={order && position + 1}
            aria-setsize={order && count}
          >
            {child}
          </li>
        ))}
      </SpatialGroup>
    </div>
  );
}

/** The row's link to its full page; arrow keys reach it like a card. */
function MoreLink({ href, label }: { href: string; label: string }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>();
  return (
    <AppLink ref={ref} href={href} className={styles.more} onFocus={onFocus}>
      {label}
    </AppLink>
  );
}

/**
 * Titled horizontal row of cards. Arrow keys move inside the row (spatial navigation); pointer
 * users scroll with the paging buttons, touch users swipe.
 */
export function Row({
  title,
  children,
  focusKey,
  variant = 'cards',
  moreHref,
  moreLabel,
}: RowProps) {
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
        {moreHref && moreLabel && <MoreLink href={moreHref} label={moreLabel} />}
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
      <RowList scrollerRef={scroller} focusKey={focusKey} variant={variant}>
        {children}
      </RowList>
    </section>
  );
}
