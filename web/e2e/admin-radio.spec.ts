import type { APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';
import { OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// /admin/radio (concept 16, 11, variant A «Reiter») by keyboard (rule 8): arrow keys across the
// tabs with `?tab=` in the URL, Tab through the stations table, Enter opens the editor in
// `?item=`, Escape closes it with the focus back on the row's button, the settings rows by
// keyboard, a running listener stopped through the confirmation; the RAM-buffer warning follows
// the server's buffer mode; visible focus, no focus trap.

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
  await page.goto('/admin/radio');
  await expect(page.getByRole('heading', { level: 1, name: 'Radio' })).toBeVisible();
  await expect(
    page.getByRole('navigation', { name: 'Verwaltung' }).getByRole('link', { name: 'Radio' }),
  ).toHaveAttribute('aria-current', 'page');
  // The warning without RAM disk follows the server's buffer mode (e2e runs without tmpfs).
  await expect(page.getByText('Kein RAM-Puffer', { exact: true })).toHaveCount(
    config.bufferMode === 'memory' ? 1 : 0,
  );
  await expect(page.getByRole('group', { name: 'Zusammenfassung' })).toBeVisible();

  // Arrow keys across the tabs; the URL says which one.
  const tabs = page.getByRole('tablist', { name: 'Bereiche' });
  const overview = tabs.getByRole('tab', { name: /Übersicht/ });
  await expect(overview).toHaveAttribute('aria-selected', 'true');
  await resetFocus(page);
  await tabTo(page, overview, 60);
  await page.keyboard.press('ArrowRight');
  const stationsTab = tabs.getByRole('tab', { name: /Sender/ });
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
  // «Verwerfen» goes with the changes (SaveBar); the time shift is the stored one again.
  await expect(page.getByRole('button', { name: 'Verwerfen' })).toHaveCount(0);
  await expect(page.getByText('Ungespeicherte Änderungen')).toHaveCount(0);

  await resetFocus(page);
  await expectNoFocusTrap(page);
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
