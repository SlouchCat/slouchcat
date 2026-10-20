import { expect, test } from './fixtures';
import { OWNER_PROFILE } from './env';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { createDownload, offlinePlayer, PREPARE_TIMEOUT_MS, waitForComplete } from './offline';
import { H264_TITLE, itemIdByTitle } from './playback';

// C-1: the installed app starts without network in under 3 s and opens the offline library
// (the service worker answers the navigation with the precached shell, the router skips the
// server). The production build registers the worker once the page is idle
// (app/serviceWorker.ts); with clientsClaim it controls the open page right away.
//
// Chromium only (playwright.config.ts): in Firefox, setOffline does not reach the requests the
// service worker makes itself (a probe got 200 from /api/v1/auth/session in "flight mode"), so
// the app finds its session there and opens the start page; cutting the API with a route
// instead makes Firefox bypass the worker for the page's own requests (NS_ERROR_OFFLINE). Neither
// is flight mode. That the worker controls the page in Firefox too, and that the offline pages
// work from its precache, show prepareOffline and the offline specs that run in both browsers.

const COLD_START_MS = 3000;

test('cold start in flight mode: the offline library in under 3 s, by keyboard', async ({
  page,
  context,
  owner,
}) => {
  test.setTimeout(240_000);
  await page.goto('/');
  // The worker takes over the page it was registered from (clientsClaim, W11).
  await page.waitForFunction(() => navigator.serviceWorker?.controller != null, undefined, {
    timeout: 30_000,
  });

  await createDownload(page, await itemIdByTitle(owner, H264_TITLE));
  await waitForComplete(page);

  await context.setOffline(true);
  const started = Date.now();
  await page.goto('/');
  const heading = page.getByRole('heading', { level: 1, name: 'Offline-Bibliothek' });
  await expect(heading).toBeVisible({ timeout: COLD_START_MS * 2 });
  const elapsed = Date.now() - started;
  test.info().annotations.push({ type: 'cold-start-ms', description: String(elapsed) });
  expect(elapsed, `offline cold start took ${elapsed} ms`).toBeLessThan(COLD_START_MS);
  await expect(page).toHaveURL(/\/offline$/);
  await expect(page.getByText('Offline', { exact: true })).toBeVisible();

  // The profile tiles by arrows and Enter; the main profile has no PIN.
  const tile = page.getByRole('button', { name: new RegExp(OWNER_PROFILE) });
  await expectFocusOn(page, tile);
  await page.keyboard.press('ArrowRight');
  await expectVisibleFocus(page);
  await tile.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { name: `Downloads von ${OWNER_PROFILE}` })).toBeVisible();
  const card = page
    .getByRole('list', { name: 'Heruntergeladene Titel' })
    .getByRole('link', { name: new RegExp(H264_TITLE) });
  await expectFocusOn(page, card);

  // Tab reaches "Profil wechseln" and the cards; no focus trap.
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Profil wechseln' }), 10);
  await tabTo(page, card, 5);
  await expectNoFocusTrap(page);

  // Enter plays it offline; Escape returns to the library, Escape again to the profiles.
  await card.focus();
  await page.keyboard.press('Enter');
  await expect(offlinePlayer(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/offline$/);
  await expect(page.getByRole('heading', { name: `Downloads von ${OWNER_PROFILE}` })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('list', { name: 'Profile' })).toBeVisible();

  // Back online the library offers the way back into the app.
  await context.setOffline(false);
  await page.getByRole('link', { name: 'Zur App' }).click();
  await expect(page).toHaveURL(/\/$/);
});

test('cold start in flight mode to the offline library, three times (C-1)', async ({
  page,
  context,
  owner,
}) => {
  test.setTimeout(PREPARE_TIMEOUT_MS * 2);
  await page.goto('/');
  await page.waitForFunction(() => navigator.serviceWorker?.controller != null, undefined, {
    timeout: 30_000,
  });
  await createDownload(page, await itemIdByTitle(owner, H264_TITLE));
  await waitForComplete(page);
  await context.setOffline(true);
  const runs: number[] = [];
  for (let run = 0; run < 3; run += 1) {
    // A new tab of the installed app each time.
    const tab = await context.newPage();
    const started = Date.now();
    await tab.goto('/');
    await expect(tab.getByRole('heading', { level: 1, name: 'Offline-Bibliothek' })).toBeVisible({
      timeout: COLD_START_MS * 2,
    });
    runs.push(Date.now() - started);
    await tab.close();
  }
  test.info().annotations.push({ type: 'cold-start-ms', description: JSON.stringify(runs) });
  console.log(`MEASURE ${test.info().project.name} cold-start-ms ${JSON.stringify({ runs })}`);
  for (const ms of runs) expect(ms, `cold start took ${ms} ms`).toBeLessThan(COLD_START_MS);
  await context.setOffline(false);
});
