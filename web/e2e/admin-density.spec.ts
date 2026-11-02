import type { Locator, Page } from '@playwright/test';
import { expect, test } from './fixtures';

// The compact density of the administration (concept 12, 5.1 and 5.2; ADR 0020; K12-8): with a
// mouse from 768 px the content of /admin/* and the dialogs opened from it use 32 px controls and
// a 24 px title. The app header, the administration's sidebar and /settings/* keep today's size,
// and so do touch devices and phones (44 px).

const DESKTOP = { width: 1440, height: 900 };

const height = async (locator: Locator) => Math.round((await locator.boundingBox())?.height ?? 0);
/**
 * The layout height without transforms: a dialog rises from scale(0.98) for 240 ms
 * (Dialog.module.css `rise`), and boundingBox() measures the scaled box (31.36 px).
 */
const layoutHeight = (locator: Locator) =>
  locator.evaluate((element) => Math.round(parseFloat(getComputedStyle(element).height)));
const fontSize = (locator: Locator) =>
  locator.evaluate((element) => getComputedStyle(element).fontSize);

/** The h1's text (Text variant pageTitle inside the h1 of AdminPage). */
const title = (page: Page) => page.getByRole('heading', { level: 1 }).locator('span');
const help = (page: Page) => page.getByRole('button', { name: 'Hilfe zu dieser Seite' });
const menu = (page: Page) => page.getByRole('button', { name: 'Menü', exact: true });

async function openJobs(page: Page) {
  await page.goto('/admin/jobs');
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
}

test.describe('with a mouse from 768 px', () => {
  test.use({ viewport: DESKTOP });

  test('the content of /admin is compact; header and sidebar keep 44 px', async ({ page }) => {
    await openJobs(page);
    expect(await page.evaluate(() => matchMedia('(pointer: fine)').matches)).toBe(true);
    await expect(page.locator('html')).toHaveAttribute('data-shell', 'admin');

    await expect.poll(() => height(help(page))).toBe(32);
    expect(await fontSize(title(page))).toBe('24px');
    // The side margin of the content: 32 px instead of 64 px from 1440 px.
    expect(
      await page
        .locator('#content')
        .evaluate((element) => getComputedStyle(element).getPropertyValue('--page-gutter').trim()),
    ).toBe('32px');
    // The h1 is as high as its 24 px title, not the 48 px line of an h1's own 2em.
    expect(await height(page.getByRole('heading', { level: 1 }))).toBeLessThanOrEqual(32);

    // Outside #content: the app header and the administration's sidebar.
    expect(await height(menu(page))).toBe(44);
    const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
    const entry = sidebar.getByRole('link', { name: /^Hintergrundjobs(,|$)/ });
    expect(await height(entry)).toBeGreaterThanOrEqual(44);
  });

  test('running text is 14 px in the content of /admin', async ({ page }) => {
    await page.goto('/admin/spotlight');
    await expect(page.getByRole('heading', { level: 1, name: 'Spotlight' })).toBeVisible();
    const hint = page.getByText(/^Wo die Reihe auf der Startseite steht/);
    await expect(hint).toBeVisible();
    expect(await fontSize(hint)).toBe('14px');
  });

  test('/settings/profiles keeps today’s size', async ({ page }) => {
    await page.goto('/settings/profiles');
    await expect(page.getByRole('heading', { level: 1, name: 'Profile' })).toBeVisible();
    await expect(page.locator('html')).not.toHaveAttribute('data-shell', 'admin');
    expect(await height(page.getByRole('button', { name: 'Profil hinzufügen' }))).toBe(44);
    expect(await fontSize(title(page))).toBe('60px');
  });

  test('leaving /admin in the app drops the compact density; coming back restores it', async ({
    page,
  }) => {
    await openJobs(page);
    await expect.poll(() => height(help(page))).toBe(32);

    // In the app, without a new page load: the menu's drawer links to the own profiles.
    await menu(page).click();
    await page
      .getByRole('dialog', { name: 'Menü' })
      .getByRole('link', { name: 'Profile', exact: true })
      .click();
    await expect(page).toHaveURL(/\/settings\/profiles$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Profile' })).toBeVisible();
    await expect(page.locator('html')).not.toHaveAttribute('data-shell');
    await expect
      .poll(() => height(page.getByRole('button', { name: 'Profil hinzufügen' })))
      .toBe(44);

    await page.goBack();
    await expect(page).toHaveURL(/\/admin\/jobs$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('data-shell', 'admin');
    await expect.poll(() => height(help(page))).toBe(32);
  });

  test('a dialog opened from an administration page is compact', async ({ page }) => {
    await page.goto('/admin/accounts');
    await expect(page.getByRole('heading', { level: 1, name: 'Konten' })).toBeVisible();
    await page.getByRole('button', { name: 'Konto einladen' }).click();
    const dialog = page.getByRole('dialog', { name: 'Konto einladen' });
    await expect(dialog).toBeVisible();
    const create = dialog.getByRole('button', { name: 'Einladung erstellen' });
    expect(await layoutHeight(create)).toBe(32);
    expect(await fontSize(create)).toBe('14px');
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
  });

  test('the question before leaving with unsaved input is compact', async ({ page }) => {
    await page.goto('/admin/integrations');
    await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
    await page.getByLabel('YouTube-API-Schlüssel').fill('unsaved-key');
    await page
      .getByRole('navigation', { name: 'Verwaltung' })
      .getByRole('link', { name: /^Hintergrundjobs(,|$)/ })
      .click();
    const question = page.getByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await expect(question).toBeVisible();
    const stay = question.getByRole('button', { name: 'Auf der Seite bleiben' });
    expect(await layoutHeight(stay)).toBe(32);
    await question.getByRole('button', { name: 'Verwerfen und verlassen' }).click();
    await expect(page).toHaveURL(/\/admin\/jobs$/);
  });
});

test.describe('on a phone', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('controls keep 44 px below 768 px', async ({ page }) => {
    await openJobs(page);
    await expect.poll(() => height(help(page))).toBe(44);
  });
});

test.describe('on a tablet with touch', () => {
  // A touch screen makes the primary pointer coarse in Chromium and Firefox.
  test.use({ viewport: { width: 1024, height: 768 }, hasTouch: true });

  test('controls keep 44 px with a coarse pointer from 768 px', async ({ page }) => {
    await openJobs(page);
    expect(await page.evaluate(() => matchMedia('(pointer: coarse)').matches)).toBe(true);
    await expect.poll(() => height(help(page))).toBe(44);
  });
});
