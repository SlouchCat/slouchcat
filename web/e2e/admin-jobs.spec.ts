import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// /admin/jobs (concept 09, ADR 0017) by keyboard (rule 8): the page is reached through the admin
// navigation, "Läuft gerade" and the sweep's card are walked with Tab, "Jetzt ausführen" of the
// sweep queues a run that ends in "Erledigt", "Pausieren" and "Fortsetzen" switch the card's
// chip and the banner, the dialog "Läufe von …" walks its tabs with the arrow keys and closes
// with Escape with the focus back on its button, the dialog "Zeitplan von …" sets the sweep to
// every 2 hours and the card shows it; browser back, visible focus, no trap. The sweep ends in
// seconds on the e2e server and queues nothing else. A pause survives a restart (F-6) and the
// server is shared, so the test resumes every paused type and restores the sweep's schedule
// afterwards. The confirmations of "Wartende abbrechen" need a waiting run, which no API creates
// while the type is paused; the component test JobsPage.test.tsx covers them.

/** Eighteen cards with up to six buttons each: Tab needs room to walk them. */
const TAB_MAX = 200;

const SWEEP = 'library.sweep';
const SWEEP_NAME = 'Aufräumen fehlender Titel';

async function resumeAll(api: APIRequestContext): Promise<void> {
  const response = await api.get('/api/v1/admin/job-types');
  expect(response.status(), await response.text()).toBe(200);
  const overview = (await response.json()) as { types: { type: string; paused: boolean }[] };
  for (const info of overview.types.filter((entry) => entry.paused)) {
    const resumed = await api.post(`/api/v1/admin/job-types/${info.type}/resume`, {
      headers: CSRF,
    });
    expect(resumed.status(), await resumed.text()).toBe(200);
  }
}

async function restoreSweep(api: APIRequestContext): Promise<void> {
  const response = await api.patch(`/api/v1/admin/job-types/${SWEEP}/schedule`, {
    headers: CSRF,
    data: { everySeconds: 3600, clearWindow: true, enabled: true },
  });
  expect(response.status(), await response.text()).toBe(200);
}

