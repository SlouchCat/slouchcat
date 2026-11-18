import type { MyMediaEntry } from '@slouchcat/api-client';
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
import { myMediaQuery } from '@/features/catalog/queries';
import { notificationsQuery, notificationsSource } from '@/features/discover/queries';
import { Announcements } from '@/features/notifications/Announcements';
import type { AdminFinder as AdminFinderType } from '@/features/admin/AdminFinder';
import type { AdminShellNav as AdminShellNavType } from '@/features/admin/AdminShellNav';
import { currentSection, firstSectionOf } from '@/features/admin/sections';
import { offlineSupported } from '@/features/offline/support';
import { wishCountsQuery } from '@/features/wishes/queries';
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
  type HeaderAdmin,
  type SideNavEntry,
  type SideNavProps,
} from '@/ui';
import { libraryIcon } from './libraryIcon';
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

type AdminFinderModule = { default: ComponentType<ComponentProps<typeof AdminFinderType>> };

/**
 * The administration's search palette (build plan 2.4), loaded when it opens first: like the
 * sidebar it builds on '@/ui/aria' (P-10).
 */
const AdminFinder = lazy(() =>
  import('@/features/admin/AdminFinder').then(
    (module): AdminFinderModule => ({ default: module.AdminFinder }),
    (): AdminFinderModule => ({ default: () => null }),
  ),
);

