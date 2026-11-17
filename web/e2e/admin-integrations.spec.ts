import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { YOUTUBE_KEY } from './env';
import { expect, test } from './fixtures';
import { setUpdateManifest } from './mocks';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { areaTab } from './admin';

// Integrations (A-2; concept 12, variant A; admin redesign) against the real server: a tile per
// service that opens its view (`?item=`) and leads back by keyboard, provider cards with their state (the e2e server sets the Trakt client id in its
// environment, no TMDB key; concept 20 P-12), «Prüfen» against the Trakt mock, a YouTube key
// saved and deleted by keyboard, the public address and
// the update check in the «Server» block, the e-mail server and the Telegram bot as setting rows
// (both from the environment, with a test mail through the SMTP mock), and the question before
// leaving with unsaved input. Every test leaves the settings as it found them.

interface SettingsData {
  publicUrl?: string;
  publicUrlSource: string;
  updateCheckEnabled: boolean;
  keys: Record<string, { source: string; hint?: string }>;
}

async function settingsOf(api: APIRequestContext): Promise<SettingsData> {
  const response = await api.get('/api/v1/admin/settings');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as SettingsData;
}

async function putSettings(api: APIRequestContext, data: object): Promise<void> {
  const response = await api.put('/api/v1/admin/settings', { headers: CSRF, data });
  expect(response.status(), await response.text()).toBe(200);
}

const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });

/** A setting row of the «Server» block: a group named by its label. */
const serverRow = (page: Page, name: string) =>
  region(page, 'Server').getByRole('group', { name, exact: true });

const SERVICES = {
  keys: 'Schlüssel der Anbieter',
  webhooks: 'Webhooks von Radarr und Sonarr',
  // Its view has a spec of its own (admin-seerr.spec.ts, concept 25).
  seerr: 'Seerr',
  email: 'E-Mail-Server',
  telegram: 'Telegram-Bot',
  push: 'Web-Push',
  server: 'Server',
};

/** Opens the view of a service (`?item=`, admin redesign). */
async function openIntegrations(page: Page, item: keyof typeof SERVICES) {
  await page.goto(`/admin/integrations?item=${item}`);
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  await expect(region(page, SERVICES[item])).toBeVisible();
}

/** The link of a service's tile. */
const tileLink = (page: Page, name: string) =>
  page.getByRole('list', { name: 'Dienste' }).getByRole('link', { name, exact: true });

test('a tile per service opens its view and leads back, all by keyboard @keyboard', async ({
  page,
}) => {
  await page.goto('/admin/integrations');
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  const list = page.getByRole('list', { name: 'Dienste' });
  await expect(list.getByRole('link')).toHaveText(Object.values(SERVICES));
  // The whole state from the dashboard stands at the tiles (build plan 7.2): the e2e server
  // has the Trakt id in its environment.
  await expect(list.getByText('Trakt: Eingerichtet')).toBeVisible();
  await expect(page.getByText(/^\d von 8 eingerichtet$/)).toBeVisible();

  // The intro sentence stands open below the title (build plan 2.6); no help button any more.
  await expect(page.getByRole('button', { name: 'Hilfe zu dieser Seite' })).toHaveCount(0);
  await expect(page.getByText(/Geheime Werte speichert Slouchcat verschlüsselt/)).toBeVisible();

  // Tab reaches the first tile; Enter opens its view, whose heading takes the focus.
  const tile = tileLink(page, 'Schlüssel der Anbieter');
  await resetFocus(page);
  await tabTo(page, tile, 40);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/integrations\?item=keys$/);
  const keys = region(page, 'Schlüssel der Anbieter');
  await expectFocusOn(
    page,
    keys.getByRole('heading', { level: 2, name: 'Schlüssel der Anbieter' }),
  );
  await expect(list).toHaveCount(0);
  await expect(region(page, 'E-Mail-Server')).toHaveCount(0);
  // The next stop is the first card; Shift+Tab goes back to «Alle Integrationen».
  await page.keyboard.press('Tab');
  await expectFocusOn(page, region(page, 'TMDB').getByRole('button', { name: /: TMDB$/ }));
  await page.keyboard.press('Shift+Tab');
  const back = page.getByRole('link', { name: 'Alle Integrationen' });
  await expectFocusOn(page, back);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/integrations$/);
  await expectFocusOn(page, tile);
  await expectNoFocusTrap(page, 200);
});