/** Presses → in the admin navigation until `name` has the focus. */
async function navTo(page: Page, name: string): Promise<void> {
  const link = page
    .getByRole('navigation', { name: 'Verwaltung' })
    .getByRole('link', { name, exact: true });
  for (let presses = 0; presses < 20; presses += 1) {
    if (await link.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowRight');
  }
  await expectFocusOn(page, link);
}

test.afterEach(async ({ owner }) => {
  await resumeAll(owner);
  await restoreSweep(owner);
});

test('background jobs by keyboard: run, pause, resume, runs and schedule', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  await resumeAll(owner);
  await restoreSweep(owner);

  // From the overview through the admin navigation; the dashboard links the page.
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Alle Jobs' })).toHaveAttribute(
    'href',
    '/admin/jobs',
  );
  const nav = page.getByRole('navigation', { name: 'Verwaltung' });
  await resetFocus(page);
  await tabTo(page, nav.getByRole('link', { name: 'Übersicht' }), TAB_MAX);
  await navTo(page, 'Hintergrundjobs');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
  await expectVisibleFocus(page);

  // "Läuft gerade" and the sweep's card with its code.
  await expect(page.getByRole('region', { name: 'Läuft gerade' })).toBeVisible();
  const sweep = page.getByRole('article', { name: new RegExp(SWEEP_NAME) });
  await expect(sweep).toBeVisible();
  await expect(sweep.getByText(SWEEP, { exact: true })).toBeVisible();

  // "Jetzt ausführen" with Enter: one run is queued and ends in "Erledigt".
  const runButton = sweep.getByRole('button', {
    name: `${SWEEP_NAME} jetzt ausführen`,
    exact: true,
  });
  await tabTo(page, runButton, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status')).toContainText(`${SWEEP_NAME}: 1 Lauf eingereiht.`);
  await expect
    .poll(
      async () => {
        const response = await owner.get(`/api/v1/admin/jobs?type=${SWEEP}&state=done&limit=5`);
        return ((await response.json()) as { total: number }).total;
      },
      { timeout: 30_000 },
    )
    .toBeGreaterThan(0);

  // "Pausieren" with Enter: chip and banner say so, the button becomes "Fortsetzen", a manual
  // run is refused (409 job_type_paused) and the run button is gone; "Fortsetzen" ends it.
  await tabTo(
    page,
    sweep.getByRole('button', { name: `${SWEEP_NAME} pausieren`, exact: true }),
    TAB_MAX,
  );
  await page.keyboard.press('Enter');
  await expect(sweep.getByText('pausiert', { exact: true }).first()).toBeVisible(); // chip and next run
  await expect(page.getByText('1 Job pausiert')).toBeVisible();
  await expect(runButton).toHaveCount(0);
  const refused = await owner.post(`/api/v1/admin/job-types/${SWEEP}/run`, { headers: CSRF });
  expect(refused.status(), await refused.text()).toBe(409);
  const resumeButton = sweep.getByRole('button', { name: `${SWEEP_NAME} fortsetzen`, exact: true });
  await tabTo(page, resumeButton, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(sweep.getByText('pausiert', { exact: true })).toHaveCount(0);
  await expect(page.getByText('1 Job pausiert')).toHaveCount(0);
  await expect(runButton).toBeVisible();

  // The dialog "Läufe von …": tabs with the arrow keys, Escape closes, focus returns.
  const runsButton = sweep.getByRole('button', { name: `Läufe von ${SWEEP_NAME}`, exact: true });
  await tabTo(page, runsButton, TAB_MAX);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: `Läufe von ${SWEEP_NAME}` });
  await expect(dialog).toBeVisible();
  const tabs = dialog.getByRole('tablist', { name: 'Läufe nach Zustand' });
  await expect(tabs.getByRole('tab', { selected: true })).toHaveText('Wartend');
  await tabTo(page, tabs.getByRole('tab', { selected: true }), 5); // the first Tab stop of the dialog
  await page.keyboard.press('ArrowRight');
  await expect(tabs.getByRole('tab', { selected: true })).toHaveText('Geplant');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(tabs.getByRole('tab', { selected: true })).toHaveText('Erledigt');
  await expect(dialog.getByRole('table', { name: `Läufe von ${SWEEP_NAME}` })).toBeVisible();
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, runsButton);

  // The dialog "Zeitplan von …": Tab through switch and interval, "alle 2 Stunden", Enter saves.
  const scheduleButton = sweep.getByRole('button', {
    name: `Zeitplan von ${SWEEP_NAME}`,
    exact: true,
  });
  await tabTo(page, scheduleButton, TAB_MAX);
  await page.keyboard.press('Enter');
  const schedule = page.getByRole('dialog', { name: `Zeitplan von ${SWEEP_NAME}` });
  await expect(schedule).toBeVisible();
  await tabTo(page, schedule.getByRole('switch', { name: 'Zeitplan eingeschaltet' }), 5);
  await page.keyboard.press('Tab');
  const interval = schedule.getByRole('combobox', { name: 'Takt' });
  await expectFocusOn(page, interval);
  await expect(interval).toHaveValue('3600');
  await interval.selectOption('7200');
  await expect(interval).toHaveValue('7200');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, schedule.getByRole('switch', { name: 'Nur in einem Zeitfenster' }));
  await tabTo(page, schedule.getByRole('button', { name: 'Speichern', exact: true }), TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(schedule).toHaveCount(0);
  await expect(page.getByRole('status')).toContainText(`Zeitplan von ${SWEEP_NAME} gespeichert.`);
  await expect(sweep.getByText(/alle 2 h/)).toBeVisible();
  await expectFocusOn(page, scheduleButton);
  await expectNoFocusTrap(page);

  // Browser back returns to the overview.
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
});
