import {
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type KeyboardEvent,
  type RefObject,
} from 'react';
import styles from './AdminNav.module.css';
import { AppLink } from './AppLink';

export interface AdminNavLink {
  href: string;
  label: string;
  current?: boolean;
}

export interface AdminNavGroup {
  /** Shown before the group's links; a group without one (the overview) shows its link only. */
  label?: string;
  links: readonly AdminNavLink[];
}

interface AdminNavProps {
  /** Accessible name of the navigation ("Verwaltung"). */
  label: string;
  groups: readonly AdminNavGroup[];
  /**
   * Every page renders its own navigation (PageNav), so following a link replaces it and its
   * focused link with it. A ref that outlives the pages (the layout's) carries the focused link
   * over: the next page's navigation focuses the same link, as if the row had stayed.
   */
  focusCarry?: RefObject<number | null>;
}

/**
 * Section navigation of the administration (A-2): every admin page as one row of links in groups.
 * Like Tabs it is a single Tab stop (the current page's link, roving tabIndex): ArrowLeft and
 * ArrowRight move between the links (wrapping), Home and End go to the first and last, Enter
 * follows the link. Leaving the row puts the Tab stop back on the current page; following a link
 * keeps the focus on it (`focusCarry`).
 */
export function AdminNav({ label, groups, focusCarry }: AdminNavProps) {
  const id = useId();
  const links = groups.flatMap((group) => group.links);
  const offsets = groups.map((_, index) =>
    groups.slice(0, index).reduce((sum, group) => sum + group.links.length, 0),
  );
  const current = Math.max(
    0,
    links.findIndex((link) => link.current),
  );
  const [focused, setFocused] = useState<number | null>(null);
  const refs = useRef<(HTMLAnchorElement | null)[]>([]);
  const navRef = useRef<HTMLElement>(null);
  const tabStop = focused ?? current;

  useLayoutEffect(() => {
    const node = navRef.current;
    if (!focusCarry || !node) return;
    const carried = focusCarry.current;
    focusCarry.current = null;
    // Only when the focus went away with the previous page (it is on the body now).
    const active = document.activeElement;
    if (carried !== null && (active === null || active === document.body)) {
      refs.current[carried]?.focus();
    }
    return () => {
      // Layout cleanups run before React removes the page, so the link still has the focus here.
      const index = [...node.querySelectorAll('a')].findIndex(
        (link) => link === document.activeElement,
      );
      if (index >= 0) focusCarry.current = index;
    };
  }, [focusCarry]);

  const move = (to: number) => {
    const next = (to + links.length) % links.length;
    setFocused(next);
    refs.current[next]?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLAnchorElement>, index: number) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    if (event.key === 'ArrowRight') move(index + 1);
    else if (event.key === 'ArrowLeft') move(index - 1);
    else if (event.key === 'Home') move(0);
    else if (event.key === 'End') move(links.length - 1);
    else return;
    event.preventDefault();
  };

  const onBlur = (event: FocusEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget)) setFocused(null);
  };

  return (
    <nav ref={navRef} aria-label={label} className={styles.nav} onBlur={onBlur}>
      <ul className={styles.groups}>
        {groups.map((group, groupIndex) => {
          const labelId = `${id}-${groupIndex}`;
          return (
            <li key={group.label ?? groupIndex} className={styles.group}>
              {group.label && (
                <span id={labelId} className={styles.groupLabel}>
                  {group.label}
                </span>
              )}
              <ul className={styles.links} aria-labelledby={group.label ? labelId : undefined}>
                {group.links.map((link, linkIndex) => {
                  const index = (offsets[groupIndex] ?? 0) + linkIndex;
                  return (
                    <li key={link.href}>
                      <AppLink
                        ref={(element) => {
                          refs.current[index] = element;
                        }}
                        href={link.href}
                        className={styles.link}
                        tabIndex={index === tabStop ? 0 : -1}
                        aria-current={link.current ? 'page' : undefined}
                        onKeyDown={(event) => onKeyDown(event, index)}
                        onFocus={() => setFocused(index)}
                      >
                        {link.label}
                      </AppLink>
                    </li>
                  );
                })}
              </ul>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
