import { getRouteApi, Outlet, useLocation } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { AppHeader, type NavItem } from '@/ui';
import { paths } from './paths';
import { useLogout } from './session';
import styles from './AppLayout.module.css';

const authed = getRouteApi('/_authed');

/** Header plus page for every signed-in page except the player. */
export function AppLayout() {
  const { t } = useTranslation();
  const { session } = authed.useRouteContext();
  const { pathname } = useLocation();
  const logout = useLogout();

  const nav: NavItem[] = [{ href: paths.home, label: t('nav.home'), current: pathname === '/' }];
  if (session.account.role === 'owner') {
    nav.push({
      href: paths.adminLibraries,
      label: t('nav.libraries'),
      current: pathname.startsWith('/admin/libraries'),
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
        logoutLabel={t('nav.logout')}
        onLogout={() => logout.mutate()}
      />
      <div id="content" tabIndex={-1} className={styles.content}>
        <Outlet />
      </div>
    </>
  );
}
