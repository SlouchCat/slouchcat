import { useQuery } from '@tanstack/react-query';
import { Outlet, useLocation, useNavigate } from '@tanstack/react-router';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { notificationsQuery } from '@/features/discover/queries';
import { offlineSupported } from '@/features/offline/support';
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
  const navigate = useNavigate();
  // Unread notifications for the badge (E-8), asked every 30 s; this layout only renders with an
  // active profile (router: _active).
  const inbox = useQuery(notificationsQuery);
  const unread = inbox.data?.unread ?? 0;

  // "/" opens the search from any page with the header; the search page focuses its field itself.
  useEffect(() => {
    if (pathname === paths.search) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey) return;
      const target = event.target;
      if (
        target instanceof HTMLElement &&
        (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))
      ) {
        return;
      }
      if (target instanceof Element && target.closest('[role="dialog"], [role="alertdialog"]')) {
        return;
      }
      event.preventDefault();
      void navigate({ to: '/search' });
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [navigate, pathname]);

  // Only the account's primary profile manages anything (design 3); the main account's also the
  // libraries and guests.
  const isPrimary = session.profile.isPrimary;
  const isAdmin = isPrimary && session.account.role === 'owner';

  const nav: NavItem[] = [
    { href: paths.home, label: t('nav.home'), current: pathname === '/' },
    { href: paths.search, label: t('nav.search'), current: pathname.startsWith(paths.search) },
    {
      href: paths.watchlist,
      label: t('nav.watchlist'),
      current: pathname.startsWith(paths.watchlist),
    },
  ];
  // Every profile has its downloads on this device (M5), where the browser can keep them (K13).
  if (offlineSupported()) {
    nav.push({
      href: paths.downloads,
      label: t('nav.downloads'),
      current: pathname.startsWith(paths.downloads),
    });
  }
  if (isAdmin) {
    nav.push(
      {
        href: paths.adminLibraries,
        label: t('nav.libraries'),
        current: pathname.startsWith(paths.adminLibraries),
      },
      {
        href: paths.adminPodcasts,
        label: t('nav.podcasts'),
        current: pathname.startsWith(paths.adminPodcasts),
      },
      {
        href: paths.adminPicks,
        label: t('nav.picks'),
        current: pathname.startsWith(paths.adminPicks),
      },
      {
        href: paths.adminCollections,
        label: t('nav.collections'),
        current: pathname.startsWith(paths.adminCollections),
      },
      {
        href: paths.adminHomeRows,
        label: t('nav.homeRows'),
        current: pathname.startsWith(paths.adminHomeRows),
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
      {
        href: paths.adminDevices,
        label: t('nav.devices'),
        current: pathname.startsWith(paths.adminDevices),
      },
      {
        href: paths.adminStreams,
        label: t('nav.streams'),
        current: pathname.startsWith(paths.adminStreams),
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
        notifications={{
          href: paths.notifications,
          label: t('nav.notifications'),
          accessibleLabel:
            unread > 0 ? t('notifications.unreadLabel', { count: unread }) : t('nav.notifications'),
          unread,
          current: pathname.startsWith(paths.notifications),
        }}
      />
      <div id="content" tabIndex={-1} className={styles.content}>
        <Outlet />
      </div>
    </>
  );
}
