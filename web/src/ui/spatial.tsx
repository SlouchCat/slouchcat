import {
  FocusContext,
  getCurrentFocusKey,
  init,
  pause,
  resume,
  setFocus,
  doesFocusableExist,
  useFocusable,
  type Direction,
} from '@noriginmedia/norigin-spatial-navigation';
import { useCallback, useEffect, type ReactNode, type Ref } from 'react';

// Spatial (remote-control style) navigation for the browsing pages, built on norigin.
//
// - norigin moves DOM focus itself (`shouldFocusDOMNode`), so the native :focus-visible ring,
//   screen readers and Enter activation all work on real elements.
// - Native events stay untouched (`shouldUseNativeEvents`): Enter clicks the focused link or
//   button natively, and forms keep their arrow keys. SpatialPage prevents page scrolling on arrow
//   keys only while a browsing page is mounted.
// - Outside a SpatialPage norigin is paused; Tab order is always the DOM order.

let initialised = false;

export function initSpatialNavigation(): void {
  if (initialised) return;
  init({
    shouldFocusDOMNode: true,
    shouldUseNativeEvents: true,
    domNodeFocusOptions: { preventScroll: true },
    distanceCalculationMethod: 'center',
  });
  pause();
  initialised = true;
}

const reducedMotion = () =>
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/** Scrolls a focused element into view the way the rows expect it (centred horizontally). */
export function revealElement(node: Element): void {
  if (typeof node.scrollIntoView !== 'function') return;
  node.scrollIntoView({
    block: 'nearest',
    inline: 'center',
    behavior: reducedMotion() ? 'auto' : 'smooth',
  });
}

const ARROWS: Record<string, Direction> = {
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
};

const isEditable = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));

interface SpatialPageProps {
  children: ReactNode;
  /** Focus key that receives focus on the first arrow press when nothing is focused yet. */
  defaultFocusKey: string;
}

/** Enables arrow-key navigation while a browsing page (home, detail) is mounted. */
export function SpatialPage({ children, defaultFocusKey }: SpatialPageProps) {
  useEffect(() => {
    initSpatialNavigation();
    resume();

    const onKeyDown = (event: KeyboardEvent) => {
      if (!(event.key in ARROWS) || isEditable(event.target) || event.altKey || event.metaKey) {
        return;
      }
      // Arrow keys move focus here, they must not scroll the page as well.
      event.preventDefault();
      const active = document.activeElement;
      if (active && active !== document.body) return;
      // Nothing focused yet (fresh page or a click on empty space): land on the remembered or
      // the default element instead of navigating from a stale position.
      event.stopImmediatePropagation();
      const current = getCurrentFocusKey();
      void setFocus(current && doesFocusableExist(current) ? current : defaultFocusKey);
    };
    window.addEventListener('keydown', onKeyDown, { capture: true });
    return () => {
      window.removeEventListener('keydown', onKeyDown, { capture: true });
      pause();
    };
  }, [defaultFocusKey]);

  return <>{children}</>;
}

interface SpatialItemOptions {
  focusKey?: string;
  /** Return false to consume the arrow press (for example to page a carousel). */
  onArrowPress?: (direction: string) => boolean;
}

/**
 * Registers a focusable leaf (button, link, card). Returns a ref and an onFocus handler that keeps
 * norigin in sync when focus arrives by Tab or pointer instead of arrow keys.
 */
export function useSpatialItem<E extends HTMLElement>({
  focusKey,
  onArrowPress,
}: SpatialItemOptions = {}) {
  const {
    ref,
    focusSelf,
    focusKey: key,
  } = useFocusable<object, E>({
    focusKey,
    onArrowPress: onArrowPress ? (direction) => onArrowPress(direction) : undefined,
    onFocus: (layout) => revealElement(layout.node),
  });
  const onFocus = useCallback(() => {
    if (getCurrentFocusKey() !== key) focusSelf();
  }, [focusSelf, key]);
  return { ref: ref as Ref<E>, onFocus, focusKey: key };
}

interface SpatialGroupProps {
  children: ReactNode;
  focusKey?: string;
  className?: string;
  /** Remember the last focused child, so returning to a row lands where the viewer left it. */
  rememberLastChild?: boolean;
  as?: 'div' | 'ul' | 'nav' | 'section';
  'aria-label'?: string;
  'aria-labelledby'?: string;
}

/** A spatial container (row, hero, header) whose children navigate among each other first. */
export function SpatialGroup({
  children,
  focusKey,
  className,
  rememberLastChild = true,
  as: Tag = 'div',
  ...aria
}: SpatialGroupProps) {
  const { ref, focusKey: key } = useFocusable<object, HTMLElement>({
    focusKey,
    saveLastFocusedChild: rememberLastChild,
    trackChildren: false,
  });
  return (
    <FocusContext.Provider value={key}>
      <Tag ref={ref as unknown as Ref<never>} className={className} {...aria}>
        {children}
      </Tag>
    </FocusContext.Provider>
  );
}
