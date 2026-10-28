import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Outlet, useLocation, useNavigate } from '@tanstack/react-router';
import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { notificationsPageQuery, notificationsQuery } from '@/features/discover/queries';
import { Announcements } from '@/features/notifications/Announcements';
import { offlineSupported } from '@/features/offline/support';
import { AppHeader, BADGE_MAX, type NavItem } from '@/ui';
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
  const queryClient = useQueryClient();
  // A new notification (the badge counts up) reaches an open inbox page as well.
  const counted = inbox.data?.unread;
  const lastCounted = useRef(counted);
  useEffect(() => {
    const before = lastCounted.current;
    lastCounted.current = counted;
    if (before !== undefined && counted !== undefined && counted > before) {
      void queryClient.invalidateQueries({ queryKey: notificationsPageQuery.queryKey });
    }
  }, [counted, queryClient]);

  // "/" opens the search from any page with the header; the search page focuses its field itself.
  useEffect(() => {
    if (pathname === paths.search) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return;
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
  const isAdmin = isPrimary && session.account.role === 'admin';

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
  // The administration is one entry (M8): its pages have a navigation of their own (AdminNav).
  if (isAdmin) {
    nav.push({
      href: paths.admin,
      label: t('nav.admin'),
      current: pathname === paths.admin || pathname.startsWith(`${paths.admin}/`),
    });
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
          overflowLabel: t('notifications.badgeMax', { max: BADGE_MAX }),
          current: pathname.startsWith(paths.notifications),
        }}
        update={
          isAdmin && session.updateAvailable
            ? {
                href: paths.admin,
                label: t('nav.update'),
                accessibleLabel: t('nav.updateLabel', {
                  version: session.updateAvailable.version,
                }),
              }
            : undefined
        }
      />
      <div id="content" tabIndex={-1} className={styles.content}>
        <Outlet />
      </div>
      <Announcements profileId={session.profile.id} />
    </>
  );
}
