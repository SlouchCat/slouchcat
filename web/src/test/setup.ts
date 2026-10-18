import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeAll } from 'vitest';
import i18n from '@/i18n';

// jsdom does not implement scrolling; the router restores scroll positions on navigation.
window.scrollTo = () => undefined;

beforeAll(async () => {
  await i18n.changeLanguage('de');
});

afterEach(() => {
  cleanup();
});
