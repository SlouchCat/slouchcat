import type { SessionInfo, StatsDays } from '@slouchcat/api-client';
import type { QueryClient } from '@tanstack/react-query';
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  lazyRouteComponent,
  redirect,
  type RouterHistory,
} from '@tanstack/react-router';
import { HomePage } from '@/features/catalog/HomePage';
import { ItemPage } from '@/features/catalog/ItemPage';
import { homeQuery } from '@/features/catalog/queries';
import { isStatsDays } from '@/features/admin/statsDays';
import { validateLibraryParams } from '@/features/catalog/libraryParams';
import { validateSearchParams } from '@/features/discover/searchParams';
import { leaveOfflineProfile, offlineChoice } from '@/features/offline/offlineSession';
import { offlineSupported } from '@/features/offline/support';
import { loadSpatialNavigation } from '@/ui';
import { AppLayout } from './AppLayout';
import { paths } from './paths';
import { isReturnTarget } from './returnTarget';
import { RootError, RootLayout } from './RootLayout';
import {
  applyProfileAccent,
  applyProfileLanguage,
  sessionQuery,
  setupStatusQuery,
  type ActiveSession,
} from './session';

export interface RouterContext {
  queryClient: QueryClient;
}

/** The offline library and its player: they never ask the server (C-1, O-4). */
const isOfflinePath = (pathname: string) =>
  pathname === paths.offline || pathname.startsWith(`${paths.offline}/`);

/**
 * Longest wait for the session before an app that can keep downloads opens the offline library
 * (C-1: the library within 3 s, also when the server does not answer at all, W10).
 */
export const SESSION_TIMEOUT_MS = 2000;

/** The session did not come within SESSION_TIMEOUT_MS. */
class SessionTimeout extends Error {
  constructor() {
    super(`no session within ${SESSION_TIMEOUT_MS} ms`);
    this.name = 'SessionTimeout';
  }
}

/**
 * fetch rejects with a TypeError when the network or the server cannot be reached; a session
 * that does not come in time counts the same.
 */
const isNetworkError = (error: unknown) =>
  error instanceof TypeError || error instanceof SessionTimeout;

/**
 * The session, asked at most SESSION_TIMEOUT_MS (a known one at once). Without network the
 * request fails at once and is not retried; a server that never answers is given up.
 */
async function sessionWithin(queryClient: QueryClient): Promise<SessionInfo | null> {
  const known = queryClient.getQueryData<SessionInfo | null>(sessionQuery.queryKey);
  if (known !== undefined) return known;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => reject(new SessionTimeout()), SESSION_TIMEOUT_MS);
  });
  try {
    return await Promise.race([
      // `always`: a browser that claims to be offline still asks (the server may be on this
      // machine, K11); without network the request fails at once, so no retry then.
      queryClient.fetchQuery({
        ...sessionQuery,
        networkMode: 'always',
        ...(navigator.onLine ? {} : { retry: false }),
      }),
      timeout,
    ]);
  } catch (error) {
    if (error instanceof SessionTimeout) {
      void queryClient.cancelQueries({ queryKey: sessionQuery.queryKey });
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

// Guard: the session first. A session exists only after the wizard ran, so `/setup` is asked only
// when there is none (401). On the start page its data is requested in parallel with the session
// instead of after the guard (P-10: two round trips less before the hero image). The prefetch is
// not awaited, so the page renders its skeleton as soon as the session is known.
// Offline start (C-1): where the browser can keep downloads, an app whose session request fails
// at the network (no network, the server unreachable) or does not answer within 2 s opens the
// offline library. A browser that only claims to be offline still reaches a server on the same
// machine (K11). Online nothing changes: a known session is never asked for again, and any other
// failure shows the error page (with the way to the offline library for a 5xx).
const rootRoute = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async ({ context: { queryClient }, location, preload }) => {
    if (isOfflinePath(location.pathname)) return;
    // The profile chosen in the offline library stays there: the app plays for the session's (W1).
    // A preload (hovering "Zur App" in the library) is not leaving it yet.
    if (!preload) leaveOfflineProfile();
    // Without download storage (plain HTTP) there is no offline library: the error page stays.
    const offline = offlineSupported();
    if (location.pathname === paths.home) void queryClient.prefetchQuery(homeQuery);
    let session: SessionInfo | null;
    try {
      session = offline
        ? await sessionWithin(queryClient)
        : await queryClient.ensureQueryData(sessionQuery);
    } catch (error) {
      if (offline && isNetworkError(error)) throw redirect({ to: '/offline' });
      throw error;
    }
    const setupRequired =
      !session && (await queryClient.ensureQueryData(setupStatusQuery)).setupRequired;
    const onSetup = location.pathname === paths.setup;
    if (setupRequired && !onSetup) throw redirect({ to: '/setup' });
    if (!setupRequired && onSetup) throw redirect({ to: '/' });
  },
  component: RootLayout,
  errorComponent: RootError,
});

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/setup',
  component: lazyRouteComponent(() => import('@/features/setup/SetupPage'), 'SetupPage'),
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  beforeLoad: async ({ context }) => {
    const session = await context.queryClient.ensureQueryData(sessionQuery);
    if (session) throw redirect({ to: '/' });
    // Signed out (logout or an expired session): back to the default accent.
    applyProfileAccent(null);
  },
  component: lazyRouteComponent(() => import('@/features/auth/LoginPage'), 'LoginPage'),
});

