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

// /admin/jobs (concept 09, ADR 0017; concept 12 variant C) by keyboard (rule 8): the page is
// reached through the admin navigation, the group tabs move with the arrow keys and live in
// `?tab=`, the sweep's row says what the job does in its own column, folds out with Enter and
// opens with that sentence in full, "Jetzt ausführen" of the sweep queues a run that ends in
// "Erledigt", "Pausieren" and "Fortsetzen" switch the row's chip and the banner, the runs of the
// row walk their tabs with the arrow keys, the schedule in the row sets the sweep to every 2 hours
// and the row shows it; the menu «Weitere Aktionen» opens with Enter, moves with the arrows,
// closes with Escape onto its button and asks before «Wartende abbrechen»; the overview links the
// tab «Fehlschläge», whose failures are retried and cancelled by keyboard (stubbed, see
// the last test); browser back, visible focus, no trap. The sweep ends in seconds on
// the e2e server and queues nothing else. A pause survives a restart (F-6) and the server is
// shared, so the test resumes every paused type and restores the sweep's schedule afterwards.
// A waiting run to cancel comes from «Metadaten anwenden» (images only) while that type is
// paused: «Jetzt ausführen» refuses a paused type, the apply endpoint queues anyway, and nothing
// runs it before it is cancelled. A second one, cancelled in the row's runs, leaves its table, and
// the focus goes to the h1 (ALL-2). The keyframe index takes a time window in its row (concept 15,
// 4.3) around the server's current time, so it holds nothing back, and loses it again.

/** The sidebar, the page's head and a group's rows with an open fold-out: room to walk them. */
const TAB_MAX = 200;

const SWEEP = 'library.sweep';
const SWEEP_NAME = 'Aufräumen fehlender Titel';
const APPLY = 'metadata.apply';
const APPLY_NAME = 'Metadaten anwenden';
const PROBE = 'probe.keyframes';
const PROBE_NAME = 'Keyframe-Index (ganze Datei lesen)';

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
    data: { everySeconds: 3600, clearWindow: true, clearOnce: true, enabled: true },
  });
  expect(response.status(), await response.text()).toBe(200);
}

/** Cancels whatever of `type` still waits (a failed test must not leave a paused apply behind). */
async function cancelWaiting(api: APIRequestContext, type: string): Promise<void> {
  const response = await api.delete(`/api/v1/admin/job-types/${type}/runs?scope=queued`, {
    headers: CSRF,
  });
  expect(response.status(), await response.text()).toBe(200);
}

