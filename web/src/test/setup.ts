import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeAll } from 'vitest';
import i18n from '@/i18n';
import { initSpatialNavigation } from '@/ui';

// jsdom does not implement scrolling; the router restores scroll positions on navigation.
window.scrollTo = () => undefined;
Element.prototype.scrollIntoView = () => undefined;

initSpatialNavigation();

beforeAll(async () => {
  await i18n.changeLanguage('de');
});

afterEach(async () => {
  cleanup();
  await i18n.changeLanguage('de');
});
