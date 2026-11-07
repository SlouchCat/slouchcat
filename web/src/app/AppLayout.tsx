import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Outlet, useLocation, useNavigate } from '@tanstack/react-router';
import {
  lazy,
  Suspense,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ComponentProps,
  type ComponentType,
} from 'react';
import { useTranslation } from 'react-i18next';
import { picksCountQuery, visibleLibrariesQuery } from '@/features/catalog/queries';
import { notificationsPageQuery, notificationsQuery } from '@/features/discover/queries';
import { Announcements } from '@/features/notifications/Announcements';
import type { AdminShellNav as AdminShellNavType } from '@/features/admin/AdminShellNav';
import { offlineSupported } from '@/features/offline/support';
import { useWideScreen } from '@/lib/useWideScreen';
import {
  AppHeader,
  BADGE_MAX,
  HEADER_MENU_KEY,
  setSpatialFocus,
  SIDENAV_HIDE_KEY,
  SideNav,
  SideNavMenu,
  sideNavFocusKey,
  SpatialGroup,
  type NavItem,
  type SideNavEntry,
  type SideNavProps,
} from '@/ui';
import { paths } from './paths';
import { RADIO_FOCUS_KEY, RadioDock } from './radio/RadioDock';
import { useRadio } from './radio/useRadio';
import { useActiveSession, useLogout } from './session';
import styles from './AppLayout.module.css';

type AdminShellNavModule = { default: ComponentType<ComponentProps<typeof AdminShellNavType>> };

/**
 * The administration's sidebar (concept 11), loaded on the first /admin page: it builds on
 * '@/ui/aria', which must stay out of the start bundle (P-10). While it loads, the width of the
 * column is already reserved (`data-shell="admin"`, global.css), so nothing moves.
 */
const AdminShellNav = lazy(() =>
  import('@/features/admin/AdminShellNav').then(
    (module): AdminShellNavModule => ({ default: module.AdminShellNav }),
    // A chunk that does not load (an old tab after an update) leaves the column empty, not the
    // whole shell on an error page.
    (): AdminShellNavModule => ({ default: () => null }),
  ),
);

/** Index of «Übersicht» among the sidebar's entries (after «Zurück zur App»). */
const ADMIN_OVERVIEW_INDEX = 1;

/** localStorage key of the hidden side navigation; index.html reads it before the first paint. */
const SIDENAV_KEY = 'slouchcat.sidenav';

function readSideNavHidden(): boolean {
  try {
    return localStorage.getItem(SIDENAV_KEY) === 'hidden';
  } catch {
    return false;
  }
}

/** Stores the state on this device and mirrors it on <html> (--sidenav-width in global.css). */
function storeSideNavHidden(hidden: boolean): void {
  try {
    if (hidden) localStorage.setItem(SIDENAV_KEY, 'hidden');
    else localStorage.removeItem(SIDENAV_KEY);
  } catch {
    // Without storage the column stays hidden or shown for this visit only.
  }
  if (hidden) document.documentElement.dataset.sidenav = 'hidden';
  else delete document.documentElement.dataset.sidenav;
}

/** Id of the drawer, named by the menu button. */
const MENU_ID = 'app-menu';

/**
 * Arrow keys between the shell's parts (concept 08, 7.1). The header spans the column and the
 * content and the content starts behind the header, so their boxes overlap and norigin's geometry
 * cannot tell them apart: up from the content or the column is the header, left from the content
 * the column (on the current page's entry), right from the column the content (where the viewer
 * left it), down from the header the content. The radio's mini player (concept 16) floats over
 * the bottom of the content: down from the content, up back into it.
 */
function shellFocus(direction: string, from: string): string | null {
  if (from === 'content') {
    return { up: 'header', left: 'sidenav', down: RADIO_FOCUS_KEY }[direction] ?? null;
  }
  if (from === RADIO_FOCUS_KEY) return { up: 'content', left: 'sidenav' }[direction] ?? null;
  if (from === 'sidenav') return { up: 'header', right: 'content' }[direction] ?? null;
  if (from === 'header') return direction === 'down' ? 'content' : null;
  return null;
}

/**
 * Header, side navigation and page for every signed-in page except the player and the profile
 * picker. Tab order: skip link, header, side navigation, content.
 */
