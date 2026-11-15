import { afterEach, describe, expect, it, vi } from 'vitest';
import { cacheLoadedLocales } from './serviceWorker';

// Concept 22 section 4.4: only de, de-DE and en are precached. On the first start the page loads
// its language before the worker controls it; cacheLoadedLocales fetches those chunks again, now
// through the worker, whose CacheFirst rule keeps them for offline starts.
describe('cacheLoadedLocales', () => {
  afterEach(() => vi.restoreAllMocks());

  it('fetches the locale chunks the page loaded, nothing else', () => {
    const origin = window.location.origin;
    vi.spyOn(performance, 'getEntriesByType').mockReturnValue([
      { name: `${origin}/assets/locale-fr.BYMPFA4M.js` },
      { name: `${origin}/assets/locale-pt-BR.Bq--6_HJ.js` },
      { name: `${origin}/assets/index-Dk3l.js` },
      { name: `${origin}/api/v1/auth/session` },
      { name: `${origin}/assets/locale-fr.json` },
    ] as PerformanceEntry[]);
    const fetched = vi.fn(() => Promise.resolve(new Response('')));
    vi.stubGlobal('fetch', fetched);
    cacheLoadedLocales();
    expect(fetched.mock.calls.map((call) => (call as unknown[])[0])).toEqual([
      `${origin}/assets/locale-fr.BYMPFA4M.js`,
      `${origin}/assets/locale-pt-BR.Bq--6_HJ.js`,
    ]);
    vi.unstubAllGlobals();
  });
});