/** Presses ↓ in the admin sidebar until `name` has the focus. */
async function navTo(page: Page, name: string): Promise<void> {
  const link = page
    .getByRole('navigation', { name: 'Verwaltung' })
    .getByRole('link', { name: new RegExp(`^${name}(,|$)`) });
  for (let presses = 0; presses < 20; presses += 1) {
    if (await link.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowDown');
  }
  await expectFocusOn(page, link);
}

/** The fold-out of a job type's row: the cell its toggle controls. */
function foldOut(page: Page, name: string) {
  return page.locator(`td[id]`).filter({
    has: page
      .getByRole('button', { name: `${name} pausieren`, exact: true })
      .or(page.getByRole('button', { name: `${name} fortsetzen`, exact: true })),
  });
}

/** Removes the time window of the keyframe index (a failed test must not leave one behind). */
async function clearProbeWindow(api: APIRequestContext): Promise<void> {
  const response = await api.put(`/api/v1/admin/job-types/${PROBE}/window`, {
    headers: CSRF,
    data: { clearWindow: true },
  });
  expect(response.status(), await response.text()).toBe(200);
}

/** The keyframe index as the server reports it: its window and the zone of the window. */
async function probeInfo(
  api: APIRequestContext,
): Promise<{ windowTimeZone?: string; window?: { startMinute: number; endMinute: number } }> {
  const response = await api.get('/api/v1/admin/job-types');
  expect(response.status(), await response.text()).toBe(200);
  const overview = (await response.json()) as {
    types: {
      type: string;
      windowTimeZone?: string;
      window?: { startMinute: number; endMinute: number };
    }[];
  };
  const info = overview.types.find((entry) => entry.type === PROBE);
  expect(info).toBeDefined();
  return info!;
}

/** Minutes of the day as "01:00". */
const clock = (minute: number) =>
  `${String(Math.floor(minute / 60)).padStart(2, '0')}:${String(minute % 60).padStart(2, '0')}`;

test.afterEach(async ({ owner }) => {
  await cancelWaiting(owner, APPLY);
  await resumeAll(owner);
  await restoreSweep(owner);
  await clearProbeWindow(owner);
});

test('background jobs by keyboard: tabs, fold-out, run, pause, resume, runs and schedule', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  await resumeAll(owner);
  await restoreSweep(owner);

  // From the overview through the admin sidebar; the card «Aufgaben» links the page.
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  await expect(
    page
      .getByRole('region', { name: 'Aufgaben', exact: true })
      .getByRole('link', { name: 'Hintergrundjobs' }),
  ).toHaveAttribute('href', '/admin/jobs');
  const nav = page.getByRole('navigation', { name: 'Verwaltung' });
  await resetFocus(page);
  await tabTo(page, nav.getByRole('link', { name: 'Übersicht' }), TAB_MAX);
  await navTo(page, 'Hintergrundjobs');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
  await expectVisibleFocus(page);

  // «Läuft gerade» is a group that folds away; it is open.
  await expect(page.getByRole('button', { name: /^Läuft gerade/ })).toHaveAttribute(
    'aria-expanded',
    'true',
  );
  await expect(page.getByRole('list', { name: 'Arbeiter' })).toBeVisible();

  // The group tabs with the arrow keys: the selection lives in `?tab=`.
  const tabs = page.getByRole('tablist', { name: 'Jobs nach Gruppe' });
  await resetFocus(page);
  await tabTo(page, tabs.getByRole('tab', { selected: true }), TAB_MAX);
  await expect(tabs.getByRole('tab', { selected: true })).toHaveText(/^Bibliothek/);
  await page.keyboard.press('ArrowRight');
  await expect(tabs.getByRole('tab', { selected: true })).toHaveText(/^Metadaten/);
  await expect(page).toHaveURL(/\/admin\/jobs\?tab=metadata$/);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(tabs.getByRole('tab', { selected: true })).toHaveText(/^Fehlschläge/);
  await expect(page).toHaveURL(/\/admin\/jobs\?tab=failed$/);
  await expect(
    page.getByRole('heading', { level: 2, name: 'Letzte Fehlschläge (alle Arten)' }),
  ).toBeVisible();
  for (let presses = 0; presses < 4; presses += 1) await page.keyboard.press('ArrowLeft');
  await expect(tabs.getByRole('tab', { selected: true })).toHaveText(/^Bibliothek/);
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  await expectVisibleFocus(page);

  // The sweep's row says what the job does in the column after its name (concept 15, P-2); the
  // toggle keeps the name alone. The row folds out with Enter and shows its code.
  const SWEEP_DESCRIPTION = 'Entfernt Titel, deren Dateien länger als die Schonfrist fehlen.';
  await expect(
    page
      .getByRole('row', { name: new RegExp(`^${SWEEP_NAME}`) })
      .getByRole('cell')
      .first(),
  ).toHaveText(SWEEP_DESCRIPTION);
  const toggle = page.getByRole('button', { name: SWEEP_NAME, exact: true });
  await tabTo(page, toggle, 20);
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  const sweep = foldOut(page, SWEEP_NAME);
  await expect(sweep.getByText(SWEEP, { exact: true })).toBeVisible();
  // What the job does, in full at the top of the fold-out (concept 15, P-2).
  await expect(sweep.getByText(SWEEP_DESCRIPTION)).toBeVisible();

  // "Jetzt ausführen" with Enter: one run is queued and ends in "Erledigt".
  const runButton = sweep.getByRole('button', {
    name: `${SWEEP_NAME} jetzt ausführen`,
    exact: true,
  });
  await resetFocus(page); // Tab from the top: Firefox keeps the start where a replaced button stood and does not wrap
  await tabTo(page, runButton, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status').first()).toContainText(`${SWEEP_NAME}: 1 Lauf eingereiht.`);
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
  const row = page.getByRole('row', { name: new RegExp(`^${SWEEP_NAME}`) });
  await resetFocus(page); // Tab from the top: Firefox keeps the start where a replaced button stood and does not wrap
  await tabTo(
    page,
    sweep.getByRole('button', { name: `${SWEEP_NAME} pausieren`, exact: true }),
    TAB_MAX,
  );
  await page.keyboard.press('Enter');
  await expect(row.getByText('pausiert', { exact: true }).first()).toBeVisible(); // chip and next run
  await expect(page.getByText('1 Job pausiert')).toBeVisible();
  await expect(runButton).toHaveCount(0);
  const refused = await owner.post(`/api/v1/admin/job-types/${SWEEP}/run`, { headers: CSRF });
  expect(refused.status(), await refused.text()).toBe(409);
  const resumeButton = sweep.getByRole('button', { name: `${SWEEP_NAME} fortsetzen`, exact: true });
  await resetFocus(page); // Tab from the top: Firefox keeps the start where a replaced button stood and does not wrap
  await tabTo(page, resumeButton, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(row.getByText('pausiert', { exact: true })).toHaveCount(0);
  await expect(page.getByText('1 Job pausiert')).toHaveCount(0);
  await expect(runButton).toBeVisible();

  // The runs in the row: tabs with the arrow keys.
  const runs = sweep.getByRole('region', { name: `Läufe von ${SWEEP_NAME}` });
  const states = runs.getByRole('tablist', { name: 'Läufe nach Zustand' });
  await expect(states.getByRole('tab', { selected: true })).toHaveText(/^Wartend/);
  await resetFocus(page);
  await tabTo(page, states.getByRole('tab', { selected: true }), TAB_MAX);
  await page.keyboard.press('ArrowRight');
  await expect(states.getByRole('tab', { selected: true })).toHaveText(/^Geplant/);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(states.getByRole('tab', { selected: true })).toHaveText(/^Erledigt/);
  await expect(runs.getByRole('table', { name: `Läufe von ${SWEEP_NAME}` })).toBeVisible();
  await expectVisibleFocus(page);

  // The schedule in the row: Tab through switch and interval, "alle 2 Stunden", Enter saves.
  const schedule = sweep.getByRole('region', { name: `Zeitplan von ${SWEEP_NAME}` });
  await resetFocus(page);
  await tabTo(page, schedule.getByRole('switch', { name: 'Zeitplan eingeschaltet' }), TAB_MAX);
  await page.keyboard.press('Tab');
  const interval = schedule.getByRole('combobox', { name: 'Takt' });
  await expectFocusOn(page, interval);
  await expect(interval).toHaveValue('3600');
  await interval.selectOption('7200');
  await expect(interval).toHaveValue('7200');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, schedule.getByRole('switch', { name: 'Nur in einem Zeitfenster' }));
  const save = schedule.getByRole('button', { name: 'Speichern', exact: true });
  await tabTo(page, save, 10);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status').first()).toContainText(
    `Zeitplan von ${SWEEP_NAME} gespeichert.`,
  );
  await expect(row.getByText(/alle 2 h/)).toBeVisible();
  // The draft is spent: «Verwerfen» goes and the focus stays on «Speichern».
  await expect(schedule.getByRole('button', { name: 'Verwerfen' })).toHaveCount(0);
  await expectFocusOn(page, save);
  await expectNoFocusTrap(page);

  // Browser back returns to the overview.
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
});