export function AppLayout() {
  const { t } = useTranslation();
  const session = useActiveSession();
  const { pathname } = useLocation();
  const logout = useLogout();
  const navigate = useNavigate();
  // The mini player takes room at the bottom while a station is chosen.
  const radioShown = useRadio().state.station !== null;
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

  // On /admin/* an administrator gets the administration's sidebar in the column's place instead
  // (concept 11, ADR 0019), from 768 px always, even when this device hides the app's column; the
  // menu button opens the app's drawer there. Below 768 px the admin pages have a sheet of their
  // own (AdminLayout).
  const adminShell =
    isAdmin && (pathname === paths.admin || pathname.startsWith(`${paths.admin}/`));
  // index.html sets data-shell="admin" before the first paint, by the path alone; this keeps it
  // right from then on, before every paint (the reserved width of the column, global.css).
  useLayoutEffect(() => {
    const root = document.documentElement;
    if (!adminShell) {
      delete root.dataset.shell;
      return;
    }
    root.dataset.shell = 'admin';
    return () => {
      delete root.dataset.shell;
    };
  }, [adminShell]);
  // The admin sidebar's focused entry, carried over when it mounts again (AdminSideNav).
  const adminFocus = useRef<number | null>(null);

  // The column from 768 px unless the viewer hid it (per device); otherwise the drawer behind the
  // menu button. The drawer belongs to the page it was opened on: a new page closes it, and so
  // does the column appearing.
  const wide = useWideScreen();
  const [hidden, setHidden] = useState(readSideNavHidden);
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const barShown = wide && !hidden && !adminShell;
  const menuOpen = menuFor === pathname && !barShown;
  // Hiding or showing the column, or swapping it for the admin sidebar, takes away the focused
  // entry: the focus moves to what replaces it.
  const focusAfter = useRef<string | null>(null);
  useEffect(() => {
    const key = focusAfter.current;
    if (!key) return;
    focusAfter.current = null;
    const frame = requestAnimationFrame(() => setSpatialFocus(key));
    return () => cancelAnimationFrame(frame);
  }, [barShown, adminShell]);
  const setSideNavHidden = (value: boolean) => {
    storeSideNavHidden(value);
    focusAfter.current = value ? HEADER_MENU_KEY : SIDENAV_HIDE_KEY;
    setMenuFor(null);
    setHidden(value);
  };

  // "Meine Medien" (concept 08): the libraries come from the server and never hold up the shell;
  // without them (offline, an error) the group says so and points to the downloads.
  const librariesQuery = useQuery(visibleLibrariesQuery);
  const picksCount = useQuery(picksCountQuery);
  const libraries: SideNavProps['libraries'] = librariesQuery.data
    ? librariesQuery.data.map((library) => ({
        href: paths.library(library.id),
        label: library.name,
        count: library.itemCount,
        accessibleLabel: t('nav.libraryLabel', { name: library.name, count: library.itemCount }),
        current: pathname === paths.library(library.id),
      }))
    : librariesQuery.isError
      ? 'offline'
      : 'loading';
  const picks: SideNavEntry = {
    href: paths.picks,
    label: t('home.picks'),
    count: picksCount.data,
    accessibleLabel:
      picksCount.data === undefined
        ? undefined
        : t('nav.libraryLabel', { name: t('home.picks'), count: picksCount.data }),
    current: pathname === paths.picks,
  };

  const nav: NavItem[] = [
    { href: paths.home, label: t('nav.home'), current: pathname === '/' },
    { href: paths.search, label: t('nav.search'), current: pathname.startsWith(paths.search) },
    {
      href: paths.watchlist,
      label: t('nav.watchlist'),
      current: pathname.startsWith(paths.watchlist),
    },
    // Webradio (concept 16, 10.1).
    { href: paths.radio, label: t('nav.radio'), current: pathname.startsWith(paths.radio) },
  ];
  // Every profile has its downloads on this device (M5), where the browser can keep them (K13).
  if (offlineSupported()) {
    nav.push({
      href: paths.downloads,
      label: t('nav.downloads'),
      current: pathname.startsWith(paths.downloads),
    });
  }
  const footer: SideNavEntry[] = [];
  // The administration is one entry (M8): its pages have a sidebar of their own (AdminShellNav).
  // The update hint of the main account (A-5) is its badge; on /admin/* the sidebar's overview
  // entry carries it.
  if (isAdmin) {
    const update = session.updateAvailable;
    footer.push({
      href: paths.admin,
      label: t('nav.admin'),
      current: pathname === paths.admin || pathname.startsWith(`${paths.admin}/`),
      badge: update ? t('nav.update') : undefined,
      accessibleLabel: update ? t('nav.adminUpdateLabel', { version: update.version }) : undefined,
    });
  }
  if (isPrimary) {
    footer.push({
      href: paths.profileSettings,
      label: t('nav.profiles'),
      current: pathname.startsWith(paths.profileSettings),
    });
  }
  // Every profile sets its own languages and subtitles (concept 13 11.4, F-13).
  footer.push({
    href: paths.playbackSettings,
    label: t('nav.playback'),
    current: pathname.startsWith(paths.playbackSettings),
  });

  const sideNav: SideNavProps = {
    label: t('nav.main'),
    primary: nav,
    librariesLabel: t('nav.libraries'),
    toggleLibrariesLabel: t('nav.toggleLibraries'),
    libraries,
    picks: libraries === 'offline' ? undefined : picks,
    librariesOffline: t('nav.librariesOffline'),
    offlineLink: offlineSupported()
      ? { href: paths.downloads, label: t('nav.librariesOfflineLink') }
      : undefined,
    footer,
    logoutLabel: t('nav.logout'),
    onLogout: () => logout.mutate(),
    hideLabel: t('nav.hideSidebar'),
    onHide: () => setSideNavHidden(true),
  };

  // Into the administration from the app column, or back: the column that had the focus goes away,
  // the focus falls to the body. Entering, the admin sidebar takes it on «Übersicht» once it has
  // loaded; leaving, the app column on the current page's entry (or the menu button when the
  // column is hidden). What the sidebar carried from an earlier visit is forgotten either way.
  const currentEntry = [...nav, ...(typeof libraries === 'string' ? [] : [...libraries, picks])]
    .concat(footer)
    .find((entry) => entry.current);
  const returnKey = barShown ? currentEntry && sideNavFocusKey(currentEntry.href) : HEADER_MENU_KEY;
  const wasAdminShell = useRef(adminShell);
  useLayoutEffect(() => {
    if (wasAdminShell.current === adminShell) return;
    wasAdminShell.current = adminShell;
    const active = document.activeElement;
    const lost = active === null || active === document.body;
    adminFocus.current = adminShell && lost ? ADMIN_OVERVIEW_INDEX : null;
    if (!adminShell && lost) focusAfter.current = returnKey ?? null;
  }, [adminShell, returnKey]);

  return (
    <>
      <a className={styles.skip} href="#content">
        {t('nav.skip')}
      </a>
      {/*
        The column (or the admin sidebar) that had the focus goes away with the page: norigin
        would pull the focus into the header 300 ms later, out of the sidebar that took it
        (focusAfter and AdminSideNav place the focus themselves).
      */}
      <SpatialGroup
        focusKey="shell"
        rememberLastChild={false}
        autoRestoreFocus={false}
        nextFocusKey={shellFocus}
      >
        <AppHeader
          appName={t('app.name')}
          homeHref={paths.home}
          menu={
            barShown
              ? undefined
              : {
                  label: t('nav.menu'),
                  expanded: menuOpen,
                  controls: MENU_ID,
                  onPress: () => setMenuFor(pathname),
                }
          }
          search={{
            href: paths.search,
            label: t('nav.search'),
            accessibleLabel: t('nav.searchOpen'),
            current: pathname.startsWith(paths.search),
          }}
          profileName={session.profile.name}
          profileHref={paths.profiles}
          profileLabel={t('nav.switchProfile', { name: session.profile.name })}
          notifications={{
            href: paths.notifications,
            label: t('nav.notifications'),
            accessibleLabel:
              unread > 0
                ? t('notifications.unreadLabel', { count: unread })
                : t('nav.notifications'),
            unread,
            overflowLabel: t('notifications.badgeMax', { max: BADGE_MAX }),
            current: pathname.startsWith(paths.notifications),
          }}
        />
        {barShown && <SideNav {...sideNav} />}
        {adminShell && wide && (
          <Suspense fallback={null}>
            <AdminShellNav focusCarry={adminFocus} />
          </Suspense>
        )}
        <div
          id="content"
          tabIndex={-1}
          className={styles.content}
          data-radio={radioShown ? '' : undefined}
        >
          <SpatialGroup focusKey="content">
            <Outlet />
          </SpatialGroup>
        </div>
        {/* After the content in the Tab order; it stays over the bottom of every page. */}
        <div className={styles.radio}>
          <RadioDock />
        </div>
        <SideNavMenu
          {...sideNav}
          id={MENU_ID}
          isOpen={menuOpen}
          onClose={() => setMenuFor(null)}
          menuLabel={t('nav.menu')}
          closeLabel={t('nav.menuClose')}
          // On /admin/* the column is the administration's: showing the app's would change nothing.
          show={
            wide && !adminShell
              ? { label: t('nav.showSidebar'), onPress: () => setSideNavHidden(false) }
              : undefined
          }
        />
      </SpatialGroup>
      <Announcements profileId={session.profile.id} />
    </>
  );
}
