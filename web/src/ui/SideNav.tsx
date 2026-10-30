import { lazy, Suspense, useState, type ReactNode } from 'react';
import type { NavItem } from './AppHeader';
import { AppLink } from './AppLink';
import styles from './SideNav.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

// The side navigation of concept 08 (7.1): one content, two renderings. From 768 px a fixed column
// at the left of every page with the app header (SideNav); below 768 px, and wherever the viewer
// has hidden the column, a modal drawer behind the header's menu button (SideNavMenu). No icons
// (N-2): without them there is no narrow state, so the column is hidden instead of collapsed.
// The drawer needs react-aria; it is a chunk of its own, loaded on the first opening, so the start
// bundle stays without react-aria (P-10).

const SideNavDrawer = lazy(() =>
  import('./SideNavDrawer').then((module) => ({ default: module.SideNavDrawer })),
);

/** One entry of the side navigation. */
export interface SideNavEntry extends NavItem {
  /** Number after the name: the visible titles of a library, Cat's recommendations. */
  count?: number;
  /** Accessible name when it says more than `label` (the count, the update hint). */
  accessibleLabel?: string;
  /** Small badge after the name ("Update"); `accessibleLabel` names it. */
  badge?: string;
}

export interface SideNavProps {
  /** Name of the <nav> ("Hauptnavigation"), the same in the column and in the drawer. */
  label: string;
  /** Start, search, watchlist, downloads. */
  primary: readonly NavItem[];
  /** Title of the libraries group ("Meine Medien"). */
  librariesLabel: string;
  /** Accessible name of the button that opens and closes the group. */
  toggleLibrariesLabel: string;
  /**
   * The libraries the active profile may see, from the server (rule 12); 'loading' until they
   * are there, 'offline' when they could not be read. Neither blocks the rest of the navigation.
   */
  libraries: readonly SideNavEntry[] | 'loading' | 'offline';
  /** The group's last entry: Cat's recommendations with their count. */
  picks?: SideNavEntry;
  /** Shown in the group instead of the libraries when they could not be read. */
  librariesOffline: string;
  /** The way to what works without the server (the downloads), next to that text. */
  offlineLink?: NavItem;
  /** Administration (with the update badge) and profiles. */
  footer: readonly SideNavEntry[];
  logoutLabel: string;
  onLogout: () => void;
  /** "Seitenleiste ausblenden": only in the column. */
  hideLabel: string;
  onHide: () => void;
}

/** localStorage key of the libraries group's state on this device ("closed" when collapsed). */
const LIBRARIES_KEY = 'slouchcat.sidenav.libraries';

function readLibrariesOpen(): boolean {
  try {
    return localStorage.getItem(LIBRARIES_KEY) !== 'closed';
  } catch {
    return true;
  }
}

function storeLibrariesOpen(open: boolean): void {
  try {
    if (open) localStorage.removeItem(LIBRARIES_KEY);
    else localStorage.setItem(LIBRARIES_KEY, 'closed');
  } catch {
    // Without storage the group keeps its state for this visit only.
  }
}

/** Focus key of an entry: the column and the drawer use prefixes of their own. */
const entryKey = (prefix: string, href: string) => `${prefix}-${href}`;

/** Focus key of the column's "Seitenleiste ausblenden", for the focus after showing the column. */
export const SIDENAV_HIDE_KEY = 'sidenav-hide';

function EntryLink({ entry, focusKey }: { entry: SideNavEntry; focusKey: string }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink
      ref={ref}
      onFocus={onFocus}
      href={entry.href}
      className={styles.entry}
      aria-label={entry.accessibleLabel}
      aria-current={entry.current ? 'page' : undefined}
    >
      <span className={styles.label}>{entry.label}</span>
      {entry.badge && (
        <span className={styles.badge} aria-hidden="true">
          {entry.badge}
        </span>
      )}
      {entry.count !== undefined && (
        <span className={styles.count} aria-hidden="true">
          {entry.count}
        </span>
      )}
    </AppLink>
  );
}

interface EntryButtonProps {
  focusKey: string;
  onPress: () => void;
  className?: string;
  children: ReactNode;
  'aria-label'?: string;
  'aria-expanded'?: boolean;
  'aria-controls'?: string;
}

function EntryButton({ focusKey, onPress, className, children, ...aria }: EntryButtonProps) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  return (
    <button
      ref={ref}
      type="button"
      className={className ?? styles.entry}
      onFocus={onFocus}
      onClick={onPress}
      {...aria}
    >
      {children}
    </button>
  );
}

interface SideNavContentProps extends SideNavProps {
  /** Prefix of the focus keys and element ids: the column and the drawer differ. */
  prefix: string;
  /** The column's last entry ("Seitenleiste ausblenden") or the drawer's ("einblenden"). */
  last?: { label: string; focusKey: string; onPress: () => void };
}

