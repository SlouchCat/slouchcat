import {
  Children,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type PointerEvent,
  type ReactNode,
  type RefObject,
} from 'react';
import { useTranslation } from 'react-i18next';
import { AppLink } from './AppLink';
import { Button } from './Button';
import styles from './Row.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

interface RowProps {
  title: string;
  children: ReactNode;
  focusKey?: string;
  /**
   * `chips`: items as wide as their content (genre chips) instead of one poster wide; `tiles`: two
   * posters wide (the library tiles of «Meine Medien», concept 15 P-7).
   */
  variant?: 'cards' | 'chips' | 'tiles';
  /** A page with all of the row's entries ("Alle anzeigen ›"), linked right of the title. */
  moreHref?: string;
  moreLabel?: string;
}

/**
 * The paging buttons of a row (concept 20 P-1): `hover` at the right end of the row's `header`,
 * shown only with a mouse while the pointer is on the row (the start page); `always` fixed left
 * and right beside the row, on touch screens too, disabled at the start and the end and gone when
 * every card fits; `none` without buttons. Never Tab stops: the arrow keys reach every card.
 */
export type RowArrows = 'hover' | 'always' | 'none';

/** A pointer move further than this (px) drags the row and is no click on a card. */
const DRAG_THRESHOLD = 6;

