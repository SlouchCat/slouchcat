import { lazy, Suspense, type ReactNode } from 'react';
import type { NavItem } from './AppHeader';
import { AppLink } from './AppLink';
import { Icon, type IconName } from './Icon';
import styles from './SideNav.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

// The side navigation of concept 08 (7.1): one content, two renderings. From 768 px a fixed column
// at the left of every page with the app header (SideNav); below 768 px, and wherever the viewer
// has hidden the column, a modal drawer behind the header's menu button (SideNavMenu). Since
// concept 17 (D-8, F-13) it follows the administration's sidebar: a card with the active profile
// on top, an icon before every entry, bands for «Meine Medien» (always open) and «Konto». There is
// still no narrow icon-only state: the column is hidden, not collapsed (N-2).
// The drawer needs react-aria; it is a chunk of its own, loaded on the first opening, so the start
// bundle stays without react-aria (P-10).

const SideNavDrawer = lazy(() =>
  import('./SideNavDrawer').then((module) => ({ default: module.SideNavDrawer })),
);

/** One entry of the side navigation. */
export interface SideNavEntry extends NavItem {
  /** Icon before the name (concept 17, D-8). */
  icon?: IconName;
  /** Number after the name, as a pill: the visible titles of a library, the user picks. */
  count?: number;
  /** Accessible name when it says more than `label` (the count, the update hint). */
  accessibleLabel?: string;
  /** Small badge after the name ("Update"); `accessibleLabel` names it. */
  badge?: string;
}

/** The card with the active profile at the top (concept 17, D-8 variant B). */
export interface SideNavProfile {
  /** The profile's name; its first letter is the avatar, on the accent of the session. */
  name: string;
  /** What the profile is ("Hauptprofil · Server-Admin"). */
  role: string;
  /** The profile picker. */
  href: string;
  /** Visible text of the link there ("Wechseln"). */
  switchLabel: string;
  /** Its accessible name, with the profile's name ("Profil wechseln, jetzt Marco"). */
  switchAccessibleLabel: string;
}

export interface SideNavProps {
  /** Name of the <nav> ("Hauptnavigation"), the same in the column and in the drawer. */
  label: string;
  profile: SideNavProfile;
  /** Start, search, watchlist, radio, downloads. */
  primary: readonly SideNavEntry[];
  /** Title of the libraries band ("Meine Medien"). */
  librariesLabel: string;
  /**
   * The libraries the active profile may see, from the server (rule 12); 'loading' until they
   * are there, 'offline' when they could not be read. Neither blocks the rest of the navigation.
   */
  libraries: readonly SideNavEntry[] | 'loading' | 'offline';
  /** The band's last entry: the user picks with their count. */
  picks?: SideNavEntry;
  /** Shown in the band instead of the libraries when they could not be read. */
  librariesOffline: string;
  /** The way to what works without the server (the downloads), next to that text. */
  offlineLink?: NavItem;
  /** Title of the account band ("Konto"). */
  accountLabel: string;
  /** Playback, profiles and administration (with the update badge). */
  account: readonly SideNavEntry[];
  logoutLabel: string;
  onLogout: () => void;
  /** "Seitenleiste ausblenden": only in the column. */
  hideLabel: string;
  onHide: () => void;
}

/** Focus key of an entry: the column and the drawer use prefixes of their own. */
const entryKey = (prefix: string, href: string) => `${prefix}-${href}`;

/** Focus key of the column's entry for `href`, for the focus when the column comes back. */
export const sideNavFocusKey = (href: string) => entryKey('sidenav', href);

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
      {entry.icon && <Icon name={entry.icon} size="sm" />}
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
}

function EntryButton({ focusKey, onPress, className, children }: EntryButtonProps) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  return (
    <button
      ref={ref}
      type="button"
      className={className ?? styles.entry}
      onFocus={onFocus}
      onClick={onPress}
    >
      {children}
    </button>
  );
}

function ProfileCard({ profile, focusKey }: { profile: SideNavProfile; focusKey: string }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <div className={styles.profile}>
      <span className={styles.avatar} aria-hidden="true">
        {profile.name.slice(0, 1).toUpperCase()}
      </span>
      <span className={styles.profileText}>
        <span className={styles.profileName}>{profile.name}</span>
        <span className={styles.profileRole}>{profile.role}</span>
      </span>
      <AppLink
        ref={ref}
        onFocus={onFocus}
        href={profile.href}
        className={styles.switch}
        aria-label={profile.switchAccessibleLabel}
      >
        {profile.switchLabel}
      </AppLink>
    </div>
  );
}

/** A group with its title band, as in the administration's sidebar; the band names the list. */
function Band({
  id,
  label,
  icon,
  children,
}: {
  id: string;
  label: string;
  icon: IconName;
  children: ReactNode;
}) {
  return (
    <div className={styles.group}>
      <span id={id} className={styles.band}>
        <Icon name={icon} size="sm" />
        {label}
      </span>
      <ul className={styles.list} aria-labelledby={id}>
        {children}
      </ul>
    </div>
  );
}

interface SideNavContentProps extends SideNavProps {
  /** Prefix of the focus keys and element ids: the column and the drawer differ. */
  prefix: string;
  /** The column's last entry ("Seitenleiste ausblenden") or the drawer's ("einblenden"). */
  last?: { label: string; focusKey: string; onPress: () => void };
}

const entryItem = (prefix: string, entry: SideNavEntry) => (
  <li key={entry.href}>
    <EntryLink entry={entry} focusKey={entryKey(prefix, entry.href)} />
  </li>
);

/**
 * The entries of both renderings, top to bottom (concept 17, D-8 variant B): the profile card;
 * start, search, watchlist, radio, downloads; the band «Meine Medien» with the libraries and the
 * user picks last; the band «Konto» with playback, profiles and administration; at the bottom
 * sign out and the entry that hides or shows the column.
 */
export function SideNavContent({
  prefix,
  profile,
  primary,
  librariesLabel,
  libraries,
  picks,
  librariesOffline,
  offlineLink,
  accountLabel,
  account,
  logoutLabel,
  onLogout,
  last,
}: SideNavContentProps) {
  return (
    <>
      <ProfileCard profile={profile} focusKey={`${prefix}-switch`} />
      <ul className={styles.list}>{primary.map((entry) => entryItem(prefix, entry))}</ul>
      <Band id={`${prefix}-libraries`} label={librariesLabel} icon="film">
        {libraries === 'offline' ? (
          <li className={styles.offline}>
            <span className={styles.offlineText}>{librariesOffline}</span>
            {offlineLink && <EntryLink entry={offlineLink} focusKey={`${prefix}-offline`} />}
          </li>
        ) : (
          <>
            {libraries !== 'loading' && libraries.map((entry) => entryItem(prefix, entry))}
            {picks && entryItem(prefix, picks)}
          </>
        )}
      </Band>
      <Band id={`${prefix}-account`} label={accountLabel} icon="settings">
        {account.map((entry) => entryItem(prefix, entry))}
      </Band>
      <ul className={`${styles.list} ${styles.end}`}>
        <li>
          <EntryButton focusKey={`${prefix}-logout`} onPress={onLogout}>
            <Icon name="logout" size="sm" />
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
function currentKey({ primary, libraries, picks, account }: SideNavProps): string | undefined {
  const entries = [
    ...primary,
    ...(typeof libraries === 'string' ? [] : libraries),
    ...(picks ? [picks] : []),
    ...account,
  ];
  const current = entries.find((entry) => entry.current);
  return current && sideNavFocusKey(current.href);
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
