import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { destroyAnnouncer } from 'react-aria/private/live-announcer/LiveAnnouncer';
import { afterEach, beforeAll } from 'vitest';
import { setLanguage } from '@/i18n';
import { loadSpatialNavigation } from '@/ui';

// @testing-library/react marks the React act environment only where Jest's globals exist
// (beforeAll/afterAll); Vitest runs without globals here. Without the flag react-aria's live
// announcer defers its first announcement by 100 ms (a Safari workaround), and that timer can fire
// after Vitest has torn down jsdom: "ReferenceError: document is not defined", an unhandled error
// that failed the web job on LoginPage.test.tsx (run 41034366273). With the flag the announcer
// writes at once, as it does under Jest.
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// jsdom does not implement scrolling; the router restores scroll positions on navigation.
window.scrollTo = () => undefined;
Element.prototype.scrollIntoView = () => undefined;

await loadSpatialNavigation();

// The page modules the router loads lazily are resolved in src/test/api.tsx, which every test of
// the whole app imports (renderApp); a test that renders a component alone does not pay for them.

beforeAll(async () => {
  await setLanguage('de');
});

afterEach(async () => {
  cleanup();
  // The flag above is not always set when the announcer starts: @testing-library/react switches
  // it off while findBy and waitFor run, so an announcement made then still takes the 100-ms
  // path. When the file ended inside those 100 ms, the timer fired after jsdom was gone and its
  // document.createElement threw "document is not defined" (LoginPage.test.tsx, the flake the flag
  // did not end). Destroying the announcer after every test leaves the timer nothing to write to
  // (it checks the announcer first) and gives the next test a fresh one.
  destroyAnnouncer();
  await setLanguage('de');
});