/** Whether the shortcuts of this device use ⌘ (Apple) rather than Ctrl. */
const appleKeys = () => /Mac|iPhone|iPad/.test(navigator.userAgent);

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
      void queryClient.invalidateQueries({ queryKey: notificationsSource.key });
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
  // The search palette of the administration (build plan 2.4): «Bereich suchen» in the header, and
  // Ctrl+K / ⌘K on every admin page while no dialog is open. "/" keeps opening the media search.
  const [finderOpen, setFinderOpen] = useState(false);
  useEffect(() => {
    if (!adminShell) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.altKey || event.shiftKey) return;
      if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== 'k') return;
      if (document.querySelector('[role="dialog"], [role="alertdialog"]')) return;
      event.preventDefault();
      setFinderOpen(true);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [adminShell]);
  const adminHeader = (): HeaderAdmin => {
    const section = currentSection(pathname);
    const root = { href: paths.admin, label: t('admin.nav.label') };
    const apple = appleKeys();
    return {
      tagline: t('admin.nav.tagline'),
      breadcrumbLabel: t('admin.nav.breadcrumb'),
      crumbs: section
        ? [
            root,
            {
              href: firstSectionOf(section.area).href,
              label: t(`admin.nav.areas.${section.area}`),
            },
            { href: section.href, label: t(`admin.sections.${section.key}.title`) },
          ]
        : [root, { href: paths.admin, label: t('admin.nav.overview') }],
      finder: {
        label: t('admin.finder.button'),
        shortcut: t(apple ? 'admin.finder.shortcutMac' : 'admin.finder.shortcut'),
        keyShortcuts: apple ? 'Meta+K' : 'Control+K',
        expanded: finderOpen,
        onPress: () => setFinderOpen(true),
      },
    };
  };

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

  // "Meine Medien" (concept 08; concept 21 P-9, P-11): the libraries, «Radio», «Sammlungen» and
  // «User-Empfehlungen» come from the server in the order the administration stored, without what
  // it hid here, and never hold up the shell; without them (offline, an error) the group says so
  // and points to the downloads.
  const myMedia = useQuery(myMediaQuery);
  // «Wünsche» (concept 25, 5): its count of open wishes, read only while the server says the
  // wishes are on; the entry shows for a profile that may request or has wishes.
  const wishRights = session.wishes;
  const wishCounts = useQuery({ ...wishCountsQuery, enabled: wishRights?.enabled === true });
  const wishTotal = wishCounts.data
    ? wishCounts.data.open + wishCounts.data.available + wishCounts.data.closed
    : 0;
  const mediaEntry = (entry: MyMediaEntry): SideNavEntry => {
    const { count } = entry;
    if (entry.library) {
      const { library } = entry;
      return {
        href: paths.library(library.id),
        label: library.name,
        icon: libraryIcon(library),
        count,
        accessibleLabel: t('nav.libraryLabel', { name: library.name, count }),
        current: pathname === paths.library(library.id),
      };
    }
    if (entry.key === 'radio') {
      // Webradio (concept 16, 10.1), with the number of the profile's favourites.
      return {
        href: paths.radio,
        label: t('nav.radio'),
        icon: 'radio',
        count: t('nav.radioCount', { count }),
        accessibleLabel: t('nav.radioLabel', { name: t('nav.radio'), count }),
        current: pathname.startsWith(paths.radio),
      };
    }
    if (entry.key === 'collections') {
      return {
        href: paths.collections,
        label: t('nav.collections'),
        icon: 'stack',
        count,
        accessibleLabel: t('nav.collectionsLabel', { name: t('nav.collections'), count }),
        current: pathname.startsWith(paths.collections),
      };
    }
    return {
      href: paths.picks,
      label: t('home.picks'),
      icon: 'star',
      count,
      accessibleLabel: t('nav.libraryLabel', { name: t('home.picks'), count }),
      current: pathname === paths.picks,
    };
  };
  const libraries: SideNavProps['libraries'] = myMedia.data
    ? myMedia.data.map(mediaEntry)
    : myMedia.isError
      ? 'offline'
      : 'loading';

  const nav: SideNavEntry[] = [
    { href: paths.home, label: t('nav.home'), icon: 'home', current: pathname === '/' },
    {
      href: paths.search,
      label: t('nav.search'),
      icon: 'search',
      current: pathname.startsWith(paths.search),
    },
    // Without a count (concept 17, decision of 8 November 2026).
    {
      href: paths.watchlist,
      label: t('nav.watchlist'),
      icon: 'bookmark',
      current: pathname.startsWith(paths.watchlist),
    },
    // Everything the profile has watched or heard, below «Merkliste» for every profile (concept
    // 20 P-14); «Radio» moved into «Meine Medien» (concept 21 P-9).
    {
      href: paths.watched,
      label: t('nav.watched'),
      icon: 'check',
      current: pathname.startsWith(paths.watched),
    },
  ];
  if (wishRights?.enabled && (wishRights.canRequest || wishTotal > 0)) {
    const open = wishCounts.data?.open ?? 0;
    nav.push({
      // «Entdecken» while the server shows the area's pages, else «Meine Wünsche» (concept 25, 5.4).
      href: wishRights.discover ? paths.discover : paths.wishes,
      label: t('nav.wishes'),
      icon: 'heart',
      ...(open > 0
        ? {
            count: open,
            accessibleLabel: t('nav.wishesLabel', { name: t('nav.wishes'), count: open }),
          }
        : {}),
      current:
        pathname.startsWith(paths.wishes) ||
        pathname.startsWith(paths.discover) ||
        pathname.startsWith('/titles/'),
    });
  }
  // Every profile has its downloads on this device (M5), where the browser can keep them (K13).
  if (offlineSupported()) {
    nav.push({
      href: paths.downloads,
      label: t('nav.downloads'),
      icon: 'download',
      current: pathname.startsWith(paths.downloads),
    });
  }
  // The band «Konto» (concept 17, D-8): every profile sets its own languages and subtitles
  // (concept 13 11.4, F-13).
  const account: SideNavEntry[] = [
    {
      href: paths.playbackSettings,
      label: t('nav.playback'),
      icon: 'sliders',
      current: pathname.startsWith(paths.playbackSettings),
    },
  ];
  if (isPrimary) {
    account.push({
      href: paths.profileSettings,
      label: t('nav.profiles'),
      icon: 'users',
      current: pathname.startsWith(paths.profileSettings),
    });
  }
  // The administration is one entry (M8): its pages have a sidebar of their own (AdminShellNav).
  // The update hint of the main account (A-5) is its badge; on /admin/* the sidebar's overview
  // entry carries it.
  if (isAdmin) {
    const update = session.updateAvailable;
    account.push({
      href: paths.admin,
      label: t('nav.admin'),
      icon: 'wrench',
      current: pathname === paths.admin || pathname.startsWith(`${paths.admin}/`),
      badge: update ? t('nav.update') : undefined,
      accessibleLabel: update ? t('nav.adminUpdateLabel', { version: update.version }) : undefined,
    });
  }

  const sideNav: SideNavProps = {
    label: t('nav.main'),
    profile: {
      name: session.profile.name,
      role: t(
        isAdmin
          ? 'nav.profileRole.admin'
          : isPrimary
            ? 'nav.profileRole.primary'
            : 'nav.profileRole.other',
      ),
      href: paths.profiles,
      switchLabel: t('nav.switchProfileShort'),
      switchAccessibleLabel: t('nav.switchProfileCard', { name: session.profile.name }),
    },
    primary: nav,
    librariesLabel: t('nav.libraries'),
    libraries,
    librariesOffline: t('nav.librariesOffline'),
    offlineLink: offlineSupported()
      ? { href: paths.downloads, label: t('nav.librariesOfflineLink') }
      : undefined,
    accountLabel: t('nav.account'),
    account,
    logoutLabel: t('nav.logout'),
    onLogout: () => logout.mutate(),
    hideLabel: t('nav.hideSidebar'),
    onHide: () => setSideNavHidden(true),
  };

  // Into the administration from the app column, or back: the column that had the focus goes away,
  // the focus falls to the body. Entering, the admin sidebar takes it on «Übersicht» once it has
  // loaded; leaving, the app column on the current page's entry (or the menu button when the
  // column is hidden). What the sidebar carried from an earlier visit is forgotten either way.
  const currentEntry = [...nav, ...(typeof libraries === 'string' ? [] : libraries)]
    .concat(account)
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
          admin={adminShell ? adminHeader() : undefined}
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
      {adminShell && finderOpen && (
        <Suspense fallback={null}>
          <AdminFinder onClose={() => setFinderOpen(false)} />
        </Suspense>
      )}
      <Announcements profileId={session.profile.id} />
    </>
  );
}