test('unsaved input survives a change of view; leaving asks first', async ({ page, owner }) => {
  const before = await settingsOf(owner);
  await openIntegrations(page, 'keys');
  const keys = region(page, 'Schlüssel der Anbieter');
  const youtube = region(page, 'YouTube');
  await youtube.getByRole('button', { name: 'Einrichten: YouTube' }).click();
  const field = youtube.getByRole('textbox', { name: 'Schlüssel', exact: true });
  await field.fill('unsaved-key');
  await expect(keys.getByText('ungespeichert', { exact: true }).first()).toBeVisible();

  // Back to the tiles: the tile says so, and its view still holds the key.
  await page.getByRole('link', { name: 'Alle Integrationen' }).click();
  const tile = page
    .getByRole('list', { name: 'Dienste' })
    .getByRole('listitem')
    .filter({ has: page.getByRole('link', { name: 'Schlüssel der Anbieter', exact: true }) });
  await expect(tile.getByText('ungespeichert', { exact: true })).toBeVisible();
  await tileLink(page, 'Schlüssel der Anbieter').click();
  await expect(field).toHaveValue('unsaved-key');

  // The tab link waits for the question, which takes the focus; Escape stays on the page.
  const jobs = areaTab(page, 'Hintergrundjobs');
  await jobs.focus();
  await page.keyboard.press('Enter');
  const question = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  await expect(question).toBeVisible();
  await expect
    .poll(() => question.evaluate((dialog) => dialog.contains(document.activeElement)))
    .toBe(true);
  await page.keyboard.press('Escape');
  await expect(question).toHaveCount(0);
  await expect(page).toHaveURL(/\/admin\/integrations\?item=keys$/);
  await expect(field).toHaveValue('unsaved-key');

  // «Auf der Seite bleiben» by keyboard stays as well.
  await jobs.focus();
  await page.keyboard.press('Enter');
  const stay = question.getByRole('button', { name: 'Auf der Seite bleiben' });
  await expect(stay).toBeVisible();
  await stay.focus();
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(question).toHaveCount(0);
  await expect(page).toHaveURL(/\/admin\/integrations\?item=keys$/);
  await expect(field).toHaveValue('unsaved-key');

  // «Verwerfen und verlassen» leaves without saving anything.
  await jobs.focus();
  await page.keyboard.press('Enter');
  const leave = question.getByRole('button', { name: 'Verwerfen und verlassen' });
  await expect(leave).toBeVisible();
  await leave.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  expect(await settingsOf(owner)).toEqual(before);
});

