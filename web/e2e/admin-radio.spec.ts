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
// itself is covered by RadioAdminPage.test.tsx); visible focus, no focus trap. Stage 2: the tab
// «Eigene Sender» (a local address refused, a public one added and deleted) by keyboard. The tab
// «Einordnung» is gone with the tag rules (contract 1.28.0, concept 19 part B); the tabs «Angebot»
// (PB-10) and the new «Sender» (B9.3) by keyboard.

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
    `/api/v1/admin/radio/stations?offered=false&limit=50&q=${encodeURIComponent(name)}`,
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
  await expect(tabs.getByRole('tab', { name: 'Angebot' })).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/[?&]tab=angebot/);
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
  // Stage 2 put «Eigene Sender» between them.
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

/** Deletes the own stations a test of this file added. */
async function removeOwnStations(owner: APIRequestContext, prefix: string): Promise<void> {
  const response = await owner.get(
    '/api/v1/admin/radio/stations?source=own&offered=false&limit=200',
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

interface Offer {
  version: number;
  countries: { code: string; mode?: 'all' | 'marked'; offered: number; total: number }[];
}

async function readOffer(owner: APIRequestContext): Promise<Offer> {
  const response = await owner.get('/api/v1/admin/radio/offer');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as Offer;
}

/** Applies steps on the current offer, as the tab «Sender» does. */
async function applyOffer(owner: APIRequestContext, steps: object[]): Promise<void> {
  const { version } = await readOffer(owner);
  const response = await owner.post('/api/v1/admin/radio/offer/apply', {
    headers: CSRF,
    data: { version, steps },
  });
  expect(response.status(), await response.text()).toBe(200);
}

test('«Angebot» by keyboard: search a country, a continent, the station list, apply (PB-10) @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  // Spain leaves the offer for the test (its two stations are used nowhere else) and comes back
  // whole after it: «Alle», no exceptions.
  await applyOffer(owner, [{ op: 'removeCountry', country: 'ES' }]);
  try {
    await page.goto('/admin/radio?tab=angebot');
    await expect(page.getByRole('tab', { name: 'Angebot' })).toHaveAttribute(
      'aria-selected',
      'true',
    );

    // The search finds the country by name; Space chooses it.
    const search = page.getByRole('textbox', { name: 'Land suchen' });
    await resetFocus(page);
    await tabTo(page, search, 80);
    await page.keyboard.type('Spanien');
    const hits = page.getByRole('table', { name: 'Gefundene Länder' });
    const spain = hits.getByRole('checkbox', { name: 'Spanien wählen' });
    await tabTo(page, spain);
    await page.keyboard.press('Space');
    await expect(spain).toBeChecked();
    await expect(page.getByText('2 Sender kommen dazu.')).toBeVisible();

    // A continent opens with Enter, arrows go through its countries, Escape closes it.
    const europe = page.getByRole('button', { name: /^Europa/ });
    await tabTo(page, europe);
    await page.keyboard.press('Enter');
    await expect(europe).toHaveAttribute('aria-expanded', 'true');
    const countries = page.getByRole('table', { name: 'Länder in Europa' });
    const germany = countries.getByRole('checkbox', { name: 'Deutschland wählen' });
    await tabTo(page, germany, 10);
    await expect(germany).toBeChecked();
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, countries.getByRole('checkbox', { name: 'Frankreich wählen' }));
    await page.keyboard.press('Escape');
    await expect(countries).toHaveCount(0);
    await expectFocusOn(page, europe);
    await expect(europe).toHaveAttribute('aria-expanded', 'false');

    // The chosen country opens its station list; Space takes a station out of the draft.
    const chosen = page.getByRole('table', { name: 'Gewählte Länder' });
    const open = chosen.getByRole('button', { name: 'Sender in Spanien wählen' });
    await tabTo(page, open, 40);
    await page.keyboard.press('Enter');
    const list = page.getByRole('table', { name: 'Sender in Spanien' });
    await expect(list).toBeVisible();
    await expect(
      page.getByText('Alle Sender dieses Landes sind im Angebot, ausser den demarkierten.', {
        exact: false,
      }),
    ).toBeVisible();
    const los40 = list.getByRole('checkbox', { name: 'LOS40 im Angebot' });
    await tabTo(page, los40, 10);
    await page.keyboard.press('ArrowDown');
    const ser = list.getByRole('checkbox', { name: 'Cadena SER im Angebot' });
    await expectFocusOn(page, ser);
    await expect(ser).toBeChecked();
    await page.keyboard.press('Space');
    await expect(ser).not.toBeChecked();
    await expect(page.getByText('1 Sender kommt dazu.')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(list).toHaveCount(0);
    await expectFocusOn(page, chosen.getByRole('button', { name: 'Sender in Spanien wählen' }));

    // «Übernehmen» at the bottom; the message stands in its place.
    const apply = page.getByRole('button', { name: 'Übernehmen' });
    await tabTo(page, apply, 40);
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(
        'Übernommen. Die Radioseite zeigt das neue Angebot sofort. Logos und weitere Angaben lädt der Server jetzt im Hintergrund.',
      ),
    ).toBeVisible();
    const stored = (await readOffer(owner)).countries.find((country) => country.code === 'ES');
    expect(stored).toMatchObject({ mode: 'all', offered: 1, total: 2 });

    await resetFocus(page);
    await expectNoFocusTrap(page, 120);
  } finally {
    await applyOffer(owner, [
      { op: 'chooseCountry', country: 'ES' },
      { op: 'markAll', country: 'ES' },
    ]);
  }
});

