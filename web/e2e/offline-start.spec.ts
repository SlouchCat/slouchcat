import { expect, test } from './fixtures';
import { OWNER_PROFILE } from './env';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { createDownload, offlinePlayer, waitForComplete } from './offline';
import { H264_TITLE, itemIdByTitle } from './playback';

// C-1: the installed app starts without network in under 3 s and opens the offline library
// (the service worker answers the navigation with the precached shell, the router skips the
// server). Only where a service worker controls the page: the production build registers it
// once the page is idle (app/serviceWorker.ts).

const COLD_START_MS = 3000;

test('cold start in flight mode: the offline library in under 3 s, by keyboard', async ({
  page,
  context,
  owner,
}) => {
  test.setTimeout(240_000);
  await page.goto('/');
  const controlled = await page
    .waitForFunction(() => navigator.serviceWorker?.controller != null, undefined, {
      timeout: 30_000,
    })
    .then(() => true)
    .catch(() => false);
  test.skip(!controlled, 'no service worker controls the page in this browser');

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
