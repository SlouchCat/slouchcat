import {
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type KeyboardEvent,
  type RefObject,
} from 'react';
import type { AdminSideNavTone } from './AdminSideNav';
import { AppLink } from './AppLink';
import styles from './AreaTabs.module.css';

/**
 * One page of the area. On screen a number, pill or badge stays short and is hidden from screen
 * readers; `accessibleLabel` then speaks it ("Pflege, 12 Titel brauchen Pflege").
 */
export interface AreaTab {
  href: string;
  label: string;
  current?: boolean;
  /** Muted number after the name ("2.318", "3 / 8"), formatted by the page. */
  count?: string;
  /** Coloured pill after the name instead of the count. */
  alarm?: { value: string; tone: AdminSideNavTone };
  /** Small badge after the name ("Update"). */
  badge?: string;
  /** The spoken name with the spelled-out number, pill or badge. */
  accessibleLabel?: string;
}

export interface AreaTabsProps {
  /** Accessible name of the navigation ("Seiten im Bereich Medien"). */
  label: string;
  tabs: readonly AreaTab[];
  /**
   * Following a tab replaces the page and these tabs with it. A ref that outlives the pages (the
   * admin layout's) carries the focused tab over to the next page's copy, as AdminSideNav does.
   */
  focusCarry?: RefObject<number | null>;
}

/**
 * The pages of an administration area as tabs above the page (build plan 2.5): a navigation of
 * links in the look of the mockup's tab bar (a rounded box, the current page raised), each with
 * the number, pill or badge the sidebar's entry used to carry. It wraps on narrow screens.
 *
 * Keyboard like the admin sidebar and Tabs: a single Tab stop on the current page (roving
 * tabIndex); ArrowRight and ArrowLeft move between the pages (wrapping), Home and End go to the
 * first and last, Enter follows the link. Leaving puts the Tab stop back on the current page. The
 * page's own tabs (`?tab=`) below keep their underlined look, so both levels stay apart.
 */
export function AreaTabs({
  label,
  tabs,
  // A name ending in Ref tells the React compiler that writing `.current` is fine.
  focusCarry: focusCarryRef,
}: AreaTabsProps) {
  const current = Math.max(
    0,
    tabs.findIndex((tab) => tab.current),
  );
  const [focused, setFocused] = useState<number | null>(null);
  const refs = useRef<(HTMLAnchorElement | null)[]>([]);
  const navRef = useRef<HTMLElement>(null);
  const tabStop = focused ?? current;

  useLayoutEffect(() => {
    const node = navRef.current;
    if (!focusCarryRef || !node) return;
    const carried = focusCarryRef.current;
    focusCarryRef.current = null;
    // Only when the focus went away with the previous page (it is on the body now).
    const active = document.activeElement;
    if (carried !== null && (active === null || active === document.body)) {
      refs.current[carried]?.focus();
    }
    return () => {
      // Layout cleanups run before React removes the page, so the tab still has the focus here.
      const index = [...node.querySelectorAll('a')].findIndex(
        (link) => link === document.activeElement,
      );
      if (index >= 0) focusCarryRef.current = index;
    };
  }, [focusCarryRef]);

  const move = (to: number) => {
    const next = (to + tabs.length) % tabs.length;
    setFocused(next);
    refs.current[next]?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLAnchorElement>, index: number) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    if (event.key === 'ArrowRight') move(index + 1);
    else if (event.key === 'ArrowLeft') move(index - 1);
    else if (event.key === 'Home') move(0);
    else if (event.key === 'End') move(tabs.length - 1);
    else return;
    event.preventDefault();
  };

  const onBlur = (event: FocusEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget)) setFocused(null);
  };

  return (
    <nav ref={navRef} aria-label={label} className={styles.nav} onBlur={onBlur}>
      <ul className={styles.list}>
        {tabs.map((tab, index) => (
          <li key={tab.href}>
            <AppLink
              ref={(element) => {
                refs.current[index] = element;
              }}
              href={tab.href}
              className={styles.tab}
              tabIndex={index === tabStop ? 0 : -1}
              aria-label={tab.accessibleLabel}
              aria-current={tab.current ? 'page' : undefined}
              onKeyDown={(event) => onKeyDown(event, index)}
              onFocus={() => setFocused(index)}
            >
              {tab.label}
              {tab.badge && (
                <span className={styles.badge} aria-hidden="true">
                  {tab.badge}
                </span>
              )}
              {tab.alarm ? (
                <span className={`${styles.pill} ${styles[tab.alarm.tone]}`} aria-hidden="true">
                  {tab.alarm.value}
                </span>
              ) : (
                tab.count !== undefined && (
                  <span className={styles.count} aria-hidden="true">
                    {tab.count}
                  </span>
                )
              )}
            </AppLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
