import { expect, test } from './fixtures';

// C-1 without an answering server (W10): the session request never comes back (a dead port
// forward drops the packets), and the app opens the offline library within 3 s. Both browsers;
// the cold start in flight mode is in offline-cold-start.spec.ts.

const COLD_START_MS = 3000;

test.describe('a server that never answers', () => {
  // The route below must see the session request; a service worker would make it on its own.
  test.use({ serviceWorkers: 'block' });

  test('opens the offline library within 3 s (W10, C-1)', async ({ page }) => {
    await page.route('**/api/v1/auth/session', () => {
      // Never answered: a server behind a dead port forward drops the packets.
    });
    const started = Date.now();
    await page.goto('/');
    const heading = page.getByRole('heading', { level: 1, name: 'Offline-Bibliothek' });
    await expect(heading).toBeVisible({ timeout: COLD_START_MS * 2 });
    const elapsed = Date.now() - started;
    test.info().annotations.push({ type: 'unreachable-start-ms', description: String(elapsed) });
    expect(elapsed, `start without an answering server took ${elapsed} ms`).toBeLessThan(
      COLD_START_MS,
    );
    await expect(page).toHaveURL(/\/offline$/);
    // Online the library offers the way back into the app.
    await expect(page.getByRole('link', { name: 'Zur App' })).toBeVisible();
    await page.unrouteAll({ behavior: 'ignoreErrors' });
  });
});
