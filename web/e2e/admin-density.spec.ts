import type { Locator, Page } from '@playwright/test';
import { expect, test } from './fixtures';

// The compact density of the administration (concept 12, 5.1 and 5.2; ADR 0020; K12-8): with a
// mouse from 768 px the content of /admin/* and the dialogs opened from it use 32 px controls.
// The app header, the administration's sidebar and /settings/* keep today's size, and so do touch
// devices and phones (44 px). Since the redesign (build plan 2.6) a page in the admin frame has a
// 30 px title over the area's tabs; the overview keeps the 24 px title until step 2.

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

/** The h1's text (Text variant pageTitle inside the h1 of AdminPage outside the admin frame). */
const title = (page: Page) => page.getByRole('heading', { level: 1 }).locator('span');
/**
 * A control of the content: the current tab of the area (build plan 2.5), a control-height link.
 * It took the place of the help button, which the admin frame no longer has (build plan 2.6).
 */
const control = (page: Page) =>
  page
    .getByRole('navigation', { name: /^Seiten im Bereich / })
    .getByRole('link', { name: /^Hintergrundjobs(,|$)/ });
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

    await expect.poll(() => height(control(page))).toBe(32);
    // The admin frame's title (build plan 2.6): 30 px from 768 px.
    expect(await fontSize(page.getByRole('heading', { level: 1 }))).toBe('30px');
    // The side margin of the content: 32 px instead of 64 px from 1440 px.
    expect(
      await page
        .locator('#content')
        .evaluate((element) => getComputedStyle(element).getPropertyValue('--page-gutter').trim()),
    ).toBe('32px');
    // The h1 is as high as its 30 px title (line height 1.1), not an h1's own 2em line.
    expect(await height(page.getByRole('heading', { level: 1 }))).toBeLessThanOrEqual(34);

    // Outside #content: the app header and the administration's sidebar.
    expect(await height(menu(page))).toBe(44);
    const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
    const entry = sidebar.getByRole('link', { name: /^Automatisierung(,|$)/ });
    expect(await height(entry)).toBeGreaterThanOrEqual(44);
  });

  test('the overview has the 30 px page title of the admin frame', async ({ page }) => {
    await page.goto('/admin');
    const heading = page.getByRole('heading', { level: 1, name: 'Alles im Blick' });
    await expect(heading).toBeVisible();
    expect(await fontSize(heading)).toBe('30px');
  });

  test('a table row is 40 px high (S-5)', async ({ page }) => {
    // A single-line row is 40 px: 24 px fold-out button and state chip plus the compact cell
    // padding. Since concept 15 (P-2) a job row carries a description of up to two lines, so on
    // this 1440 px screen the row is taller by design. At 1920 px the first job type's name and
    // description fit on one line each, and there the row must be exactly 40 px again.
    await page.setViewportSize({ width: 1920, height: 1080 });
    await openJobs(page);
    // The first job type of the selected tab: its fold-out button and state chip are 24 px.
    const row = page.getByRole('tabpanel').getByRole('row').nth(1);
    await expect(row).toBeVisible();
    await expect.poll(() => height(row)).toBe(40);
  });

  test('a job row says what the job does in its own column, at most two lines cut with «…» (concept 15, P-2)', async ({
    page,
  }) => {
    const SENTENCE = 'Sucht in allen Bibliotheken neue, geänderte und fehlende Dateien.';
    const measure = (locator: Locator) =>
      locator.evaluate((element) => {
        const style = getComputedStyle(element);
        return {
          clamp: style.getPropertyValue('-webkit-line-clamp'),
          overflow: style.overflow,
          lines: Math.round(element.clientHeight / parseFloat(style.lineHeight)),
          cut: element.scrollHeight > element.clientHeight,
        };
      });
    await openJobs(page);
    const row = page.getByRole('tabpanel').getByRole('row').nth(1);
    const description = row.getByRole('cell').first().getByText(SENTENCE);
    await expect(description).toBeVisible();
    await expect(description).toHaveAttribute('title', SENTENCE);
    expect(await fontSize(description)).toBe('14px');
    // At 1440 px the sentence wraps once and shows in full on two lines.
    expect(await measure(description)).toEqual({
      clamp: '2',
      overflow: 'hidden',
      lines: 2,
      cut: false,
    });
    // At 768 px the column is narrow: two lines, the rest cut with «…» (the title holds it).
    await page.setViewportSize({ width: 768, height: 900 });
    await expect.poll(async () => (await measure(description)).cut).toBe(true);
    expect((await measure(description)).lines).toBe(2);
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
    await expect.poll(() => height(control(page))).toBe(32);

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
    await expect.poll(() => height(control(page))).toBe(32);
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
    // The provider cards start folded (concept 20, P-12).
    const youtube = page.getByRole('region', { name: 'YouTube', exact: true });
    await youtube.getByRole('button', { name: 'Einrichten: YouTube' }).click();
    await youtube.getByRole('textbox', { name: 'Schlüssel', exact: true }).fill('unsaved-key');
    await page
      .getByRole('navigation', { name: 'Seiten im Bereich Automatisierung' })
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
    await expect.poll(() => height(control(page))).toBe(44);
  });
});

test.describe('on a tablet with touch', () => {
  // A touch screen makes the primary pointer coarse in Chromium and Firefox.
  test.use({ viewport: { width: 1024, height: 768 }, hasTouch: true });

  test('controls keep 44 px with a coarse pointer from 768 px', async ({ page }) => {
    await openJobs(page);
    expect(await page.evaluate(() => matchMedia('(pointer: coarse)').matches)).toBe(true);
    await expect.poll(() => height(control(page))).toBe(44);
  });
});
