import type { Page } from '@playwright/test';
import { CSRF } from './catalog';
import { titleId } from './curation';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo, resetProgress } from './playback';

// /admin/home-rows (M7, E-9) by keyboard (rule 8): "Neu hinzugefügt" moved above "Weiterschauen"
// changes the start page's order, a hidden row is gone from it; the Trakt section shows its state
// and "Jetzt aktualisieren". ↑/↓ between the rows, Tab, Enter and Space, browser back, no trap.

/** A film the main profile has started, so its "Weiterschauen" row exists. */
const STARTED_TITLE = 'Slouchcat Test Sidecar Subtitles';

/** Headings of the start page's rows (and the hero), top to bottom. */
async function rowOrder(page: Page): Promise<string[]> {
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Neu hinzugefügt' })).toBeVisible();
  return page
    .locator('main section[aria-labelledby]')
    .evaluateAll((sections) =>
      sections.map(
        (section) =>
          document.getElementById(section.getAttribute('aria-labelledby') ?? '')?.textContent ?? '',
      ),
    );
}

test('start page rows by keyboard: reorder and hide change the start page', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const started = await titleId(owner, STARTED_TITLE);
  const progress = await owner.post(`/api/v1/items/${started}/progress`, {
    headers: CSRF,
    data: {
      positionMs: 30_000,
      durationMs: 60_000,
      playedMs: 30_000,
      seq: 1,
      state: 'paused',
      clientTime: new Date().toISOString(),
    },
  });
  expect(progress.ok(), await progress.text()).toBe(true);
  try {
    const before = await rowOrder(page);
    expect(before.indexOf('Weiterschauen')).toBeGreaterThanOrEqual(0);
    expect(before.indexOf('Weiterschauen')).toBeLessThan(before.indexOf('Neu hinzugefügt'));

    await page.goto('/admin/home-rows');
    await expect(page.getByRole('heading', { level: 1, name: 'Startseite' })).toBeVisible();
    await expect(
      page
        .getByRole('navigation', { name: 'Verwaltung' })
        .getByRole('link', { name: 'Startseite' }),
    ).toHaveAttribute('aria-current', 'page');
    const table = page.getByRole('table', { name: 'Reihen der Startseite' });
    await expect(table.getByRole('rowheader')).toHaveCount(8);

    // "Neu hinzugefügt" one up: the focus stays on the button, the row moved.
    await resetFocus(page);
    const up = table.getByRole('button', { name: 'Neu hinzugefügt nach oben' });
    await tabTo(page, up, 60);
    await page.keyboard.press('Enter');
    await expectFocusOn(page, up);
    await expect(table.getByRole('rowheader').nth(1)).toHaveText('Neu hinzugefügt');
    await expect(table.getByRole('rowheader').nth(2)).toHaveText('Weiterschauen');
    await expect(page.getByText(/Neu hinzugefügt steht jetzt an Stelle 2/)).toBeVisible();

    // ↓ to the next row ("Weiterschauen" now), ↑ back.
    await arrowTo(page, table.getByRole('button', { name: 'Weiterschauen nach oben' }));
    await expectVisibleFocus(page);
    await arrowTo(page, up, 'ArrowUp');

    const save = page.getByRole('button', { name: 'Speichern' });
    const stored = () =>
      page.waitForResponse(
        (response) =>
          response.request().method() === 'PUT' && response.url().endsWith('/admin/home-rows'),
      );
    await tabTo(page, save, 40);
    let saved = stored();
    await page.keyboard.press('Enter');
    expect((await saved).status()).toBe(200);
    await expect(page.getByText('Die Startseite ist gespeichert.')).toBeVisible();
    await expectVisibleFocus(page);

    const after = await rowOrder(page);
    expect(after.indexOf('Neu hinzugefügt')).toBeLessThan(after.indexOf('Weiterschauen'));
    await page.goBack();
    await expect(page).toHaveURL(/\/admin\/home-rows$/);

    // "Weiterschauen ausblenden" with Space: gone from the start page.
    const hide = table.getByRole('checkbox', { name: 'Weiterschauen ausblenden' });
    await resetFocus(page);
    await tabTo(page, hide, 60);
    await page.keyboard.press('Space');
    await expect(hide).toBeChecked();
    await tabTo(page, save, 40);
    saved = stored();
    await page.keyboard.press('Enter');
    expect((await saved).status()).toBe(200);
    expect(await rowOrder(page)).not.toContain('Weiterschauen');
    await expect(page.getByRole('region', { name: 'Weiterschauen' })).toHaveCount(0);
  } finally {
    await resetProgress(owner, started);
  }
});

test('Trakt section: state and "Jetzt aktualisieren" by keyboard', async ({ page, owner }) => {
  const response = await owner.get('/api/v1/admin/trakt');
  expect(response.status(), await response.text()).toBe(200);
  // start-server.mjs always sets the client id of the Trakt mock; the refusal without one is
  // covered by HomeRowsPage.test.tsx.
  const status = (await response.json()) as { configured: boolean };
  expect(status.configured, 'SLOUCHCAT_TRAKT_CLIENT_ID reaches the server').toBe(true);
  await page.goto('/admin/home-rows');
  const section = page.getByRole('region', { name: 'Trakt' });
  await expect(section.getByText('Ja', { exact: true })).toBeVisible();
  const refresh = section.getByRole('button', { name: 'Jetzt aktualisieren' });
  await resetFocus(page);
  await tabTo(page, refresh, 80);
  const queued = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' && response.url().endsWith('/admin/trakt/refresh'),
  );
  await page.keyboard.press('Enter');
  expect((await queued).status()).toBe(202);
  await expect(section.getByText('Die Trakt-Listen werden abgerufen.')).toBeVisible();
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page);
});
