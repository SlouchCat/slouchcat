import { useEffect, useState } from 'react';
import { Link } from 'react-aria-components';
import styles from './AppHeader.module.css';
import { SpatialButton } from './Button';
import { Logo } from './Logo';
import { SpatialGroup, useSpatialItem } from './spatial';

export interface NavItem {
  href: string;
  label: string;
  current?: boolean;
}

interface AppHeaderProps {
  appName: string;
  homeHref: string;
  navLabel: string;
  nav: readonly NavItem[];
  profileName: string;
  logoutLabel: string;
  onLogout: () => void;
}

function NavLink({ item }: { item: NavItem }) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>();
  return (
    <Link
      ref={ref}
      onFocus={onFocus}
      href={item.href}
      className={styles.navLink}
      aria-current={item.current ? 'page' : undefined}
    >
      {item.label}
    </Link>
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
  logoutLabel,
  onLogout,
}: AppHeaderProps) {
  const [scrolled, setScrolled] = useState(false);
  const { ref: homeRef, onFocus: onHomeFocus } = useSpatialItem<HTMLAnchorElement>();

  useEffect(() => {
    const update = () => setScrolled(window.scrollY > 8);
    update();
    window.addEventListener('scroll', update, { passive: true });
    return () => window.removeEventListener('scroll', update);
  }, []);

  return (
    <header className={styles.header} data-scrolled={scrolled || undefined}>
      <SpatialGroup focusKey="header" className={styles.inner}>
        <Link ref={homeRef} onFocus={onHomeFocus} href={homeHref} className={styles.brand}>
          <Logo name={appName} />
        </Link>
        <nav aria-label={navLabel} className={styles.nav}>
          {nav.map((item) => (
            <NavLink key={item.href} item={item} />
          ))}
        </nav>
        <div className={styles.account}>
          <span className={styles.profile}>
            <span className={styles.avatar} aria-hidden="true">
              {profileName.slice(0, 1).toUpperCase()}
            </span>
            <span className={styles.profileName}>{profileName}</span>
          </span>
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