test('background jobs by keyboard: the menu cancels waiting runs after asking; a cancelled run hands the focus to the h1 @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await resumeAll(owner);
  // A waiting «Metadaten anwenden» that nothing runs: the type is paused first.
  const paused = await owner.post(`/api/v1/admin/job-types/${APPLY}/pause`, { headers: CSRF });
  expect(paused.status(), await paused.text()).toBe(200);
  const queued = await owner.post('/api/v1/admin/metadata/apply', {
    headers: CSRF,
    data: { what: ['images'] },
  });
  expect(queued.status(), await queued.text()).toBe(202);

  await page.goto('/admin/jobs?tab=metadata');
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
  const toggle = page.getByRole('button', { name: APPLY_NAME, exact: true });
  await resetFocus(page);
  await tabTo(page, toggle, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  const apply = foldOut(page, APPLY_NAME);
  const more = apply.getByRole('button', { name: `Weitere Aktionen für ${APPLY_NAME}` });

  // Enter opens the menu on its first item; the arrows stay on what can be picked (cancelling
  // all is off without a running run); Escape closes it onto its button.
  await tabTo(page, more, 10);
  await page.keyboard.press('Enter');
  const menu = page.getByRole('menu');
  await expect(menu).toBeVisible();
  const cancelQueued = menu.getByRole('menuitem', { name: 'Wartende abbrechen (1)' });
  await expect(cancelQueued).toBeFocused();
  await expect(menu.getByRole('menuitem', { name: 'Alle Läufe abbrechen' })).toHaveAttribute(
    'aria-disabled',
    'true',
  );
  await page.keyboard.press('ArrowDown');
  await expect(cancelQueued).toBeFocused();
  await page.keyboard.press('ArrowUp');
  await expect(cancelQueued).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
  await expectFocusOn(page, more);

  // Enter on the item asks; Escape keeps the run, the confirmation cancels it.
  await page.keyboard.press('Enter');
  await expect(cancelQueued).toBeFocused();
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('alertdialog', {
    name: `Wartende Läufe von ${APPLY_NAME} abbrechen?`,
  });
  await expect(dialog).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, more);
  await page.keyboard.press('Enter');
  await expect(cancelQueued).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: 'Abbrechen', exact: true }).press('Enter');
  await expect(page.getByRole('status').first()).toContainText(
    `${APPLY_NAME}: 1 Lauf abgebrochen.`,
  );
  await expectFocusOn(page, more);

  // A run cancelled in the row's runs leaves its table: the focus goes to the h1 (ALL-2).
  const again = await owner.post('/api/v1/admin/metadata/apply', {
    headers: CSRF,
    data: { what: ['images'] },
  });
  expect(again.status(), await again.text()).toBe(202);
  const { jobId } = (await again.json()) as { jobId: number };
  const runs = apply.getByRole('region', { name: `Läufe von ${APPLY_NAME}` });
  await expect(runs.getByRole('tab', { selected: true })).toHaveText(/^Wartend/);
  const cancelRun = runs.getByRole('button', { name: `Lauf ${jobId} abbrechen`, exact: true });
  await expect(cancelRun).toBeVisible({ timeout: 15_000 }); // the runs poll every 5 s
  await tabTo(page, cancelRun, 20);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status').first()).toContainText(`Lauf ${jobId} abgebrochen.`);
  await expect(cancelRun).toHaveCount(0);
  // The h1 takes the focus by script (tabIndex -1) and draws no ring, like every page head.
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeFocused();
  await expectNoFocusTrap(page);
});

