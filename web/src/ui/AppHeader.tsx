import { useEffect, useState } from 'react';
import { AppLink } from './AppLink';
import styles from './AppHeader.module.css';
import { SpatialButton } from './Button';
import { Icon } from './Icon';
import { Logo } from './Logo';
import { NotificationBadge } from './NotificationBadge';
import { SpatialGroup, useSpatialItem } from './spatial';

export interface NavItem {
  href: string;
  label: string;
  current?: boolean;
}

/** The inbox link next to the profile (M7): visible label, unread count and its spoken label. */
export interface HeaderNotifications {
  href: string;
  label: string;
  /** Accessible name with the count ("Benachrichtigungen, 3 ungelesen"). */
  accessibleLabel: string;
  unread: number;
  /** The badge above its highest count ("99+"). */
  overflowLabel: string;
  current?: boolean;
}

/**
 * The menu button at the start of the header (concept 08): it opens the side navigation as a
 * drawer, below 768 px and wherever the viewer has hidden the column.
 */
export interface HeaderMenu {
  /** Accessible name ("Menü"). */
  label: string;
  /** Whether the drawer is open (`aria-expanded`). */
  expanded: boolean;
  /** Id of the drawer (`aria-controls`). */
  controls: string;
  onPress: () => void;
}

/** The search link: it stays in the header next to the inbox (concept 08), like the "/" key. */
export interface HeaderSearch {
  href: string;
  /** Visible text ("Suche"), hidden on phones where the icon stands alone. */
  label: string;
  /** Accessible name ("Suche öffnen"). */
  accessibleLabel: string;
  current?: boolean;
}

/** One step of the administration's breadcrumb; the last one is the current page. */
export interface HeaderCrumb {
  href: string;
  label: string;
}

/**
 * What the header shows inside the administration (build plan 2.1): a line under the logo, the
 * breadcrumb from 768 px and «Bereich suchen» in place of the search link.
 */
export interface HeaderAdmin {
  /** Under the logo's name ("Control Center"). */
  tagline: string;
  /** Accessible name of the breadcrumb ("Pfad"). */
  breadcrumbLabel: string;
  /**
   * «Verwaltung», the area and the page; the last is the current page and no link. The
   * separators: an arrow after the first step, a slash between area and page.
   */
  crumbs: readonly HeaderCrumb[];
  finder: HeaderFinder;
}

/** The button that opens the administration's search palette (build plan 2.4). */
export interface HeaderFinder {
  /** Visible text and accessible name ("Bereich suchen"); on phones the icon stands alone. */
  label: string;
  /** The shortcut as shown ("Strg K", "⌘ K"). */
  shortcut: string;
  /** The shortcut for assistive technology (`aria-keyshortcuts`, "Control+K"). */
  keyShortcuts: string;
  /** Whether the palette is open (`aria-expanded`). */
  expanded: boolean;
  onPress: () => void;
}

interface AppHeaderProps {
  appName: string;
  homeHref: string;
  menu?: HeaderMenu;
  search: HeaderSearch;
  /** Inside the administration: tagline, breadcrumb, and the finder instead of `search`. */
  admin?: HeaderAdmin;
  profileName: string;
  /** The profile picker: avatar and name link there to switch profiles. */
  profileHref: string;
  /** Accessible name of that link ("Switch profile, now Marco"). */
  profileLabel: string;
  notifications?: HeaderNotifications;
}

/** Focus key of the menu button: the focus returns to it after the column was hidden. */
export const HEADER_MENU_KEY = 'header-menu';

function SearchLink({ search }: { search: HeaderSearch }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>();
  return (
    <AppLink
      ref={ref}
      onFocus={onFocus}
      href={search.href}
      className={styles.inbox}
      aria-label={search.accessibleLabel}
      aria-keyshortcuts="/"
      aria-current={search.current ? 'page' : undefined}
    >
      <Icon name="search" size="md" />
      <span className={styles.text} aria-hidden="true">
        {search.label}
      </span>
    </AppLink>
  );
}

function FinderButton({ finder }: { finder: HeaderFinder }) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>();
  return (
    <button
      ref={ref}
      type="button"
      onFocus={onFocus}
      className={styles.finder}
      aria-label={finder.label}
      aria-keyshortcuts={finder.keyShortcuts}
      aria-haspopup="dialog"
      aria-expanded={finder.expanded}
      onClick={finder.onPress}
    >
      <Icon name="search" size="sm" />
      <span className={styles.text} aria-hidden="true">
        {finder.label}
      </span>
      <kbd className={styles.kbd} aria-hidden="true">
        {finder.shortcut}
      </kbd>
    </button>
  );
}

