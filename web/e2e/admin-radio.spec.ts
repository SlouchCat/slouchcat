import type { APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';
import { OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// /admin/radio (concept 16, 11, variant A «Reiter») by keyboard (rule 8): arrow keys across the
// tabs with `?tab=` in the URL, Tab through the stations table, Enter opens the editor in
// `?item=`, Escape closes it with the focus back on the row's button, the settings rows by
// keyboard, «Verwerfen» with the focus on the h1 after it, Back to the page before (the tabs and
// the editor replace their history entry); a running listener stopped through the confirmation;
// the suite keeps the relay buffers on disk as a tmpfs (D-14), so no RAM-buffer warning (the warning
// itself is covered by RadioAdminPage.test.tsx); visible focus, no focus trap. Stage 2: the tabs
// «Einordnung» (tag rules, coverage) and «Eigene Sender» (a local address refused, a public one
// added and deleted) by keyboard.

async function stopRadio(owner: APIRequestContext): Promise<void> {
  const response = await owner.get('/api/v1/admin/radio/relays');
  if (!response.ok()) return;
  const { relays } = (await response.json()) as { relays: { listeners: { listenId: string }[] }[] };
  for (const listener of relays.flatMap((relay) => relay.listeners)) {
    await owner.delete(`/api/v1/admin/radio/listeners/${listener.listenId}`, { headers: CSRF });
  }
}

async function stationId(owner: APIRequestContext, name: string): Promise<number> {
  const response = await owner.get(
    `/api/v1/admin/radio/stations?exceptions=false&limit=50&q=${encodeURIComponent(name)}`,
  );
  expect(response.status(), await response.text()).toBe(200);
  const { items } = (await response.json()) as { items: { id: number; name: string }[] };
  const station = items.find((entry) => entry.name === name);
  expect(station, `station "${name}" in the catalog`).toBeDefined();
  return station!.id;
}

test.afterEach(async ({ owner }) => {
  await stopRadio(owner);
});

test('radio administration by keyboard: tabs in the URL, stations table, editor, settings @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const config = (await (await owner.get('/api/v1/admin/radio/config')).json()) as {
    bufferMode: 'tmpfs' | 'memory';
  };
  // SLOUCHCAT_TEST_RADIO_DISK_BUFFER (start-server.mjs): the disk buffer counts as a tmpfs.
  expect(config.bufferMode).toBe('tmpfs');
  // Reached from the overview, so Back has a page to return to.
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  const navLink = page
    .getByRole('navigation', { name: 'Verwaltung' })
    .getByRole('link', { name: 'Radio' });
  await navLink.click();
  await expect(page.getByRole('heading', { level: 1, name: 'Radio' })).toBeVisible();
  await expect(navLink).toHaveAttribute('aria-current', 'page');
  // With the buffer on a tmpfs there is no warning.
  await expect(page.getByText('Kein RAM-Puffer', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('group', { name: 'Zusammenfassung' })).toBeVisible();

  // Arrow keys across the tabs; the URL says which one.
  const tabs = page.getByRole('tablist', { name: 'Bereiche' });
  const overview = tabs.getByRole('tab', { name: /Übersicht/ });
  await expect(overview).toHaveAttribute('aria-selected', 'true');
  await resetFocus(page);
  await tabTo(page, overview, 60);
  await page.keyboard.press('ArrowRight');
  const stationsTab = tabs.getByRole('tab', { name: 'Sender', exact: true });
  await expectFocusOn(page, stationsTab);
  await expect(stationsTab).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/[?&]tab=sender/);

  // The search covers every station; Tab reaches a row's «Bearbeiten», Enter opens the editor.
  const search = page.getByRole('textbox', { name: 'Sender suchen' });
  await tabTo(page, search);
  await page.keyboard.type('Testradio');
  const table = page.getByRole('table', { name: 'Sender' });
  await expect(table.getByRole('rowheader', { name: /Testradio Eins/ })).toBeVisible();
  const edit = table.getByRole('button', { name: 'Testradio Eins bearbeiten' });
  await tabTo(page, edit);
  await page.keyboard.press('Enter');
  const editor = page.getByRole('dialog');
  await expect(editor.getByRole('heading', { name: 'Testradio Eins' })).toBeVisible();
  // adminSearch keeps the id as text, which the router writes quoted (`?item=%221%22`).
  await expect(page).toHaveURL(/[?&]item=/);
  await expect(editor.getByRole('textbox', { name: 'Name' })).toHaveValue('Testradio Eins');
  await expect(editor.getByRole('button', { name: 'Stream prüfen' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(editor).toHaveCount(0);
  await expect(page).not.toHaveURL(/item=/);
  await expectFocusOn(page, edit);

  // The settings as rows: an arrow key picks another time shift, «Verwerfen» takes it back.
  // From the top: headless Firefox does not wrap around at the end of the page.
  await resetFocus(page);
  await tabTo(page, stationsTab, 80);
  // Stage 2 put «Einordnung» and «Eigene Sender» between them.
  await page.keyboard.press('ArrowRight');
  await expect(page).toHaveURL(/[?&]tab=einordnung/);
  await page.keyboard.press('ArrowRight');
  await expect(page).toHaveURL(/[?&]tab=eigene/);
  await page.keyboard.press('ArrowRight');
  const settingsTab = tabs.getByRole('tab', { name: /Einstellungen/ });
  await expect(settingsTab).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/[?&]tab=einstellungen/);
  const shift = page.getByRole('group', { name: 'Zeitversatz' });
  await expect(
    shift.getByText('Gilt für neu startende Weiterleitungen.', { exact: false }),
  ).toBeVisible();
  const checked = shift.getByRole('radio', { checked: true });
  await tabTo(page, checked);
  await page.keyboard.press('ArrowRight');
  await expect(page.getByText('Ungespeicherte Änderungen')).toBeVisible();
  const discard = page.getByRole('button', { name: 'Verwerfen' });
  await tabTo(page, discard, 60);
  await page.keyboard.press('Enter');
  // «Verwerfen» goes with the changes (SaveBar); the time shift is the stored one again and the
  // focus waits on the page head (ALL-2).
  await expect(page.getByRole('button', { name: 'Verwerfen' })).toHaveCount(0);
  await expect(page.getByText('Ungespeicherte Änderungen')).toHaveCount(0);
  await expect(page.getByRole('heading', { level: 1, name: 'Radio' })).toBeFocused();

  await resetFocus(page);
  await expectNoFocusTrap(page);

  // Back leaves the page to the overview: tabs and editor replaced their history entries.
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
});

test('a running listener is stopped through the confirmation', async ({ page, owner }) => {
  test.setTimeout(90_000);
  const id = await stationId(owner, 'Testradio Eins');
  const listen = await owner.post(`/api/v1/radio/stations/${id}/listen`, { headers: CSRF });
  expect(listen.status(), await listen.text()).toBe(201);

  await page.goto('/admin/radio');
  const stop = page.getByRole('button', { name: `Hören von ${OWNER_PROFILE} stoppen` });
  await expect(stop).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId('radio-relays').getByText('Testradio Eins')).toBeVisible();
  await resetFocus(page);
  await tabTo(page, stop, 60);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog');
  await expect(confirm).toBeVisible();
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Stoppen' }));
  await page.keyboard.press('Enter');
  await expect(stop).toHaveCount(0, { timeout: 15_000 });
  await expect(page.getByRole('status').filter({ hasText: 'ist gestoppt' })).toHaveCount(1);
});

type TagRule = { tag: string; facet: string; value?: string | null; source: string };

/** The administration's own tag rules, as the PUT takes them back. */
async function adminRules(owner: APIRequestContext) {
  const response = await owner.get('/api/v1/admin/radio/tag-rules');
  expect(response.status(), await response.text()).toBe(200);
  const { rules } = (await response.json()) as { rules: TagRule[] };
  return rules
    .filter((rule) => rule.source === 'admin')
    .map(({ tag, facet, value }) => ({ tag, facet, value: value ?? null }));
}

test('«Einordnung» by keyboard: a rule set to «Keins» changes the coverage, removing it restores it @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const stored = await adminRules(owner);
  try {
    await page.goto('/admin/radio?tab=einordnung');
    const panel = page.getByRole('tabpanel');
    const genre = panel.getByTestId('radio-coverage').getByRole('listitem').first();
    await expect(genre).toContainText('Genre');
    // As toHaveText reads it: the text content, without the line breaks of innerText.
    const before = (await genre.textContent()) ?? '';
    expect(before).toMatch(/^Genre\d+\s%\d+ von \d+ Sendern$/);
    // The 68 stations of the test catalog have no tag of 20 stations without a rule (D-3).
    await expect(panel.getByTestId('radio-worklist')).toContainText(
      'Alle häufigen Schlagworte sind zugeordnet.',
    );

    // The default rule «pop → Genre: Pop» to «Keins» (D-2), by keyboard in the rules table.
    const rules = panel.getByTestId('radio-rules');
    const search = rules.getByRole('textbox', { name: 'Regeln durchsuchen' });
    await resetFocus(page);
    await tabTo(page, search, 80);
    await page.keyboard.type('pop');
    const row = rules
      .getByRole('row')
      .filter({ has: page.getByRole('rowheader', { name: 'pop', exact: true }) });
    await expect(row).toContainText('Vorgabe');
    const select = row.getByRole('combobox', { name: 'Genre für das Schlagwort pop' });
    await tabTo(page, select, 20);
    const saved = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' && response.url().endsWith('/admin/radio/tag-rules'),
    );
    // «Keins» is the last choice.
    await page.keyboard.press('End');
    expect((await saved).status()).toBe(200);
    await expect(
      page.getByText('Die Regel für «pop» ist gespeichert.', { exact: false }),
    ).toBeVisible();
    await expect(row).toContainText('Eigene Regel');
    await expect(genre).not.toHaveText(before);

    // Removing the own rule asks first; the default applies again and the coverage is back.
    const remove = row.getByRole('button', { name: 'Regel Genre für pop entfernen' });
    await tabTo(page, remove, 20);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Regel für «pop» entfernen?' });
    await expect(confirm).toBeVisible();
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Entfernen' }));
    await page.keyboard.press('Enter');
    await expect(page.getByText('Die Regel für «pop» ist entfernt.')).toBeVisible();
    await expect(row).toContainText('Vorgabe');
    await expect(genre).toHaveText(before);
    await resetFocus(page);
    await expectNoFocusTrap(page);
  } finally {
    const restore = await owner.put('/api/v1/admin/radio/tag-rules', {
      headers: CSRF,
      data: { rules: stored },
    });
    expect(restore.status(), await restore.text()).toBe(200);
  }
});