/** The sweep's schedule as the server holds it. */
async function sweepSchedule(
  api: APIRequestContext,
): Promise<{ timeZone: string; once?: { local: string; at: string } }> {
  const response = await api.get('/api/v1/admin/job-types');
  expect(response.status(), await response.text()).toBe(200);
  const overview = (await response.json()) as {
    types: {
      type: string;
      schedule?: { timeZone: string; once?: { local: string; at: string } };
    }[];
  };
  const schedule = overview.types.find((info) => info.type === SWEEP)?.schedule;
  expect(schedule).toBeDefined();
  return schedule!;
}

test('background jobs by keyboard: a one-off date in the schedule as the server wall time, and removed again @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await resumeAll(owner);
  await restoreSweep(owner);
  const { timeZone } = await sweepSchedule(owner);
  // Two days ahead at 03:30: within now plus a minute to a year in any zone of the server.
  const day = new Date(Date.now() + 2 * 86_400_000).toISOString().slice(0, 10);
  const local = `${day}T03:30`;

  await page.goto('/admin/jobs');
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
  const toggle = page.getByRole('button', { name: SWEEP_NAME, exact: true });
  await resetFocus(page);
  await tabTo(page, toggle, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  const schedule = foldOut(page, SWEEP_NAME).getByRole('region', {
    name: `Zeitplan von ${SWEEP_NAME}`,
  });

  // Tab from the window's switch reaches «Einmalig am», a native date and time field that names
  // the server's zone.
  await tabTo(page, schedule.getByRole('switch', { name: 'Nur in einem Zeitfenster' }), TAB_MAX);
  await page.keyboard.press('Tab');
  const once = schedule.getByLabel('Einmalig am');
  await expectFocusOn(page, once);
  await expectVisibleFocus(page);
  await expect(once).toHaveAttribute('type', 'datetime-local');
  await expect(
    schedule.getByText(`Zeitzone des Servers: ${timeZone}.`, { exact: false }),
  ).toBeVisible();

  // The segments of the native field differ per browser and language: fill sets the value, and
  // Enter in the field saves the form. The server holds the wall time as it was typed.
  await once.fill(local);
  await expectFocusOn(page, once);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status').first()).toContainText(
    `Zeitplan von ${SWEEP_NAME} gespeichert.`,
  );
  expect((await sweepSchedule(owner)).once?.local).toBe(local);
  const row = page.getByRole('row', { name: new RegExp(`^${SWEEP_NAME}`) });
  await expect(row.getByText(/Termin: /)).toBeVisible();

  // Escape has nothing to close: the focus stays in the field, visible.
  await page.keyboard.press('Escape');
  await expectFocusOn(page, once);
  await expectVisibleFocus(page);

  // «Termin entfernen» with Enter empties the field and keeps the focus there; «Speichern»
  // removes the date on the server.
  const clear = schedule.getByRole('button', { name: 'Termin entfernen' });
  await tabTo(page, clear, 10);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, once);
  await expect(once).toHaveValue('');
  const save = schedule.getByRole('button', { name: 'Speichern', exact: true });
  await tabTo(page, save, 10);
  await page.keyboard.press('Enter');
  await expect(row.getByText(/Termin: /)).toHaveCount(0);
  expect((await sweepSchedule(owner)).once).toBeUndefined();
  await expectNoFocusTrap(page);
});

