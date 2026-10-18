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
import { AppLayout } from './AppLayout';
import { paths } from './paths';
import { RootError, RootLayout } from './RootLayout';
import { applyProfileLanguage, sessionQuery, setupStatusQuery } from './session';

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
  },
  component: lazyRouteComponent(() => import('@/features/auth/LoginPage'), 'LoginPage'),
});

/** Everything behind the login. */
const authedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: '_authed',
  beforeLoad: async ({ context }) => {
    const session = await context.queryClient.ensureQueryData(sessionQuery);
    if (!session) throw redirect({ to: '/login' });
    await applyProfileLanguage(session);
    return { session };
  },
});

/** Pages with the app header; the player runs full screen without it. */
const chromeRoute = createRoute({
  getParentRoute: () => authedRoute,
  id: '_chrome',
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

const librariesRoute = createRoute({
  getParentRoute: () => chromeRoute,
  path: '/admin/libraries',
  component: lazyRouteComponent(() => import('@/features/admin/LibrariesPage'), 'LibrariesPage'),
});

const playRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/play/$itemId',
  component: lazyRouteComponent(() => import('@/features/player/PlayerPage'), 'PlayerPage'),
});

// Home and detail are in the main bundle (LCP, P-10); the form pages (setup, login, admin) and the
// player load on demand, and with them react-aria.
export const routeTree = rootRoute.addChildren([
  setupRoute,
  loginRoute,
  authedRoute.addChildren([
    chromeRoute.addChildren([homeRoute, itemRoute, librariesRoute]),
    playRoute,
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
