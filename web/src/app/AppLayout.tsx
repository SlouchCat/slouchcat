import { Outlet, useLocation } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { AppHeader, type NavItem } from '@/ui';
import { paths } from './paths';
import { useActiveSession, useLogout } from './session';
import styles from './AppLayout.module.css';

/** Header plus page for every signed-in page except the player and the profile picker. */
export function AppLayout() {
  const { t } = useTranslation();
  const session = useActiveSession();
  const { pathname } = useLocation();
  const logout = useLogout();

  // Only the account's primary profile manages anything (design 3); the main account's also the
  // libraries and guests.
  const isPrimary = session.profile.isPrimary;
  const isAdmin = isPrimary && session.account.role === 'owner';

  const nav: NavItem[] = [{ href: paths.home, label: t('nav.home'), current: pathname === '/' }];
  if (isAdmin) {
    nav.push(
      {
        href: paths.adminLibraries,
        label: t('nav.libraries'),
        current: pathname.startsWith(paths.adminLibraries),
      },
      {
        href: paths.adminAccess,
        label: t('nav.access'),
        current: pathname.startsWith(paths.adminAccess),
      },
      {
        href: paths.adminMaintenance,
        label: t('nav.maintenance'),
        current: pathname.startsWith(paths.adminMaintenance) || pathname.startsWith('/admin/items'),
      },
    );
  }
  if (isPrimary) {
    nav.push({
      href: paths.profileSettings,
      label: t('nav.profiles'),
      current: pathname.startsWith(paths.profileSettings),
    });
  }

  return (
    <>
      <a className={styles.skip} href="#content">
        {t('nav.skip')}
      </a>
      <AppHeader
        appName={t('app.name')}
        homeHref={paths.home}
        navLabel={t('nav.main')}
        nav={nav}
        profileName={session.profile.name}
        profileHref={paths.profiles}
        profileLabel={t('nav.switchProfile', { name: session.profile.name })}
        logoutLabel={t('nav.logout')}
        onLogout={() => logout.mutate()}
      />
      <div id="content" tabIndex={-1} className={styles.content}>
        <Outlet />
      </div>
    </>
  );
}