/** A visitor with an invitation link needs no session; the page works with or without one. */
const inviteRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/invite/$token',
  component: lazyRouteComponent(() => import('@/features/invite/InvitePage'), 'InvitePage'),
});

/**
 * The offline library (O-4, C-1): the downloads of this device for a profile chosen here, with
 * the PIN checked against the stored verifier. Reachable without a session and without network.
 */
const offlineRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/offline',
  beforeLoad: () => loadSpatialNavigation(),
  component: lazyRouteComponent(
    () => import('@/features/offline/OfflineLibraryPage'),
    'OfflineLibraryPage',
  ),
});

/**
 * The player of a download, online and offline. From the offline library it plays for the
 * profile chosen there; with `as=session` (from /downloads) for the session's active profile,
 * asking the server for the session after a reload (K9). Without either the library comes first.
 */
const offlinePlayRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/offline/play/$downloadId',
  validateSearch: (search: Record<string, unknown>): { as?: 'session' } =>
    search.as === 'session' ? { as: 'session' } : {},
  beforeLoad: async ({ context: { queryClient }, search }) => {
    if (search.as !== 'session') {
      if (!offlineChoice()) throw redirect({ to: '/offline' });
      return;
    }
    const session = await sessionWithin(queryClient).catch(() => undefined);
    if (!session?.profile) throw redirect({ to: '/offline' });
  },
  component: lazyRouteComponent(
    () => import('@/features/offline/OfflinePlayerPage'),
    'OfflinePlayerPage',
  ),
});

/** Everything behind the login: a session, with or without a chosen profile. */
const authedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: '_authed',
  beforeLoad: async ({ context }) => {
    const session = await context.queryClient.ensureQueryData(sessionQuery);
    if (!session) throw redirect({ to: '/login' });
    await applyProfileLanguage(session);
    applyProfileAccent(session);
    return { session };
  },
});

/** The profile picker (K-4): reachable with a session and no profile chosen yet. */
const profilesRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/profiles',
  validateSearch: (search: Record<string, unknown>): { next?: string } =>
    isReturnTarget(search.next) ? { next: search.next } : {},
  beforeLoad: () => loadSpatialNavigation(),
  component: lazyRouteComponent(
    () => import('@/features/profiles/ProfilePickerPage'),
    'ProfilePickerPage',
  ),
});

/** Everything that needs a chosen profile; without one the picker comes first. */
const activeRoute = createRoute({
  getParentRoute: () => authedRoute,
  id: '_active',
  beforeLoad: ({ context, location }) => {
    const { session } = context;
    if (!session.profile) {
      throw redirect({
        to: '/profiles',
        search: location.pathname === paths.home ? {} : { next: location.href },
      });
    }
    return { session: session as ActiveSession };
  },
});

/**
 * Pages with the app header; the player runs full screen without it. Header, start and detail
 * page navigate with norigin, which loads as its own chunk (spatial.tsx).
 */