/**
 * The entries of both renderings, top to bottom (7.1): start, search, watchlist, downloads; the
 * libraries group with Cat's recommendations last; administration, profiles, sign out and the
 * entry that hides or shows the column.
 */
export function SideNavContent({
  prefix,
  primary,
  librariesLabel,
  toggleLibrariesLabel,
  libraries,
  picks,
  librariesOffline,
  offlineLink,
  footer,
  logoutLabel,
  onLogout,
  last,
}: SideNavContentProps) {
  const [open, setOpen] = useState(readLibrariesOpen);
  const listId = `${prefix}-libraries`;
  return (
    <>
      <ul className={styles.list}>
        {primary.map((entry) => (
          <li key={entry.href}>
            <EntryLink entry={entry} focusKey={entryKey(prefix, entry.href)} />
          </li>
        ))}
      </ul>
      <div className={styles.group}>
        <EntryButton
          focusKey={`${prefix}-toggle`}
          className={`${styles.entry} ${styles.groupTitle}`}
          aria-label={toggleLibrariesLabel}
          aria-expanded={open}
          aria-controls={listId}
          onPress={() => {
            storeLibrariesOpen(!open);
            setOpen(!open);
          }}
        >
          <span>{librariesLabel}</span>
          <span className={styles.chevron} data-open={open || undefined} aria-hidden="true" />
        </EntryButton>
        <ul id={listId} className={styles.list} hidden={!open}>
          {open &&
            (libraries === 'offline' ? (
              <li className={styles.offline}>
                <span className={styles.offlineText}>{librariesOffline}</span>
                {offlineLink && <EntryLink entry={offlineLink} focusKey={`${prefix}-offline`} />}
              </li>
            ) : (
              <>
                {libraries !== 'loading' &&
                  libraries.map((entry) => (
                    <li key={entry.href}>
                      <EntryLink entry={entry} focusKey={entryKey(prefix, entry.href)} />
                    </li>
                  ))}
                {picks && (
                  <li>
                    <EntryLink entry={picks} focusKey={entryKey(prefix, picks.href)} />
                  </li>
                )}
              </>
            ))}
        </ul>
      </div>
      <ul className={`${styles.list} ${styles.footer}`}>
        {footer.map((entry) => (
          <li key={entry.href}>
            <EntryLink entry={entry} focusKey={entryKey(prefix, entry.href)} />
          </li>
        ))}
        <li>
          <EntryButton focusKey={`${prefix}-logout`} onPress={onLogout}>
            <span className={styles.label}>{logoutLabel}</span>
          </EntryButton>
        </li>
        {last && (
          <li>
            <EntryButton
              focusKey={last.focusKey}
              className={`${styles.entry} ${styles.quiet}`}
              onPress={last.onPress}
            >
              <span className={styles.label}>{last.label}</span>
            </EntryButton>
          </li>
        )}
      </ul>
    </>
  );
}

/** Focus key of the entry that names the current page, if one does. */
function currentKey({ primary, libraries, picks, footer }: SideNavProps): string | undefined {
  const entries = [
    ...primary,
    ...(typeof libraries === 'string' ? [] : libraries),
    ...(picks ? [picks] : []),
    ...footer,
  ];
  const current = entries.find((entry) => entry.current);
  return current && entryKey('sidenav', current.href);
}

/**
 * The column (from 768 px, not hidden): fixed at the left below the header, scrolling on its own.
 * Arrow left from the content lands on the current page's entry (AppLayout routes it here).
 */
export function SideNav(props: SideNavProps) {
  return (
    <SpatialGroup
      as="nav"
      aria-label={props.label}
      focusKey="sidenav"
      className={styles.bar}
      tabIndex={-1}
      rememberLastChild={false}
      preferredChildFocusKey={currentKey(props)}
    >
      <SideNavContent
        {...props}
        prefix="sidenav"
        last={{ label: props.hideLabel, focusKey: SIDENAV_HIDE_KEY, onPress: props.onHide }}
      />
    </SpatialGroup>
  );
}

export interface SideNavMenuProps extends SideNavProps {
  isOpen: boolean;
  onClose: () => void;
  /** Id of the drawer, named by the menu button's `aria-controls`. */
  id: string;
  /** Accessible name of the drawer ("Menü") and of its close button ("Menü schliessen"). */
  menuLabel: string;
  closeLabel: string;
  /** "Seitenleiste einblenden": only where the viewer has hidden the column (from 768 px). */
  show?: { label: string; onPress: () => void };
}

/** The drawer, loaded with react-aria on its first opening; nothing while it is closed. */
export function SideNavMenu(props: SideNavMenuProps) {
  if (!props.isOpen) return null;
  return (
    <Suspense fallback={null}>
      <SideNavDrawer {...props} />
    </Suspense>
  );
}
