import type { QueryClient } from '@tanstack/react-query';
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  lazyRouteComponent,
  redirect,
  type RouterHistory,
} from '@tanstack/react-router';
import { LoginPage } from '@/features/auth/LoginPage';
import { HomePage } from '@/features/catalog/HomePage';
import { ItemPage } from '@/features/catalog/ItemPage';
import { AppLayout } from './AppLayout';
import { RootError, RootLayout } from './RootLayout';
import { applyProfileLanguage, sessionQuery, setupStatusQuery } from './session';

export interface RouterContext {
  queryClient: QueryClient;
}

// Guard order: setup first (nothing else works before the wizard ran), then the session.
const rootRoute = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async ({ context, location }) => {
    const status = await context.queryClient.ensureQueryData(setupStatusQuery);
    const onSetup = location.pathname === '/setup';
    if (status.setupRequired && !onSetup) throw redirect({ to: '/setup' });
    if (!status.setupRequired && onSetup) throw redirect({ to: '/' });
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
  component: LoginPage,
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

// Home, detail and login are in the main bundle (LCP, P-10); rarely used pages load on demand.
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