const chromeRoute = createRoute({
  getParentRoute: () => activeRoute,
  id: '_chrome',
  beforeLoad: () => loadSpatialNavigation(),
  component: AppLayout,
});

const homeRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/',
  component: HomePage,
});

const itemRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/items/$itemId',
  component: ItemPage,
});

const personRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/people/$personId',
  component: lazyRouteComponent(() => import('@/features/catalog/PersonPage'), 'PersonPage'),
});

/**
 * The administration (M8, A-2): the overview with the dashboard and every admin page below one
 * layout that puts the admin navigation above each page. The pages check the role themselves.
 */
const adminRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/admin',
  component: lazyRouteComponent(() => import('@/features/admin/AdminLayout'), 'AdminLayout'),
});

const adminHomeRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/',
  component: lazyRouteComponent(() => import('@/features/admin/AdminHomePage'), 'AdminHomePage'),
});

const integrationsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/integrations',
  component: lazyRouteComponent(
    () => import('@/features/admin/IntegrationsPage'),
    'IntegrationsPage',
  ),
});

const maintenanceRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/maintenance',
  component: lazyRouteComponent(
    () => import('@/features/admin/MaintenancePage'),
    'MaintenancePage',
  ),
});

const devicesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/devices',
  component: lazyRouteComponent(() => import('@/features/admin/DevicesPage'), 'DevicesPage'),
});

const streamsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/streams',
  component: lazyRouteComponent(() => import('@/features/admin/StreamsPage'), 'StreamsPage'),
});

const adminItemRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/items/$itemId',
  component: lazyRouteComponent(() => import('@/features/admin/AdminItemPage'), 'AdminItemPage'),
});

const librariesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/libraries',
  component: lazyRouteComponent(() => import('@/features/admin/LibrariesPage'), 'LibrariesPage'),
});

const podcastsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/podcasts',
  component: lazyRouteComponent(() => import('@/features/admin/PodcastsPage'), 'PodcastsPage'),
});

// The "Zugriff" page became part of "Konten" (ADR 0014): the invitations sit below the account
// list, where the account an invitation creates is managed. Old links and bookmarks go there.
const accessRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/access',
  beforeLoad: () => {
    throw redirect({ to: paths.adminAccounts });
  },
});

// admin M7
/** Cat's recommendations; `add` (the detail page's link) opens the dialog with that title. */
const picksRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/picks',
  validateSearch: (search: Record<string, unknown>): { add?: string } =>
    typeof search.add === 'string' && /^[0-9A-HJKMNP-TV-Z]{26}$/.test(search.add)
      ? { add: search.add }
      : {},
  component: lazyRouteComponent(() => import('@/features/admin/PicksPage'), 'PicksPage'),
});

const collectionsAdminRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/collections',
  component: lazyRouteComponent(
    () => import('@/features/admin/CollectionsPage'),
    'CollectionsPage',
  ),
});

const homeRowsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/home-rows',
  component: lazyRouteComponent(() => import('@/features/admin/HomeRowsPage'), 'HomeRowsPage'),
});
// end admin M7

// admin M11: the spotlight list of the start page (ADR 0011).
const spotlightRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/spotlight',
  component: lazyRouteComponent(() => import('@/features/admin/SpotlightPage'), 'SpotlightPage'),
});

// admin M8: accounts, backup, statistics and announcements.
const accountsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/accounts',
  component: lazyRouteComponent(() => import('@/features/admin/AccountsPage'), 'AccountsPage'),
});

const backupRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/backup',
  component: lazyRouteComponent(() => import('@/features/admin/BackupPage'), 'BackupPage'),
});

/**
 * `days`: the period of the statistics (A-8); anything but 7, 30, 90 or 365 reads as none. The
 * undefined member overrides the raw value, which the router would otherwise pass on.
 */
const statsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/stats',
  validateSearch: (search: Record<string, unknown>): { days?: StatsDays } => {
    const days = Number(search.days);
    return { days: isStatsDays(days) ? days : undefined };
  },
  component: lazyRouteComponent(() => import('@/features/admin/StatsPage'), 'StatsPage'),
});

const announcementsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/announcements',
  component: lazyRouteComponent(
    () => import('@/features/admin/AnnouncementsPage'),
    'AnnouncementsPage',
  ),
});
// end admin M8

const profileSettingsRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/settings/profiles',
  component: lazyRouteComponent(
    () => import('@/features/profiles/ProfileSettingsPage'),
    'ProfileSettingsPage',
  ),
});

const notificationSettingsRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/settings/notifications',
  component: lazyRouteComponent(
    () => import('@/features/notifications/ChannelSettingsPage'),
    'ChannelSettingsPage',
  ),
});

const downloadsRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/downloads',
  component: lazyRouteComponent(() => import('@/features/offline/DownloadsPage'), 'DownloadsPage'),
});

// discover M7: search, watchlist, collections and the inbox; browsing pages without react-aria.
const searchRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/search',
  // q, genre, from, to, rating, type, avail (design 4.1); anything else is dropped.
  validateSearch: validateSearchParams,
  component: lazyRouteComponent(() => import('@/features/discover/SearchPage'), 'SearchPage'),
});

const watchlistRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/watchlist',
  component: lazyRouteComponent(() => import('@/features/discover/WatchlistPage'), 'WatchlistPage'),
});

const collectionRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/collections/$collectionId',
  component: lazyRouteComponent(
    () => import('@/features/discover/CollectionPage'),
    'CollectionPage',
  ),
});

// concept 08: a library as a grid and every one of Cat's recommendations.
const libraryRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/libraries/$libraryId',
  // content, sort, order, letter, genre, status, from, to, rating; anything else is dropped.
  validateSearch: validateLibraryParams,
  component: lazyRouteComponent(() => import('@/features/catalog/LibraryPage'), 'LibraryPage'),
});

const picksPageRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/picks',
  component: lazyRouteComponent(() => import('@/features/catalog/PicksPage'), 'PicksPage'),
});

const notificationsRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/notifications',
  component: lazyRouteComponent(
    () => import('@/features/discover/NotificationsPage'),
    'NotificationsPage',
  ),
});

const playRoute = createRoute({
  getParentRoute: () => activeRoute,
  path: '/play/$itemId',
  // `file` picks a version of the title (B-5), `at` a start position in ms (a chapter of an
  // audiobook, M6); anything else is dropped.
  validateSearch: (search: Record<string, unknown>): { file?: string; at?: number } => {
    const at = Number(search.at);
    return {
      ...(typeof search.file === 'string' && search.file !== '' ? { file: search.file } : {}),
      ...(search.at !== undefined && Number.isInteger(at) && at >= 0 ? { at } : {}),
    };
  },
  component: lazyRouteComponent(() => import('@/features/player/PlayerPage'), 'PlayerPage'),
});

// Home and detail are in the main bundle (LCP, P-10); the form pages (setup, login, admin) and the
// player load on demand, and with them react-aria.
export const routeTree = rootRoute.addChildren([
  setupRoute,
  loginRoute,
  inviteRoute,
  offlineRoute,
  offlinePlayRoute,
  authedRoute.addChildren([
    profilesRoute,
    activeRoute.addChildren([
      chromeRoute.addChildren([
        homeRoute,
        itemRoute,
        personRoute,
        adminRoute.addChildren([
          adminHomeRoute,
          integrationsRoute,
          librariesRoute,
          podcastsRoute,
          // admin M7
          picksRoute,
          collectionsAdminRoute,
          homeRowsRoute,
          // admin M11
          spotlightRoute,
          maintenanceRoute,
          devicesRoute,
          streamsRoute,
          adminItemRoute,
          accessRoute,
          // admin M8
          accountsRoute,
          backupRoute,
          statsRoute,
          announcementsRoute,
        ]),
        profileSettingsRoute,
        notificationSettingsRoute,
        downloadsRoute,
        // discover M7
        searchRoute,
        watchlistRoute,
        collectionRoute,
        notificationsRoute,
        // concept 08
        libraryRoute,
        picksPageRoute,
      ]),
      playRoute,
    ]),
  ]),
]);

export function createAppRouter(queryClient: QueryClient, history?: RouterHistory) {
  return createRouter({
    routeTree,
    history,
    context: { queryClient },
    defaultPreload: 'intent',
    // Route data comes from TanStack Query; the router itself caches nothing.
    defaultPreloadStaleTime: 0,
    scrollRestoration: true,
  });
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