test('provider cards show their state; the Trakt id from the environment is read-only and checks by keyboard @keyboard', async ({
  page,
}) => {
  await openIntegrations(page, 'keys');
  await expect(areaTab(page, 'Integrationen')).toHaveAttribute('aria-current', 'page');
  const keys = region(page, 'Schlüssel der Anbieter');
  // Six cards, folded; the e2e server runs with the Trakt id from its environment, without TMDB.
  await expect(keys.getByRole('heading', { level: 3 })).toHaveText([
    'TMDB',
    'TVDB',
    'Fanart.tv',
    'YouTube',
    'Trakt',
    'Google Books',
  ]);
  // Other specs may leave a key of their own: the band counts the six providers.
  await expect(keys.getByText(/^\d von 6 Anbietern eingerichtet$/)).toBeVisible();
  const trakt = region(page, 'Trakt');
  await expect(trakt.getByText('In deploy/.env gesetzt', { exact: true })).toBeVisible();
  await expect(
    region(page, 'TMDB').getByRole('button', { name: 'Einrichten: TMDB' }),
  ).toBeVisible();

  // Enter opens the card; the field is locked, the note stands once, «Prüfen» asks the mock.
  const toggle = trakt.getByRole('button', { name: 'Ansehen: Trakt' });
  await resetFocus(page);
  await tabTo(page, toggle, 80);
  await page.keyboard.press('Enter');
  await expect(trakt.getByRole('button', { name: 'Zuklappen: Trakt' })).toHaveAttribute(
    'aria-expanded',
    'true',
  );
  await expect(trakt.getByRole('textbox', { name: 'Client-ID', exact: true })).toBeDisabled();
  await expect(trakt.getByText(/lässt sich nur dort ändern/)).toHaveCount(1);
  await expect(trakt.getByRole('button', { name: 'Trakt speichern' })).toHaveCount(0);
  const check = trakt.getByRole('button', { name: 'Trakt prüfen' });
  await tabTo(page, check, 6);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(
    trakt.getByText('Trakt: Der Anbieter hat geantwortet, der Schlüssel gilt.'),
  ).toBeVisible();
  // Back on the toggle, Space folds the card again.
  await trakt.getByRole('button', { name: 'Zuklappen: Trakt' }).focus();
  await page.keyboard.press('Space');
  await expect(trakt.getByRole('textbox')).toHaveCount(0);
  await expectFocusOn(page, toggle);
});

test('a YouTube key is saved write-only and deleted again, all by keyboard', async ({
  page,
  owner,
}) => {
  const key = YOUTUBE_KEY;
  try {
    await openIntegrations(page, 'keys');
    const keys = region(page, 'YouTube');
    // The card opens by keyboard; its field is the next stop (concept 20, P-12).
    await resetFocus(page);
    await tabTo(page, keys.getByRole('button', { name: 'Einrichten: YouTube' }), 80);
    await page.keyboard.press('Enter');
    const field = keys.getByRole('textbox', { name: 'Schlüssel', exact: true });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, field);
    await page.keyboard.type(key);
    // The one link of the card, then «Speichern».
    await page.keyboard.press('Tab');
    await expectFocusOn(page, keys.getByRole('link', { name: 'Schlüssel bei YouTube holen' }));
    await page.keyboard.press('Tab');
    const save = keys.getByRole('button', { name: 'YouTube speichern' });
    await expectFocusOn(page, save);
    const stored = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' && response.url().endsWith('/admin/settings'),
    );
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(keys.getByText('YouTube gespeichert.')).toBeVisible();
    // Write-only: the field is empty again, only the last four characters come back.
    await expect(field).toHaveValue('');
    await expect(field).toHaveAttribute(
      'placeholder',
      'Gesetzt, endet auf wxyz; leer lassen behält ihn',
    );
    await expect(keys.getByText('Gespeichert', { exact: true })).toBeVisible();
    expect((await settingsOf(owner)).keys.youtube).toEqual({ source: 'stored', hint: 'wxyz' });

    // "Löschen": the dialog takes the focus, Escape closes it, Enter confirms.
    const remove = keys.getByRole('button', { name: 'YouTube löschen' });
    await remove.focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', { name: 'YouTube löschen?' });
    await expect(dialog).toBeVisible();
    await expectVisibleFocus(page);
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, remove);
    await page.keyboard.press('Enter');
    await expect(dialog.getByRole('button', { name: 'Löschen' })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(keys.getByText('YouTube gelöscht.')).toBeVisible();
    await expect(remove).toHaveCount(0);
    // «Löschen …» is gone with the key: the card's «Speichern» takes the focus (ALL-2).
    await expectFocusOn(page, save);
    expect((await settingsOf(owner)).keys.youtube?.source).toBe('none');
    await expectNoFocusTrap(page, 120);
  } finally {
    if ((await settingsOf(owner)).keys.youtube?.source === 'stored') {
      await putSettings(owner, { clear: ['youtube'] });
    }
  }
});

