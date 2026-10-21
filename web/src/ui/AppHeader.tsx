import { useEffect, useState } from 'react';
import { AppLink } from './AppLink';
import styles from './AppHeader.module.css';
import { SpatialButton } from './Button';
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

/** The update hint of the main account (A-5): a small badge that links to the overview. */
export interface HeaderUpdate {
  href: string;
  /** Visible text of the badge ("Update"). */
  label: string;
  /** Accessible name with the version ("Update auf Version 0.9.1 verfügbar"). */
  accessibleLabel?: string;
}

interface AppHeaderProps {
  appName: string;
  homeHref: string;
  navLabel: string;
  nav: readonly NavItem[];
  profileName: string;
  /** The profile picker: avatar and name link there to switch profiles. */
  profileHref: string;
  /** Accessible name of that link ("Switch profile, now Marco"). */
  profileLabel: string;
  logoutLabel: string;
  onLogout: () => void;
  notifications?: HeaderNotifications;
  update?: HeaderUpdate;
}

function UpdateLink({ update }: { update: HeaderUpdate }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>();
  return (
    <AppLink
      ref={ref}
      onFocus={onFocus}
      href={update.href}
      className={styles.update}
      aria-label={update.accessibleLabel}
    >
      {update.label}
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
      <span aria-hidden="true">{inbox.label}</span>
      <NotificationBadge count={inbox.unread} overflowLabel={inbox.overflowLabel} />
    </AppLink>
  );
}

function NavLink({ item }: { item: NavItem }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>();
  return (
    <AppLink
      ref={ref}
      onFocus={onFocus}
      href={item.href}
      className={styles.navLink}
      aria-current={item.current ? 'page' : undefined}
    >
      {item.label}
    </AppLink>
  );
}

/**
 * Fixed top bar shared by viewer and admin pages. Transparent over artwork, solid once the page
 * scrolls. Takes part in spatial navigation (arrow up from the first row lands here).
 */
export function AppHeader({
  appName,
  homeHref,
  navLabel,
  nav,
  profileName,
  profileHref,
  profileLabel,
  logoutLabel,
  onLogout,
  notifications,
  update,
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
        <AppLink ref={homeRef} onFocus={onHomeFocus} href={homeHref} className={styles.brand}>
          <Logo name={appName} />
        </AppLink>
        <nav aria-label={navLabel} className={styles.nav}>
          {nav.map((item) => (
            <NavLink key={item.href} item={item} />
          ))}
        </nav>
        <div className={styles.account}>
          {update && <UpdateLink update={update} />}
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
          <SpatialButton
            variant="ghost"
            icon="logout"
            iconOnly
            aria-label={logoutLabel}
            onPress={onLogout}
          />
        </div>
      </SpatialGroup>
    </header>
  );
}