/** Deletes the own stations a test of this file added. */
async function removeOwnStations(owner: APIRequestContext, prefix: string): Promise<void> {
  const response = await owner.get(
    '/api/v1/admin/radio/stations?source=own&exceptions=false&limit=200',
  );
  if (!response.ok()) return;
  const { items } = (await response.json()) as { items: { id: number; name: string }[] };
  for (const station of items.filter((entry) => entry.name.startsWith(prefix))) {
    await owner.delete(`/api/v1/admin/radio/stations/${station.id}`, { headers: CSRF });
  }
}

test('«Eigene Sender» by keyboard: a local address is refused, a public one added and deleted @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const name = 'E2E Quartierradio';
  await removeOwnStations(owner, 'E2E ');
  try {
    await page.goto('/admin/radio?tab=eigene');
    const add = page.getByRole('button', { name: 'Eigenen Sender hinzufügen' });
    await resetFocus(page);
    await tabTo(page, add, 80);
    await page.keyboard.press('Enter');
    const form = page.getByRole('dialog', { name: 'Eigenen Sender hinzufügen' });
    await expectFocusOn(page, form.getByRole('textbox', { name: 'Name' }));
    await page.keyboard.type(name);
    const stream = form.getByRole('textbox', { name: 'Stream-Adresse' });
    await tabTo(page, stream, 5);
    await page.keyboard.type('http://192.168.1.10/stream');
    // Enter in a field sends the form; the address of the local network stays at its field (F-15).
    await page.keyboard.press('Enter');
    await expect(
      form.getByText(
        'Diese Adresse liegt im lokalen Netz. Auch eigene Sender müssen aus dem Internet kommen.',
      ),
    ).toBeVisible();
    await expect(stream).toHaveAttribute('aria-invalid', 'true');

    // A public address (TEST-NET-3) is taken; the editor opens for the logo.
    await stream.focus();
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type('http://203.0.113.10/stream');
    await page.keyboard.press('Enter');
    const editor = page.getByRole('dialog', { name: new RegExp(name) });
    await expect(editor).toBeVisible({ timeout: 30_000 });
    await expect(editor.getByRole('textbox', { name: 'Stream-Adresse' })).toHaveValue(
      'http://203.0.113.10/stream',
    );
    await page.keyboard.press('Escape');
    await expect(editor).toHaveCount(0);

    // Delete it from the table after the confirmation.
    const table = page.getByRole('table', { name: 'Eigene Sender' });
    await expect(table.getByRole('rowheader', { name: new RegExp(name) })).toBeVisible();
    const remove = table.getByRole('button', { name: `${name} löschen` });
    await resetFocus(page);
    await tabTo(page, remove, 80);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: `${name} löschen?` });
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Löschen' }));
    await page.keyboard.press('Enter');
    await expect(page.getByText(`${name} ist gelöscht.`)).toBeVisible();
    await expect(table.getByRole('rowheader', { name: new RegExp(name) })).toHaveCount(0);
    await resetFocus(page);
    await expectNoFocusTrap(page);
  } finally {
    await removeOwnStations(owner, 'E2E ');
  }
});