function CrumbLink({ crumb }: { crumb: HeaderCrumb }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>();
  return (
    <AppLink ref={ref} onFocus={onFocus} href={crumb.href} className={styles.crumbLink}>
      {crumb.label}
    </AppLink>
  );
}

/** «Verwaltung › Medien / Bibliotheken» (build plan 2.1), from 768 px. */
function Breadcrumb({ label, crumbs }: { label: string; crumbs: readonly HeaderCrumb[] }) {
  return (
    <nav aria-label={label} className={styles.breadcrumb}>
      <ol className={styles.crumbs}>
        {crumbs.map((crumb, index) => {
          const last = index === crumbs.length - 1;
          return (
            <li key={index} className={styles.crumb}>
              {index === 1 && <Icon name="chevronRight" size="sm" />}
              {index > 1 && (
                <span className={styles.slash} aria-hidden="true">
                  /
                </span>
              )}
              {last ? (
                <span className={styles.crumbCurrent} aria-current="page">
                  {crumb.label}
                </span>
              ) : (
                <CrumbLink crumb={crumb} />
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

function InboxLink({ inbox }: { inbox: HeaderNotifications }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>();
  return (
    <AppLink
      ref={ref}
      onFocus={onFocus}
      href={inbox.href}
      className={styles.inbox}
      aria-label={inbox.accessibleLabel}
      aria-current={inbox.current ? 'page' : undefined}
    >
      <Icon name="bell" size="md" />
      <span className={styles.text} aria-hidden="true">
        {inbox.label}
      </span>
      <NotificationBadge count={inbox.unread} overflowLabel={inbox.overflowLabel} />
    </AppLink>
  );
}

/**
 * Fixed top bar shared by viewer and admin pages. Transparent over artwork, solid once the page
 * scrolls. Takes part in spatial navigation (arrow up from the first row lands here). The page
 * navigation, sign out and the update hint live in the side navigation (SideNav, concept 08).
 */
export function AppHeader({
  appName,
  homeHref,
  menu,
  search,
  admin,
  profileName,
  profileHref,
  profileLabel,
  notifications,
}: AppHeaderProps) {
  const [scrolled, setScrolled] = useState(false);
  const { ref: homeRef, onFocus: onHomeFocus } = useSpatialItem<HTMLAnchorElement>();
  const { ref: profileRef, onFocus: onProfileFocus } = useSpatialItem<HTMLAnchorElement>();

  useEffect(() => {
    const update = () => setScrolled(window.scrollY > 8);
    update();
    window.addEventListener('scroll', update, { passive: true });
    return () => window.removeEventListener('scroll', update);
  }, []);

  return (
    <header
      className={styles.header}
      data-scrolled={scrolled || undefined}
      data-admin={admin ? '' : undefined}
    >
      <SpatialGroup focusKey="header" className={styles.inner}>
        {menu && (
          <SpatialButton
            focusKey={HEADER_MENU_KEY}
            variant="ghost"
            icon="menu"
            iconOnly
            aria-label={menu.label}
            aria-expanded={menu.expanded}
            aria-controls={menu.controls}
            onPress={menu.onPress}
          />
        )}
        <AppLink ref={homeRef} onFocus={onHomeFocus} href={homeHref} className={styles.brand}>
          <Logo name={appName} tagline={admin?.tagline} />
        </AppLink>
        {admin && <Breadcrumb label={admin.breadcrumbLabel} crumbs={admin.crumbs} />}
        <div className={styles.account}>
          {admin ? <FinderButton finder={admin.finder} /> : <SearchLink search={search} />}
          {notifications && <InboxLink inbox={notifications} />}
          <AppLink
            ref={profileRef}
            onFocus={onProfileFocus}
            href={profileHref}
            className={styles.profile}
            aria-label={profileLabel}
          >
            <span className={styles.avatar} aria-hidden="true">
              {profileName.slice(0, 1).toUpperCase()}
            </span>
            <span className={styles.profileName} aria-hidden="true">
              {profileName}
            </span>
          </AppLink>
        </div>
      </SpatialGroup>
    </header>
  );
}
