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

interface AdminSideNavLink {
  href: string;
  label: string;
  icon: IconName;
  current?: boolean;
}

/**
 * What an entry or a branch shows at its end. On screen a number, pill or badge stays short and is
 * hidden from screen readers, so `accessibleLabel` is required as soon as one of the three is set.
 */
type AdminSideNavFigure =
  | {
      count?: undefined;
      alarm?: undefined;
      badge?: undefined;
      /** Accessible name when it says more than `label`. */
      accessibleLabel?: string;
    }
  | {
      /** Muted number at the end of the entry ("2.318", "3 / 8"), formatted by the page. */
      count?: string;
      /** Coloured pill at the end instead of the count. */
      alarm?: { value: string; tone: AdminSideNavTone };
      /** Small badge after the name ("Update"). */
      badge?: string;
      /**
       * The spoken name with the spelled-out number, pill or badge ("Pflege, 12 Titel brauchen
       * Pflege").
       */
      accessibleLabel: string;
    };

/** One entry of the sidebar: a link, with its number, pill or badge (`AdminSideNavFigure`). */
export type AdminSideNavEntry = AdminSideNavLink & AdminSideNavFigure;

export interface AdminSideNavGroup {
  /** Shown in the band above the group's entries ("Bibliothek"). */
  label: string;
  icon: IconName;
  entries: readonly AdminSideNavEntry[];
}

/**
 * An area of the column's tree as the frame builds it, without its open state: a button that
 * folds its pages in and out, with the area's pill or badge (`AdminSideNavFigure`).
 */
export type AdminSideNavBranchData = AdminSideNavGroup & {
  /** Handed back by `onToggleBranch`. */
  id: string;
  /** The current page is one of its entries: the button is marked, open or closed. */
  current?: boolean;
} & AdminSideNavFigure;

/** A branch of the column's tree with its open state. */
export type AdminSideNavBranch = AdminSideNavBranchData & {
  /** Its pages are shown; closed they stay in the DOM, `hidden`. */
  expanded: boolean;
};

/** The active profile at the foot of the column: shown, not a link (the header has the picker). */
export interface AdminSideNavProfile {
  name: string;
  /** Its role ("Server-Admin"). */
  role: string;
}

export interface AdminSideNavProps {
  /** Accessible name of the navigation ("Verwaltung"). */
  label: string;
  /** The entries at the top: back to the app (and, in the sheet, the overview). */
  top: readonly AdminSideNavEntry[];
  /**
   * The small heading above `entries` ("Verwaltung"), in small capitals. It names their list; it
   * is no heading element, so none stands before the page's h1.
   */
  heading?: string;
  /** The entries below the heading: in the column the overview. */
  entries?: readonly AdminSideNavEntry[];
  /** After `entries`, in the same list: in the column the areas as a tree with their pages. */
  branches?: readonly AdminSideNavBranch[];
  /** A branch's button was pressed: open or close it (`AdminSideNavBranch.id`). */
  onToggleBranch?: (id: string) => void;
  /** Entries in groups, each under its band: in the sheet the areas with all their pages. */
  groups?: readonly AdminSideNavGroup[];
  /** The active profile at the foot of the column (the sheet has none). */
  profile?: AdminSideNavProfile;
  /**
   * When a page renders its own copy of the navigation, following a link replaces it and its
   * focused entry with it. A ref that outlives the pages carries the focused entry over: the next
   * page's navigation focuses the same entry, as if the column had stayed.
   */
  focusCarry?: RefObject<number | null>;
}

interface AdminSideNavContentProps extends AdminSideNavProps {
  /** Extra layout of the rendering: the column's; the sheet adds none. */
  className?: string;
  /** Scrolls the current page's entry into view (the column, which scrolls on its own). */
  revealCurrent?: boolean;
}

type Figured = AdminSideNavFigure & { icon: IconName; label: string };

function EntryContent({ entry, chevron }: { entry: Figured; chevron: boolean }) {
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
      {chevron && (
        <span className={styles.chevron} aria-hidden="true">
          <Icon name="chevronRight" size="sm" />
        </span>
      )}
    </>
  );
}

