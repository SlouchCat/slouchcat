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

// Guard: the session first. A session exists only after the wizard ran, so `/setup` is asked only
// when there is none (401). On the start page its data is requested in parallel with the session
// instead of after the guard (P-10: two round trips less before the hero image). The prefetch is
// not awaited, so the page renders its skeleton as soon as the session is known.
const rootRoute = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async ({ context: { queryClient }, location }) => {
    if (location.pathname === paths.home) void queryClient.prefetchQuery(homeQuery);
    const session = await queryClient.ensureQueryData(sessionQuery);
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

const librariesRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/admin/libraries',
  component: lazyRouteComponent(() => import('@/features/admin/LibrariesPage'), 'LibrariesPage'),
});

const profileSettingsRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/settings/profiles',
  component: lazyRouteComponent(
    () => import('@/features/profiles/ProfileSettingsPage'),
    'ProfileSettingsPage',
  ),
});

const accessRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/admin/access',
  component: lazyRouteComponent(() => import('@/features/admin/AccessPage'), 'AccessPage'),
});

const playRoute = createRoute({
  getParentRoute: () => activeRoute,
  path: '/play/$itemId',
  // `file` picks a version of the title (B-5); anything else is dropped.
  validateSearch: (search: Record<string, unknown>): { file?: string } =>
    typeof search.file === 'string' && search.file !== '' ? { file: search.file } : {},
  component: lazyRouteComponent(() => import('@/features/player/PlayerPage'), 'PlayerPage'),
});

// Home and detail are in the main bundle (LCP, P-10); the form pages (setup, login, admin) and the
// player load on demand, and with them react-aria.
export const routeTree = rootRoute.addChildren([
  setupRoute,
  loginRoute,
  inviteRoute,
  authedRoute.addChildren([
    profilesRoute,
    activeRoute.addChildren([
      chromeRoute.addChildren([
        homeRoute,
        itemRoute,
        personRoute,
        librariesRoute,
        accessRoute,
        profileSettingsRoute,
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
