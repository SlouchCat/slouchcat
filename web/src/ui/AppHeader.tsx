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

interface AppHeaderProps {
  appName: string;
  homeHref: string;
  menu?: HeaderMenu;
  search: HeaderSearch;
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
    <header className={styles.header} data-scrolled={scrolled || undefined}>
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
          <Logo name={appName} />
        </AppLink>
        <div className={styles.account}>
          <SearchLink search={search} />
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
