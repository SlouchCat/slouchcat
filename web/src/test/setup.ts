import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeAll } from 'vitest';
import { setLanguage } from '@/i18n';
import { loadSpatialNavigation } from '@/ui';

// jsdom does not implement scrolling; the router restores scroll positions on navigation.
window.scrollTo = () => undefined;
Element.prototype.scrollIntoView = () => undefined;

await loadSpatialNavigation();

beforeAll(async () => {
  await setLanguage('de');
});

afterEach(async () => {
  cleanup();
  await setLanguage('de');
});