interface RowListProps {
  children: ReactNode;
  focusKey?: string;
  variant?: 'cards' | 'chips' | 'tiles';
  arrows?: RowArrows;
  /** The row's title block (Row); with `arrows="hover"` the paging buttons close it on the right. */
  header?: ReactNode;
  /**
   * The scroll container, for a row that scrolls itself (the spotlight row of HeroCarousel) or
   * brings a card into view (the season shelf).
   */
  scrollerRef?: RefObject<HTMLDivElement | null>;
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

/** Where a row stands: at its start, at its end, or with every card in view. */
interface Edges {
  fits: boolean;
  start: boolean;
  end: boolean;
}

const sameEdges = (a: Edges, b: Edges) =>
  a.fits === b.fits && a.start === b.start && a.end === b.end;

/**
 * The scrolling list of a row: its cards (or chips) with the row's geometry and snapping, and its
 * paging buttons (`arrows`). Row uses it under its title; HeroCarousel uses it for its poster row,
 * so the posters have the normal card size.
 *
 * With `arrows="always"` a mouse can also drag the row sideways (a move over 6 px is not a click on
 * a card), and the row snaps only near a card (`proximity`), so a drag or a trackpad stops where it
 * was let go. The mouse wheel keeps scrolling the page; Shift+wheel scrolls the row.
 */
export function RowList({
  children,
  focusKey,
  variant = 'cards',
  arrows = 'none',
  header,
  scrollerRef,
  rememberLastChild,
  preferredChildFocusKey,
  order,
  ...aria
}: RowListProps) {
  const { t } = useTranslation();
  const count = Children.count(children);
  const own = useRef<HTMLDivElement>(null);
  const scroller = scrollerRef ?? own;
  const fixed = arrows === 'always';
  // Shown until measured: a card brought into view before stays in view when they go.
  const [edges, setEdges] = useState<Edges>({ fits: false, start: true, end: false });
  const drag = useRef<{ x: number; left: number; moved: boolean; snap: string } | null>(null);
  const swallowClick = useRef(false);

  // The fixed arrows follow the scroll position and the sizes of the row and its cards.
  useLayoutEffect(() => {
    const node = scroller.current;
    if (!fixed || !node) return;
    const update = () => {
      const max = node.scrollWidth - node.clientWidth;
      const next = { fits: max <= 1, start: node.scrollLeft <= 1, end: node.scrollLeft >= max - 1 };
      setEdges((previous) => (sameEdges(previous, next) ? previous : next));
    };
    update();
    node.addEventListener('scroll', update, { passive: true });
    const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(update) : undefined;
    observer?.observe(node);
    if (node.firstElementChild) observer?.observe(node.firstElementChild);
    return () => {
      node.removeEventListener('scroll', update);
      observer?.disconnect();
    };
  }, [fixed, scroller, count]);

  const page = (direction: -1 | 1) => {
    const node = scroller.current;
    if (!node) return;
    node.scrollBy({ left: direction * node.clientWidth * 0.8, behavior: 'smooth' });
  };

  // Dragging with the mouse; touch and pen swipe natively.
  const onPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    const node = scroller.current;
    if (!node || event.pointerType !== 'mouse' || event.button !== 0) return;
    drag.current = { x: event.clientX, left: node.scrollLeft, moved: false, snap: '' };
  };
  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const node = scroller.current;
    const state = drag.current;
    if (!node || !state) return;
    const distance = event.clientX - state.x;
    if (!state.moved) {
      if (Math.abs(distance) <= DRAG_THRESHOLD) return;
      state.moved = true;
      state.snap = node.style.scrollSnapType;
      // Snapping would pull the row back to a card on every step of the drag.
      node.style.scrollSnapType = 'none';
      node.dataset.dragging = '';
      node.setPointerCapture(event.pointerId);
    }
    node.scrollLeft = state.left - distance;
  };
  const endDrag = (event: PointerEvent<HTMLDivElement>) => {
    const node = scroller.current;
    const state = drag.current;
    drag.current = null;
    if (!node || !state?.moved) return;
    swallowClick.current = true;
    node.style.scrollSnapType = state.snap;
    delete node.dataset.dragging;
    if (node.hasPointerCapture(event.pointerId)) node.releasePointerCapture(event.pointerId);
  };

  const pagingButton = (direction: -1 | 1, isDisabled?: boolean) => (
    <Button
      variant={fixed ? 'secondary' : 'ghost'}
      icon={direction < 0 ? 'chevronLeft' : 'chevronRight'}
      iconOnly
      excludeFromTabOrder
      isDisabled={isDisabled}
      aria-label={t(direction < 0 ? 'ui.row.previous' : 'ui.row.next')}
      onPress={() => page(direction)}
    />
  );

  const list = (
    // -1: Firefox makes a scroll box a Tab stop of its own; the cards are the stops.
    <div
      ref={scroller}
      className={[styles.scroller, order && styles.free, fixed && styles.fixed]
        .filter(Boolean)
        .join(' ')}
      tabIndex={-1}
      {...(fixed && {
        onPointerDown,
        onPointerMove,
        onPointerUp: endDrag,
        onPointerCancel: endDrag,
        // A link would start the browser's own drag of its address instead.
        onDragStart: (event) => event.preventDefault(),
        onClickCapture: (event) => {
          if (!swallowClick.current) return;
          swallowClick.current = false;
          event.preventDefault();
          event.stopPropagation();
        },
      })}
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
            className={
              variant === 'chips' ? styles.chip : variant === 'tiles' ? styles.tile : styles.item
            }
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

  if (fixed) {
    return (
      <div className={styles.rail}>
        {!edges.fits && pagingButton(-1, edges.start)}
        {list}
        {!edges.fits && pagingButton(1, edges.end)}
      </div>
    );
  }
  if (arrows === 'hover') {
    return (
      <>
        <div className={styles.header}>
          {header}
          <div className={styles.paging}>
            {pagingButton(-1)}
            {pagingButton(1)}
          </div>
        </div>
        {list}
      </>
    );
  }
  return (
    <>
      {header}
      {list}
    </>
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
  const headingId = useId();
  return (
    <section className={styles.row} aria-labelledby={headingId}>
      <RowList
        focusKey={focusKey}
        variant={variant}
        arrows="hover"
        header={
          <>
            <h2 id={headingId} className={styles.title}>
              {title}
            </h2>
            {moreHref && moreLabel && <MoreLink href={moreHref} label={moreLabel} />}
          </>
        }
      >
        {children}
      </RowList>
    </section>
  );
}