test('the public address and the update check by keyboard', async ({ page, owner }) => {
  const before = await settingsOf(owner);
  try {
    await openIntegrations(page, 'server');
    const address = serverRow(page, 'Öffentliche Adresse');
    const field = address.getByLabel('Adresse');
    // Emptied once reached: a field focused before resetFocus would be where Firefox resumes Tab.
    await resetFocus(page);
    await tabTo(page, field, 160);
    await field.fill('');
    // Not an http(s) address: the field says so and nothing is sent.
    await page.keyboard.type('media.e2e.test');
    await page.keyboard.press('Enter');
    await expect(field).toHaveAttribute('aria-invalid', 'true');
    await expect(address.getByText(/Gib eine Adresse mit http:\/\/ oder https:\/\//)).toBeVisible();
    await field.fill('https://media.e2e.test');
    await field.focus();
    await page.keyboard.press('Enter');
    await expect(address.getByText('Öffentliche Adresse gespeichert.')).toBeVisible();
    expect((await settingsOf(owner)).publicUrl).toBe('https://media.e2e.test');

    // The update check: Space switches it off and on; each change is stored at once.
    const update = serverRow(page, 'Update-Prüfung');
    const toggle = update.getByRole('checkbox', { name: 'Täglich nach neuen Versionen suchen' });
    await expect(toggle).toBeChecked({ checked: before.updateCheckEnabled });
    await toggle.focus();
    await expectVisibleFocus(page);
    await page.keyboard.press('Space');
    await expect(update.getByText(/Update-Prüfung (aus|ein)geschaltet\./)).toBeVisible();
    await expect(toggle).toBeChecked({ checked: !before.updateCheckEnabled });
    expect((await settingsOf(owner)).updateCheckEnabled).toBe(!before.updateCheckEnabled);
    await expectFocusOn(page, toggle);
    await page.keyboard.press('Space');
    await expect(toggle).toBeChecked({ checked: before.updateCheckEnabled });
    expect((await settingsOf(owner)).updateCheckEnabled).toBe(before.updateCheckEnabled);
  } finally {
    await putSettings(owner, {
      updateCheckEnabled: before.updateCheckEnabled,
      ...(before.publicUrlSource === 'stored' && before.publicUrl
        ? { publicUrl: before.publicUrl }
        : before.publicUrlSource === 'env'
          ? {}
          : { clear: ['publicUrl'] }),
    });
  }
});

test('the e-mail server and the Telegram bot are setting rows; the test mail by keyboard', async ({
  page,
}) => {
  // The e2e server sets both in its environment (start-server.mjs): read-only rows here.
  await openIntegrations(page, 'email');
  const email = region(page, 'E-Mail-Server');
  for (const name of ['Server und Port', 'Benutzer', 'Passwort', 'Absender', 'Verschlüsselung']) {
    await expect(email.getByRole('group', { name, exact: true })).toBeVisible();
  }
  // Int-A: host and port share one row, the port on the right of the host.
  const host = email.getByRole('textbox', { name: 'Server und Port', exact: true });
  const port = email.getByRole('textbox', { name: 'Port', exact: true });
  await expect(host).toBeDisabled();
  await expect(host).toHaveValue('127.0.0.1');
  const hostBox = await host.boundingBox();
  const portBox = await port.boundingBox();
  expect(Math.round(portBox?.y ?? -1)).toBe(Math.round(hostBox?.y ?? 0));
  expect(portBox?.x ?? 0).toBeGreaterThan(hostBox?.x ?? 0);
  await expect(email.getByRole('button', { name: 'E-Mail-Server speichern' })).toHaveCount(0);

  // The disabled fields are no Tab stops: Tab goes from the notification link to the test mail.
  await expect(email.getByText(/^Die Testmail geht an .+\.$/)).toBeVisible();
  const link = email.getByRole('link', { name: 'Zu den Benachrichtigungskanälen' });
  const send = email.getByRole('button', { name: 'Testmail senden' });
  await resetFocus(page);
  await tabTo(page, link, 120);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, send);
  await page.keyboard.press('Enter');
  await expect(email.getByText(/^Testmail an .+ gesendet\.$/)).toBeVisible();
  await expectFocusOn(page, send);

  // A token from deploy/.env leaves the bot's name unknown: no «Bot» row, only the token's row.
  await expectNoFocusTrap(page, 200);
  await openIntegrations(page, 'telegram');
  const telegram = region(page, 'Telegram-Bot');
  await expect(telegram.getByRole('group', { name: 'Bot', exact: true })).toHaveCount(0);
  const token = telegram
    .getByRole('group', { name: 'Bot-Token', exact: true })
    .getByRole('textbox', { name: 'Bot-Token', exact: true });
  await expect(token).toBeDisabled();
  await expect(telegram.getByText('In deploy/.env gesetzt', { exact: true })).toBeVisible();
  await expect(telegram.getByRole('button', { name: 'Telegram-Bot speichern' })).toHaveCount(0);
  // Without the «Bot» row the token's own help says what the bot is for.
  await expect(telegram).toContainText('Für Benachrichtigungen per Telegram.');
  await expectNoFocusTrap(page, 200);
});

test('"Schlüssel neu erzeugen" asks first; Escape keeps the keys', async ({ page, owner }) => {
  const before = await settingsOf(owner);
  await openIntegrations(page, 'push');
  const push = region(page, 'Web-Push');
  const rotate = push.getByRole('button', { name: 'Schlüssel neu erzeugen' });
  await resetFocus(page);
  await tabTo(page, rotate, 160);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('alertdialog', { name: 'Neue Web-Push-Schlüssel erzeugen?' });
  await expect(dialog).toContainText('Alle Geräte verlieren ihr Abo');
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, rotate);
  expect(await settingsOf(owner)).toEqual(before);
});

