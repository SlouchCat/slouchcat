import type { Direction } from '@noriginmedia/norigin-spatial-navigation';
import { useCallback, useEffect, type ReactNode, type Ref } from 'react';

// Spatial (remote-control style) navigation for the browsing pages, built on norigin.
//
// - norigin moves DOM focus itself (`shouldFocusDOMNode`), so the native :focus-visible ring,
//   screen readers and Enter activation all work on real elements.
// - Native events stay untouched (`shouldUseNativeEvents`): Enter clicks the focused link or
//   button natively, and forms keep their arrow keys. SpatialPage prevents page scrolling on arrow
//   keys only while a browsing page is mounted.
// - Outside a SpatialPage norigin is paused; Tab order is always the DOM order.
// - norigin (with lodash-es, about 17 kB gzip) and the focus scroll (reveal.ts) are a chunk of
//   their own (spatialRuntime.ts), requested once the start bundle runs, so they do not share the
//   link with the hero image (P-10). The routes with the app header wait for it (router.tsx);
//   everything below may then use it synchronously.

type Norigin = typeof import('./spatialRuntime');

let norigin: Norigin | undefined;
let loading: Promise<void> | undefined;

/** Loads and initialises norigin (paused); repeated calls share one load, a failed one is retried. */
export function loadSpatialNavigation(): Promise<void> {
  loading ??= import('./spatialRuntime').then(
    (module) => {
      module.init({
        shouldFocusDOMNode: true,
        shouldUseNativeEvents: true,
        domNodeFocusOptions: { preventScroll: true },
        distanceCalculationMethod: 'center',
      });
      module.pause();
      norigin = module;
    },
    (error: unknown) => {
      loading = undefined;
      throw error;
    },
  );
  return loading;
}

function nav(): Norigin {
  if (!norigin) throw new Error('Spatial navigation used before loadSpatialNavigation() resolved');
  return norigin;
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
    const { doesFocusableExist, getCurrentFocusKey, pause, resume, setFocus } = nav();
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

/** Focus key of the element norigin focused last (by arrows, Tab or pointer). */
export const spatialFocusKey = (): string => nav().getCurrentFocusKey();

/** Moves focus to a registered element, for when the focused one goes away. */
export function setSpatialFocus(focusKey: string): void {
  void nav().setFocus(focusKey);
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
  const { getCurrentFocusKey, revealElement, useFocusable } = nav();
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
  }, [getCurrentFocusKey, focusSelf, key]);
  return { ref: ref as Ref<E>, onFocus, focusKey: key };
}

/** A registered child of a group, as norigin hands it to a resolver. */
interface Sibling {
  focusKey: string;
  node: HTMLElement;
}

/**
 * The child an arrow press moves to in a row that follows the DOM order: ← and → walk the
 * siblings in order, ↑ and ↓ (and the ends of the row) leave the group. Null hands the press to
 * the parent group, which decides by geometry.
 */
export function nextInOrder(
  direction: string,
  from: string,
  siblings: readonly Sibling[],
): Sibling | null {
  if (direction !== 'left' && direction !== 'right') return null;
  const ordered = [...siblings].sort((a, b) =>
    a.node.compareDocumentPosition(b.node) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1,
  );
  const index = ordered.findIndex((sibling) => sibling.focusKey === from);
  if (index < 0) return null;
  return ordered[index + (direction === 'right' ? 1 : -1)] ?? null;
}

interface SpatialGroupProps {
  children: ReactNode;
  focusKey?: string;
  className?: string;
  /** Remember the last focused child, so returning to a row lands where the viewer left it. */
  rememberLastChild?: boolean;
  /** Child that receives focus when the group is entered and no last child is remembered. */
  preferredChildFocusKey?: string;
  /** Arrow keys never leave the group (a modal drawer). */
  focusBoundary?: boolean;
  /**
   * Picks the child an arrow press moves to from the child `from` (focus keys), instead of the
   * geometry; null leaves the group. For containers whose children overlap, like the app shell's
   * header, side navigation and content.
   */
  nextFocusKey?: (direction: string, from: string) => string | null;
  /**
   * A row whose arrows follow the DOM order (nextInOrder): ← and → walk the children, ↑ and ↓
   * leave the row. For a row of buttons that wraps when the space runs out (the detail page's
   * actions in a wide font): by geometry ↓ from the first button would land on the one that
   * wrapped under it instead of the row below, and → would never reach it.
   */
  ordered?: boolean;
  as?: 'div' | 'ul' | 'nav' | 'section';
  'aria-label'?: string;
  'aria-labelledby'?: string;
  /** -1 keeps a scrolling group out of the Tab order (Firefox makes scroll boxes Tab stops). */
  tabIndex?: -1;
}

/** A spatial container (row, hero, header) whose children navigate among each other first. */
export function SpatialGroup({
  children,
  focusKey,
  className,
  rememberLastChild = true,
  preferredChildFocusKey,
  focusBoundary,
  nextFocusKey,
  ordered,
  as: Tag = 'div',
  ...aria
}: SpatialGroupProps) {
  const { FocusContext, useFocusable } = nav();
  const { ref, focusKey: key } = useFocusable<object, HTMLElement>({
    focusKey,
    saveLastFocusedChild: rememberLastChild,
    preferredChildFocusKey,
    isFocusBoundary: focusBoundary,
    nextFocusResolver: nextFocusKey
      ? (direction, from, siblings) => {
          const next = nextFocusKey(direction, from);
          return siblings.find((sibling) => sibling.focusKey === next) ?? null;
        }
      : ordered
        ? (direction, from, siblings) =>
            siblings.find((sibling) => sibling === nextInOrder(direction, from, siblings)) ?? null
        : undefined,
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