test('«Sender» by keyboard: unreachable ones removed and back, out of the offer, whole catalog, details, back in @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  const name = 'Testradio Drei';
  const id = await stationId(owner, name);
  // Every station a «Nicht erreichbare entfernen» took out comes back after the test.
  const demarked: number[] = [];
  page.on('response', async (response) => {
    if (!response.url().endsWith('/admin/radio/offer/apply') || !response.ok()) return;
    const body = (await response.json().catch(() => null)) as {
      demarkedStationIds?: number[];
    } | null;
    demarked.push(...(body?.demarkedStationIds ?? []));
  });
  const title = page.getByRole('heading', { level: 1, name: 'Radio' });
  try {
    await page.goto('/admin/radio?tab=sender');
    const table = page.getByRole('table', { name: 'Sender', exact: true });
    await expect(table).toBeVisible();

    // «Nicht erreichbare entfernen» takes the unreachable ones out and turns disabled: the focus
    // waits on the h1. «Rückgängig» brings them back and goes with its message: the h1 again.
    const remove = page.getByRole('button', { name: /^Nicht erreichbare entfernen \(\d+\)$/ });
    await resetFocus(page);
    await tabTo(page, remove, 80);
    await page.keyboard.press('Enter');
    await expect(page.getByText(/aus dem Angebot genommen\.$/)).toBeVisible();
    await expect(
      page.getByRole('button', { name: 'Nicht erreichbare entfernen (0)' }),
    ).toBeDisabled();
    await expect(title).toBeFocused();
    const undo = page.getByRole('button', { name: 'Rückgängig' });
    await tabTo(page, undo, 40);
    await page.keyboard.press('Enter');
    await expect(page.getByText('Die Sender sind wieder im Angebot.')).toBeVisible();
    await expect(undo).toHaveCount(0);
    await expect(remove).toBeEnabled();
    await expect(title).toBeFocused();

    // In the list of the offer a station taken out leaves the list: the focus waits on the h1.
    const search = page.getByRole('textbox', { name: 'Sender suchen' });
    await tabTo(page, search, 40);
    await page.keyboard.type(name);
    await expect(table.getByRole('rowheader', { name: new RegExp(name) })).toBeVisible();
    const out = table.getByRole('button', { name: `${name} aus dem Angebot nehmen` });
    await tabTo(page, out, 30);
    await page.keyboard.press('Enter');
    await expect(
      page.getByRole('status').filter({ hasText: `${name} ist nicht mehr im Angebot.` }),
    ).toHaveCount(1);
    await expect(table.getByRole('rowheader', { name: new RegExp(name) })).toHaveCount(0);
    await expect(title).toBeFocused();

    // The whole catalog by the switch shows it again.
    const whole = page.getByRole('switch', { name: 'Ganzen Katalog zeigen' });
    await tabTo(page, whole, 40);
    await page.keyboard.press('Space');
    await expect(whole).toBeChecked();
    await expect(table.getByRole('rowheader', { name: new RegExp(name) })).toBeVisible();

    // The details of the row in a dialog (stage B2): a station outside the offer has no Wikidata
    // details; Escape closes it, the focus goes back.
    const details = table.getByRole('button', { name: `Angaben zu ${name}` });
    await tabTo(page, details, 30);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: `Angaben zu ${name}` });
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Schliessen' }));
    await expect(dialog.getByText('Wikidata ergänzt nur Sender im Angebot.')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, details);

    // Back into the offer and out again here: the row stays, the focus on its button.
    const back = table.getByRole('button', { name: `${name} ins Angebot nehmen` });
    await tabTo(page, back, 5);
    await page.keyboard.press('Enter');
    await expect(
      page.getByRole('status').filter({ hasText: `${name} ist im Angebot.` }),
    ).toHaveCount(1);
    await expectFocusOn(page, out);
    await page.keyboard.press('Enter');
    await expect(
      page.getByRole('status').filter({ hasText: `${name} ist nicht mehr im Angebot.` }),
    ).toHaveCount(1);
    await expectFocusOn(page, back);

    await resetFocus(page);
    await expectNoFocusTrap(page, 120);
  } finally {
    await applyOffer(owner, [{ op: 'mark', stationIds: [id, ...demarked] }]);
  }
});

