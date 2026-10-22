import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeAll } from 'vitest';
import { setLanguage } from '@/i18n';
import { loadSpatialNavigation } from '@/ui';

// jsdom does not implement scrolling; the router restores scroll positions on navigation.
window.scrollTo = () => undefined;
Element.prototype.scrollIntoView = () => undefined;

await loadSpatialNavigation();

// Every page of the app is reached through lazyRouteComponent(() => import(...)) in the router, so
// the first render in a worker waits for Vite to transform that module graph. That wait falls
// inside findBy's one-second window and has nothing to do with what a test asserts: AdminHomePage
// went red that way on a documentation-only commit (run 39767863551) and ProfileLimits on the
// commit after it (run 39837140392), both with a body that had not been drawn yet. Resolving the
// page modules once moves the transform out of every assertion window. The module registry is per
// worker, so the first test file pays and every later one gets them for free.
const lazyPages = {
  ...import.meta.glob('../features/*/*Page.tsx'),
  ...import.meta.glob('../features/admin/AdminLayout.tsx'),
};

beforeAll(async () => {
  await Promise.all(Object.values(lazyPages).map((load) => load()));
  await setLanguage('de');
});

afterEach(async () => {
  cleanup();
  await setLanguage('de');
});