test('a newer release in the manifest shows the update badge; switching the check off removes it (A-5)', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  try {
    // The manifest mock answers 404 by default; a release far ahead of any build version is newer.
    await setUpdateManifest('99.0.0');
    await putSettings(owner, { updateCheckEnabled: true });
    const checked = await owner.post('/api/v1/admin/update/check', { headers: CSRF });
    expect(checked.status(), await checked.text()).toBe(200);
    expect(((await checked.json()) as { available: boolean }).available).toBe(true);

    // On the admin pages the badge sits on the sidebar's «Übersicht» (concept 11), whose one Tab
    // stop is the current area: the arrows go up to it.
    await page.goto('/admin/integrations');
    const nav = page.getByRole('navigation', { name: 'Verwaltung' });
    const badge = nav.getByRole('link', {
      name: 'Übersicht, Update auf Version 99.0.0 verfügbar',
    });
    await expect(badge).toBeVisible();
    await expect(badge).toHaveText(/Update/);
    await expect(badge).toHaveAttribute('href', '/admin');
    await resetFocus(page);
    await tabTo(page, nav.getByRole('link', { name: /^Automatisierung(,|$)/ }), 20);
    await page.keyboard.press('Home');
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, badge);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.getByText('Version 99.0.0 ist verfügbar')).toBeVisible();

    // Switching the daily check off forgets the known release: no badge, no notice.
    await putSettings(owner, { updateCheckEnabled: false });
    await page.reload();
    await expect(page.getByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
    await expect(page.getByRole('link', { name: /Update auf Version/ })).toHaveCount(0);
  } finally {
    await setUpdateManifest(null);
    await putSettings(owner, { updateCheckEnabled: false });
    await putSettings(owner, { updateCheckEnabled: before.updateCheckEnabled });
  }
});

test('browser back leaves the page for the overview', async ({ page }) => {
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
  // Straight from the overview to the page: the search palette of the header (build plan 2.4).
  await page.getByRole('button', { name: 'Bereich suchen' }).click();
  await page.getByRole('combobox', { name: 'Bereich der Verwaltung suchen' }).fill('Integrationen');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
});