test('«Angaben» by keyboard: the origin of each value, owner and founding date from Wikidata (concept 19 B2) @keyboard', async ({
  page,
  owner,
}) => {
  // The e2e radio server answers the SPARQL queries (testdata/radio/wikidata.csv); the setup's
  // apply queued a run already, this one makes sure it ran with the mock.
  const run = await owner.post('/api/v1/admin/job-types/radio.wikidata.sync/run', {
    headers: CSRF,
  });
  expect(run.status(), await run.text()).toBe(202);
  const id = await stationId(owner, 'SRF 3');
  await expect
    .poll(
      async () => {
        const response = await owner.get(`/api/v1/admin/radio/stations/${id}`);
        const body = (await response.json()) as { wikidata?: { owner?: string } };
        return body.wikidata?.owner;
      },
      { timeout: 30_000 },
    )
    .toBe('Schweizer Radio und Fernsehen');

  await page.goto('/admin/radio?tab=sender');
  const table = page.getByRole('table', { name: 'Sender', exact: true });
  await expect(table).toBeVisible();
  const search = page.getByRole('textbox', { name: 'Sender suchen' });
  await resetFocus(page);
  await tabTo(page, search, 40);
  await page.keyboard.type('SRF 3');
  // The list follows the search (Radio SRF 1 has the same owner); the owner is the provider.
  await expect(table.getByRole('button', { name: 'Angaben zu Radio SRF 1' })).toHaveCount(0);
  await expect(
    table.getByRole('button', { name: 'Nach Anbieter Schweizer Radio und Fernsehen filtern' }),
  ).toBeVisible();
  const details = table.getByRole('button', { name: 'Angaben zu SRF 3' });
  await tabTo(page, details, 30);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Angaben zu SRF 3' });
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Schliessen' }));
  const fact = (term: string) => dialog.locator('dt', { hasText: term }).locator('+ dd');
  await expect(fact('Name')).toContainText('Herkunft: Katalog (Radio Browser)');
  await expect(fact('Anbieter')).toContainText('Schweizer Radio und Fernsehen');
  await expect(fact('Anbieter')).toContainText('Herkunft: Wikidata');
  await expect(fact('Gründung')).toContainText('1. November 1983');
  await expect(fact('Wikidata-Kennung')).toContainText('Q1157782');
  // Tab stays in the dialog, Enter on «Schliessen» closes it and the focus goes back.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Schliessen' }));
  await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, details);

  await resetFocus(page);
  await expectNoFocusTrap(page, 120);
});