/** A focusable item of the navigation in DOM order: a link, or a branch's button. */
interface Item {
  /** The entry of a link; none for a branch's button. */
  entry?: AdminSideNavEntry;
  /** Shown: not in a closed branch. */
  visible: boolean;
  /** For a link in a branch: the index of the branch's button. */
  owner?: number;
}

/**
 * The entries of both renderings, the column (AdminSideNav) and the sheet (AdminSideNavMenu):
 * back to the app at the top, then in the column the heading «Verwaltung» with the overview and
 * the five areas as a tree (each a button that folds its pages in and out), in the sheet the
 * overview and the areas as bands with all their pages.
 *
 * Like Tabs it is a single Tab stop (the current page's entry, roving tabIndex) across all shown
 * items, links and the branches' buttons: ArrowDown and ArrowUp move between them (wrapping),
 * Home and End go to the first and last, Enter follows the link or, like Space, opens or closes a
 * branch, Tab leaves the column. Leaving puts the Tab stop back on the current page, or on its
 * branch's button while the branch is closed. The heading and the group titles are no headings,
 * so none stands before the page's h1.
 */
export function AdminSideNavContent({
  label,
  top,
  heading,
  entries: main = [],
  branches = [],
  onToggleBranch,
  groups = [],
  // A name ending in Ref tells the React compiler that writing `.current` is fine.
  focusCarry: focusCarryRef,
  className,
  revealCurrent = false,
}: AdminSideNavContentProps) {
  const id = useId();
  const items: Item[] = [...top, ...main].map((entry) => ({ entry, visible: true }));
  const branchIndex: number[] = [];
  for (const branch of branches) {
    const owner = items.length;
    branchIndex.push(owner);
    items.push({ visible: true });
    for (const entry of branch.entries) items.push({ entry, visible: branch.expanded, owner });
  }
  const offsets: number[] = [];
  for (const group of groups) {
    offsets.push(items.length);
    for (const entry of group.entries) items.push({ entry, visible: true });
  }
  // The current-page arrow would read as a branch's disclosure arrow in the tree.
  const chevron = branches.length === 0;

  /** The item itself when it is shown, else its branch's button. */
  const shown = (index: number) => {
    const item = items[index];
    return item && !item.visible && item.owner !== undefined ? item.owner : index;
  };
  const currentIndex = items.findIndex((item) => item.entry?.current);
  const current = currentIndex >= 0 ? shown(currentIndex) : 0;
  const [focused, setFocused] = useState<number | null>(null);
  const refs = useRef<(HTMLElement | null)[]>([]);
  const navRef = useRef<HTMLElement>(null);
  const tabStop = focused === null ? current : shown(focused);
  const revealed = currentIndex >= 0 && items[currentIndex]?.visible ? currentIndex : -1;

  useLayoutEffect(() => {
    const node = navRef.current;
    if (!focusCarryRef || !node) return;
    const carried = focusCarryRef.current;
    focusCarryRef.current = null;
    // Only when the focus went away with the previous page (it is on the body now).
    const active = document.activeElement;
    if (carried !== null && (active === null || active === document.body)) {
      // A link in a closed branch is hidden: its branch's button takes the focus.
      const target = refs.current[carried];
      const owner = target?.closest('ul[hidden]')?.previousElementSibling;
      (owner instanceof HTMLElement ? owner : target)?.focus();
    }
    return () => {
      // Layout cleanups run before React removes the page, so the entry still has the focus here.
      // The index counts every item in DOM order, links and branches' buttons, hidden ones too.
      const index = [...node.querySelectorAll('a, button')].findIndex(
        (element) => element === document.activeElement,
      );
      if (index >= 0) focusCarryRef.current = index;
    };
  }, [focusCarryRef]);

  // On mount and whenever the page changes: the current entry may sit below the column's fold.
  useLayoutEffect(() => {
    if (revealCurrent && revealed >= 0) {
      refs.current[revealed]?.scrollIntoView({ block: 'nearest' });
    }
  }, [revealCurrent, revealed]);

  const visible = items.flatMap((item, index) => (item.visible ? [index] : []));
  const move = (from: number, step: number | 'first' | 'last') => {
    const position = visible.indexOf(shown(from));
    const next =
      step === 'first'
        ? visible[0]
        : step === 'last'
          ? visible[visible.length - 1]
          : visible[(position + step + visible.length) % visible.length];
    if (next === undefined) return;
    setFocused(next);
    refs.current[next]?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>, index: number) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    if (event.key === 'ArrowDown') move(index, 1);
    else if (event.key === 'ArrowUp') move(index, -1);
    else if (event.key === 'Home') move(index, 'first');
    else if (event.key === 'End') move(index, 'last');
    else return;
    event.preventDefault();
  };

  const onBlur = (event: FocusEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget)) setFocused(null);
  };

  const setRef = (index: number) => (element: HTMLElement | null) => {
    refs.current[index] = element;
  };

  const link = (entry: AdminSideNavEntry, index: number, entryClass?: string) => (
    <li key={entry.href}>
      <AppLink
        ref={setRef(index)}
        href={entry.href}
        className={entryClass}
        tabIndex={index === tabStop ? 0 : -1}
        aria-label={entry.accessibleLabel}
        aria-current={entry.current ? 'page' : undefined}
        onKeyDown={(event) => onKeyDown(event, index)}
        onFocus={() => setFocused(index)}
      >
        <EntryContent entry={entry} chevron={chevron && entry.current === true} />
      </AppLink>
    </li>
  );

  const branch = (item: AdminSideNavBranch, branchNumber: number) => {
    const index = branchIndex[branchNumber] ?? 0;
    const listId = `${id}-branch-${branchNumber}`;
    return (
      <li key={item.id} className={styles.branch}>
        <button
          ref={setRef(index)}
          type="button"
          className={`${styles.entry} ${styles.toggle}`}
          tabIndex={index === tabStop ? 0 : -1}
          aria-expanded={item.expanded}
          aria-controls={listId}
          aria-current={item.current ? 'true' : undefined}
          aria-label={item.accessibleLabel}
          onClick={() => onToggleBranch?.(item.id)}
          onKeyDown={(event) => onKeyDown(event, index)}
          onFocus={() => setFocused(index)}
        >
          <EntryContent entry={item} chevron={false} />
          <span className={styles.disclosure} aria-hidden="true">
            <Icon name="chevronRight" size="sm" />
          </span>
        </button>
        <ul
          id={listId}
          className={`${styles.list} ${styles.children}`}
          aria-label={item.label}
          hidden={!item.expanded}
        >
          {item.entries.map((entry, entryIndex) =>
            link(entry, index + 1 + entryIndex, `${styles.entry} ${styles.child}`),
          )}
        </ul>
      </li>
    );
  };

  const headingId = `${id}-heading`;
  return (
    <nav
      ref={navRef}
      aria-label={label}
      className={className ? `${styles.nav} ${className}` : styles.nav}
      onBlur={onBlur}
    >
      <ul className={styles.list}>{top.map((entry, index) => link(entry, index, styles.entry))}</ul>
      {(main.length > 0 || branches.length > 0) && (
        <div className={styles.section}>
          {heading && (
            <span id={headingId} className={styles.heading}>
              {heading}
            </span>
          )}
          <ul className={styles.list} aria-labelledby={heading ? headingId : undefined}>
            {main.map((entry, index) => link(entry, top.length + index, styles.entry))}
            {branches.map(branch)}
          </ul>
        </div>
      )}
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
 * The administration's sidebar (concept 11, 11.1 and 11.6; redesign, build plan 2.2) at the place
 * and in the width of the app's side navigation. Presentational only: the entries, their numbers
 * and pills come in as props.
 */
export function AdminSideNav({ profile, ...props }: AdminSideNavProps) {
  return (
    // With an area open the column can be higher than the window and scrolls; Firefox would make
    // the scrolling box a Tab stop of its own before the entries (as SideNav's bar, tabIndex -1).
    <div className={styles.column} tabIndex={-1}>
      <AdminSideNavContent {...props} revealCurrent />
      {profile && (
        <div className={styles.profile}>
          <span className={styles.avatar} aria-hidden="true">
            {profile.name.slice(0, 1).toUpperCase()}
          </span>
          <span className={styles.who}>
            <span className={styles.name}>{profile.name}</span>
            <span className={styles.role}>{profile.role}</span>
          </span>
        </div>
      )}
    </div>
  );
}
