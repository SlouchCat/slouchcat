import {
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type KeyboardEvent,
  type RefObject,
} from 'react';
import styles from './AdminSideNav.module.css';
import { AppLink } from './AppLink';
import { Icon, type IconName } from './Icon';

/** Colour of an alarm pill (F-7): open cases, running streams, failed jobs, silent webhooks. */
export type AdminSideNavTone = 'warning' | 'accent' | 'danger';

export interface AdminSideNavEntry {
  href: string;
  label: string;
  icon: IconName;
  current?: boolean;
  /** Muted number at the end of the entry ("2.318", "3 / 8"), formatted by the page. */
  count?: string;
  /** Coloured pill at the end instead of the count. */
  alarm?: { value: string; tone: AdminSideNavTone };
  /** Small badge after the name ("Update"); `accessibleLabel` names it. */
  badge?: string;
  /**
   * Accessible name when the entry says more than `label`: the spelled-out number, pill or badge
   * ("Pflege, 12 Titel brauchen Pflege"). On screen the number stays short and is hidden from
   * screen readers.
   */
  accessibleLabel?: string;
}

export interface AdminSideNavGroup {
  /** Shown in the band above the group's entries ("Bibliothek"). */
  label: string;
  icon: IconName;
  entries: readonly AdminSideNavEntry[];
}

export interface AdminSideNavProps {
  /** Accessible name of the navigation ("Verwaltung"). */
  label: string;
  /** The entries above the groups: back to the app and the overview. */
  top: readonly AdminSideNavEntry[];
  groups: readonly AdminSideNavGroup[];
  /**
   * When a page renders its own copy of the navigation, following a link replaces it and its
   * focused entry with it. A ref that outlives the pages carries the focused entry over: the next
   * page's navigation focuses the same entry, as if the column had stayed.
   */
  focusCarry?: RefObject<number | null>;
}

interface AdminSideNavContentProps extends AdminSideNavProps {
  /** The rendering's own layout: the column or the sheet's list. */
  className?: string;
}

function EntryContent({ entry }: { entry: AdminSideNavEntry }) {
  return (
    <>
      <Icon name={entry.icon} size="sm" />
      <span className={styles.label}>{entry.label}</span>
      {entry.badge && (
        <span className={styles.badge} aria-hidden="true">
          {entry.badge}
        </span>
      )}
      {entry.alarm ? (
        <span className={`${styles.pill} ${styles[entry.alarm.tone]}`} aria-hidden="true">
          {entry.alarm.value}
        </span>
      ) : (
        entry.count !== undefined && (
          <span className={styles.count} aria-hidden="true">
            {entry.count}
          </span>
        )
      )}
    </>
  );
}

/**
 * The entries of both renderings, the column (AdminSideNav) and the sheet (AdminSideNavMenu):
 * back to the app, the overview, then the four groups with a title band each and their pages.
 *
 * Like Tabs it is a single Tab stop (the current page's entry, roving tabIndex) across all
 * entries: ArrowDown and ArrowUp move between them (wrapping), Home and End go to the first and
 * last, Enter follows the link, Tab leaves the column. Leaving puts the Tab stop back on the
 * current page. The group titles are no headings, so no four headings stand before the page's h1.
 */
export function AdminSideNavContent({
  label,
  top,
  groups,
  // A name ending in Ref tells the React compiler that writing `.current` is fine.
  focusCarry: focusCarryRef,
  className,
}: AdminSideNavContentProps) {
  const id = useId();
  const entries = [...top, ...groups.flatMap((group) => group.entries)];
  const offsets = groups.map(
    (_, index) =>
      top.length + groups.slice(0, index).reduce((sum, group) => sum + group.entries.length, 0),
  );
  const current = Math.max(
    0,
    entries.findIndex((entry) => entry.current),
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
      // Layout cleanups run before React removes the page, so the entry still has the focus here.
      const index = [...node.querySelectorAll('a')].findIndex(
        (link) => link === document.activeElement,
      );
      if (index >= 0) focusCarryRef.current = index;
    };
  }, [focusCarryRef]);

  const move = (to: number) => {
    const next = (to + entries.length) % entries.length;
    setFocused(next);
    refs.current[next]?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLAnchorElement>, index: number) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    if (event.key === 'ArrowDown') move(index + 1);
    else if (event.key === 'ArrowUp') move(index - 1);
    else if (event.key === 'Home') move(0);
    else if (event.key === 'End') move(entries.length - 1);
    else return;
    event.preventDefault();
  };

  const onBlur = (event: FocusEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget)) setFocused(null);
  };

  const link = (entry: AdminSideNavEntry, index: number, entryClass?: string) => (
    <li key={entry.href}>
      <AppLink
        ref={(element) => {
          refs.current[index] = element;
        }}
        href={entry.href}
        className={entryClass}
        tabIndex={index === tabStop ? 0 : -1}
        aria-label={entry.accessibleLabel}
        aria-current={entry.current ? 'page' : undefined}
        onKeyDown={(event) => onKeyDown(event, index)}
        onFocus={() => setFocused(index)}
      >
        <EntryContent entry={entry} />
      </AppLink>
    </li>
  );

  return (
    <nav ref={navRef} aria-label={label} className={className} onBlur={onBlur}>
      <ul className={styles.list}>{top.map((entry, index) => link(entry, index, styles.entry))}</ul>
      {groups.map((group, groupIndex) => {
        const bandId = `${id}-${groupIndex}`;
        return (
          <div key={group.label} className={styles.group}>
            <span id={bandId} className={styles.band}>
              <Icon name={group.icon} size="sm" />
              {group.label}
            </span>
            <ul className={styles.list} aria-labelledby={bandId}>
              {group.entries.map((entry, entryIndex) =>
                link(
                  entry,
                  (offsets[groupIndex] ?? 0) + entryIndex,
                  `${styles.entry} ${styles.indented}`,
                ),
              )}
            </ul>
          </div>
        );
      })}
    </nav>
  );
}

/**
 * The administration's sidebar (concept 11, 11.1 and 11.6) at the place and in the width of the
 * app's side navigation. Presentational only: the entries, their numbers and pills come in as
 * props.
 */
export function AdminSideNav(props: AdminSideNavProps) {
  return <AdminSideNavContent {...props} className={`${styles.nav} ${styles.column}`} />;
}
