import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  createHashHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
  useRouter,
} from '@tanstack/react-router';
import { useState } from 'react';
import { AppLink, LinkRouterProvider } from './AppLink';
import { ConfirmDialog } from './Dialog';
import { useUnsavedChanges } from './useUnsavedChanges';

/** A page with unsaved input and a link away from it, the way the administration has them. */
function DirtyPage() {
  const [value, setValue] = useState('alle 2 h');
  const unsaved = useUnsavedChanges(value !== 'alle 6 h');
  return (
    <>
      <nav aria-label="Verwaltung">
        <AppLink href="/admin/backup">Sicherung</AppLink>
      </nav>
      <input
        aria-label="Zeitplan"
        value={value}
        onChange={(event) => setValue(event.target.value)}
      />
      <ConfirmDialog {...unsaved.dialog} />
    </>
  );
}

/** The app's root layout: AppLink navigates through the router. */
function Layout() {
  const router = useRouter();
  return (
    <LinkRouterProvider navigate={(href) => void router.navigate({ href })}>
      <Outlet />
    </LinkRouterProvider>
  );
}

/**
 * The router lives in the URL's hash, so the browser's Back button reaches the guard like in the
 * app: `#/` is a page that links to the guarded one, no hash opens the guarded page directly.
 */
function GuardedPage() {
  const [router] = useState(() => {
    const history = createHashHistory();
    if (!window.location.hash) history.replace('/admin/jobs');
    const root = createRootRoute({ component: Layout });
    const start = createRoute({
      getParentRoute: () => root,
      path: '/',
      component: () => <AppLink href="/admin/jobs">Hintergrundjobs</AppLink>,
    });
    const jobs = createRoute({
      getParentRoute: () => root,
      path: '/admin/jobs',
      component: DirtyPage,
    });
    const backup = createRoute({
      getParentRoute: () => root,
      path: '/admin/backup',
      component: () => <h1>Sicherung</h1>,
    });
    return createRouter({ routeTree: root.addChildren([start, jobs, backup]), history });
  });
  return <RouterProvider router={router as never} />;
}

const meta = {
  title: 'UI/useUnsavedChanges',
  component: GuardedPage,
} satisfies Meta<typeof GuardedPage>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A page with unsaved input; the link leads away from it. */
export const Dirty: Story = {};

/**
 * Enter on the link asks first (concept 12, S-13); the focus starts on «Auf der Seite bleiben», so
 * a second Enter by habit keeps the input.
 */
export const Asking: Story = { tags: ['press:Tab+Enter'] };