test('background jobs by keyboard: a time window for the keyframe index, set and removed again @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await resumeAll(owner);
  await clearProbeWindow(owner);
  // A window from an hour ago to two hours ahead on the server's clock: open now, so the
  // column names the window and the index holds nothing back while the test runs.
  const { windowTimeZone } = await probeInfo(owner);
  expect(windowTimeZone).toBeTruthy();
  // A server without TZ names its zone "Local": the e2e server runs on this machine's clock.
  const parts = new Intl.DateTimeFormat('en-GB', {
    ...(windowTimeZone === 'Local' ? {} : { timeZone: windowTimeZone }),
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(new Date());
  const part = (name: string) => Number(parts.find((entry) => entry.type === name)?.value);
  const now = part('hour') * 60 + part('minute');
  const startMinute = (now + 1440 - 60) % 1440;
  const endMinute = (now + 120) % 1440;

  await page.goto('/admin/jobs');
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
  const row = page.getByRole('row', { name: /^Keyframe-Index \(ganze Datei lesen\)/ });
  await expect(row.getByText('automatisch', { exact: true })).toBeVisible();
  const toggle = page.getByRole('button', { name: PROBE_NAME, exact: true });
  await resetFocus(page);
  await tabTo(page, toggle, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  const block = foldOut(page, PROBE_NAME).getByRole('region', {
    name: `Zeitfenster von ${PROBE_NAME}`,
  });
  await expect(block.getByText(/^Kein Zeitplan: läuft von selbst/)).toBeVisible();

  // Tab reaches the switch, Space turns it on; Tab walks Von and Bis, which name the zone.
  const windowSwitch = block.getByRole('switch', { name: 'Nur in einem Zeitfenster' });
  await tabTo(page, windowSwitch, TAB_MAX);
  await expectVisibleFocus(page);
  await expect(windowSwitch).not.toBeChecked();
  await page.keyboard.press('Space');
  await expect(windowSwitch).toBeChecked();
  await expect(block.getByText(/Wiedergabe und Downloads starten trotzdem sofort\./)).toBeVisible();
  await page.keyboard.press('Tab');
  const start = block.getByLabel('Von');
  await expectFocusOn(page, start);
  await expectVisibleFocus(page);
  await start.fill(clock(startMinute));
  // Tab walks the segments of the native time field (hours, minutes) before it leaves it.
  const end = block.getByLabel('Bis');
  await tabTo(page, end, 5);
  await expectVisibleFocus(page);
  await end.fill(clock(endMinute));
  await expect(
    block.getByText(`Zeitzone des Servers: ${windowTimeZone}.`, { exact: false }),
  ).toBeVisible();

  // «Speichern» with Enter: the server holds the window, the column names it.
  const save = block.getByRole('button', { name: 'Speichern', exact: true });
  await tabTo(page, save, 10);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status').first()).toContainText(
    `Zeitfenster von ${PROBE_NAME} gespeichert.`,
  );
  expect((await probeInfo(owner)).window).toEqual({ startMinute, endMinute });
  await expect(
    row.getByText(`automatisch · ${clock(startMinute)}–${clock(endMinute)}`, { exact: true }),
  ).toBeVisible();
  await expect(block.getByRole('button', { name: 'Verwerfen' })).toHaveCount(0);
  await expectFocusOn(page, save);

  // Space on the switch turns it off again, «Speichern» removes the window on the server.
  await resetFocus(page); // Tab from the top: Firefox keeps the start where a replaced field stood
  await tabTo(page, windowSwitch, TAB_MAX);
  await page.keyboard.press('Space');
  await expect(windowSwitch).not.toBeChecked();
  await expect(start).toHaveCount(0);
  await tabTo(page, save, 10);
  await page.keyboard.press('Enter');
  await expect(row.getByText('automatisch', { exact: true })).toBeVisible();
  expect((await probeInfo(owner)).window).toBeUndefined();
  await expectNoFocusTrap(page);
});

/** Two failures the stubbed dashboard adds to the server's own (see the next test). */
const STUB_RETRY = 990_001;
const STUB_CANCEL = 990_002;

test.describe('failures', () => {
  // Chromium sends the page's GET requests through the service worker, where page.route does not
  // see them; the dashboard is answered here, so the worker stays out.
  test.use({ serviceWorkers: 'block' });

  test('the overview links the tab «Fehlschläge»; retry and cancel there by keyboard, the focus goes to the h1 @keyboard', async ({
    page,
  }) => {
    // No API of the e2e server ends a run as failed in time: every type has five attempts with a
    // backoff from 10 s doubling (about 2.5 min, `server/internal/jobs/queue.go`), and there is no
    // test hook. The dashboard is therefore answered by the server plus two stubbed failures, and
    // their «Erneut» and «Abbrechen» calls are answered here; everything else is the real page.
    const updatedAt = new Date().toISOString();
    const stubs = [
      {
        id: STUB_RETRY,
        type: 'metadata.match',
        state: 'failed',
        attempts: 5,
        maxAttempts: 5,
        lastError: 'tmdb: HTTP 503 Service Unavailable',
        updatedAt,
      },
      {
        id: STUB_CANCEL,
        type: 'probe.keyframes',
        state: 'failed',
        attempts: 5,
        maxAttempts: 5,
        lastError: 'ffprobe: Invalid data found when processing input',
        updatedAt,
      },
    ];
    let failures = [...stubs];
    const calls: string[] = [];
    await page.route('**/api/v1/admin/dashboard', async (route) => {
      const response = await route.fetch();
      const body = (await response.json()) as { jobs: { failed: number; recentFailed: unknown[] } };
      body.jobs.failed += failures.length;
      body.jobs.recentFailed = [...failures, ...body.jobs.recentFailed].slice(0, 10);
      await route.fulfill({ response, json: body });
    });
    await page.route(`**/api/v1/admin/jobs/${STUB_RETRY}/retry`, async (route) => {
      calls.push(`${route.request().method()} retry`);
      failures = failures.filter((job) => job.id !== STUB_RETRY);
      await route.fulfill({ json: { ...stubs[0], state: 'queued', attempts: 0 } });
    });
    await page.route(`**/api/v1/admin/jobs/${STUB_CANCEL}`, async (route) => {
      if (route.request().method() !== 'DELETE') return route.fallback();
      calls.push('DELETE cancel');
      failures = failures.filter((job) => job.id !== STUB_CANCEL);
      await route.fulfill({ status: 204 });
    });

    // The overview's card «Aufgaben» links the tab.
    await page.goto('/admin');
    const failuresLink = page
      .getByRole('region', { name: 'Aufgaben', exact: true })
      .getByRole('link', { name: 'Fehlschläge ansehen' });
    await expect(failuresLink).toHaveAttribute('href', '/admin/jobs?tab=failed');
    await resetFocus(page);
    await tabTo(page, failuresLink, TAB_MAX);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/jobs\?tab=failed$/);
    const tabs = page.getByRole('tablist', { name: 'Jobs nach Gruppe' });
    await expect(tabs.getByRole('tab', { selected: true })).toHaveText(/^Fehlschläge/);
    const table = page.getByRole('table', { name: 'Letzte Fehlschläge (alle Arten)' });
    const retryRow = table.getByRole('row', { name: /^Titel zuordnen/ });
    await expect(retryRow.getByText('tmdb: HTTP 503 Service Unavailable')).toBeVisible();
    const h1 = page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' });

    // «Erneut» with Enter: the row goes, the h1 takes the focus (it draws no ring by design).
    const retry = table.getByRole('button', {
      name: `Lauf ${STUB_RETRY} von Titel zuordnen erneut einreihen`,
    });
    await resetFocus(page);
    await tabTo(page, retry, TAB_MAX);
    await page.keyboard.press('Enter');
    await expect(page.getByRole('status').first()).toContainText(
      `Lauf ${STUB_RETRY} wartet wieder.`,
    );
    await expect(retryRow).toHaveCount(0);
    await expect(h1).toBeFocused();

    // «Abbrechen» asks: Escape keeps the run with the focus back on the button, Enter on the
    // confirmation cancels it, the row goes and the h1 takes the focus.
    const cancel = table.getByRole('button', {
      name: `Lauf ${STUB_CANCEL} von Keyframe-Index (ganze Datei lesen) abbrechen`,
    });
    await tabTo(page, cancel, TAB_MAX);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', {
      name: 'Lauf von Keyframe-Index (ganze Datei lesen) abbrechen?',
    });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('Slouchcat versucht diesen Lauf nicht mehr.')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, cancel);
    expect(calls).toEqual(['POST retry']);
    await page.keyboard.press('Enter');
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Abbrechen', exact: true }).press('Enter');
    await expect(page.getByRole('status').first()).toContainText(
      `Lauf ${STUB_CANCEL} abgebrochen.`,
    );
    await expect(cancel).toHaveCount(0);
    await expect(h1).toBeFocused();
    expect(calls).toEqual(['POST retry', 'DELETE cancel']);
    // Firefox keeps the closing confirmation in the page for its exit animation; wait it out.
    await expect(dialog).toHaveCount(0);
    await expectNoFocusTrap(page);
  });
});
