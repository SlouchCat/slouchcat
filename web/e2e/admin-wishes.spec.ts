import type { APIRequestContext, APIResponse, Locator, Page } from '@playwright/test';
import {
  acceptInvitation,
  anonymousApi,
  createInvitation,
  removeInvitedAccounts,
} from './accounts';
import { areaTab } from './admin';
import { arrControl, arrReset, runJob, testArr } from './arr';
import { CSRF } from './catalog';
import { GUEST_PASSWORD, OWNER, OWNER_PROFILE, SEERR_BASE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { declineOpenWishes } from './seerr';
import { SEERR_API_KEY, USERS } from './seerr-server.mjs';
import {
  closeWishes,
  openWishes,
  seerrRequests,
  setSeerrRequest,
  type WishesSetup,
} from './wishes';

// Medien › Wünsche (concept 25, M-23, M-24 and M-25) against the real server and the Seerr mock
// (seerr-server.mjs): the tabs «Anfragen» (the default), «Seerr-Benutzer» and «Einstellungen» (`?tab=`), the table of the
// accounts whose rows fold out, the Seerr user chosen and unlinked by hand at once, a user taken
// meanwhile (409 at the row), and the settings that take effect at once, with the rollback of a
// refused change and «Wünsche aktiv» locked before a passed test. All by keyboard, with a visible
// focus and no focus trap. Every test leaves the connection, the mappings and the settings as it
// found them.

const GUEST = { email: 'wuensche-gast@slouchcat.test', password: GUEST_PASSWORD };
const USER = (name: string) => {
  const user = USERS.find((entry) => entry.displayName === name);
  if (!user) throw new Error(`no Seerr user ${name}`);
  return user;
};

async function send(api: APIRequestContext, method: 'put' | 'post', path: string, data?: object) {
  const response = await api[method](`/api/v1${path}`, { headers: CSRF, data });
  expect(response.ok(), await response.text()).toBe(true);
  return response;
}

async function json<T>(api: APIRequestContext, path: string): Promise<T> {
  const response = await api.get(`/api/v1${path}`);
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as T;
}

/** Connects the mock, tests it and waits until the job `seerr.users` read its users. */
async function connect(api: APIRequestContext): Promise<void> {
  await send(api, 'put', '/admin/seerr', { url: SEERR_BASE, apiKey: SEERR_API_KEY, enabled: true });
  await send(api, 'post', '/admin/seerr/test', {});
  await send(api, 'post', '/admin/wishes/mappings/users/refresh');
  await expect
    .poll(async () => (await json<unknown[]>(api, '/admin/wishes/mappings/users')).length, {
      timeout: 15_000,
    })
    .toBeGreaterThanOrEqual(USERS.length);
}

/**
 * Waits until no seerr.provision job is queued, planned or running: a hand choice queues one
 * that writes the limit, and a choice undone before that job ran gets opened in Seerr by it
 * (a server race, reported). Tests undo a choice only after it settled.
 */
async function provisionSettled(api: APIRequestContext): Promise<void> {
  await expect
    .poll(
      async () => {
        let open = 0;
        for (const state of ['queued', 'planned', 'running']) {
          const page = await json<{ total: number }>(
            api,
            `/admin/jobs?type=seerr.provision&state=${state}&limit=1`,
          );
          open += page.total;
        }
        return open;
      },
      { timeout: 15_000 },
    )
    .toBe(0);
}

/** Steers the Seerr mock (`failCreate`, `used`). */
async function seerrControl(body: object): Promise<void> {
  const response = await fetch(`${SEERR_BASE}/__control`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  expect(response.ok).toBe(true);
}

interface Mapping {
  accountId: number;
  email: string;
  seerrUserId?: number;
  state?: string;
}

const mappings = async (api: APIRequestContext) =>
  (await json<{ accounts: Mapping[] }>(api, '/admin/wishes/mappings')).accounts;

let settingsBefore: Record<string, unknown> | undefined;

test.beforeEach(async ({ owner }) => {
  // The mock's users as at the start: a user an earlier test opened or changed is gone again.
  const reset = await fetch(`${SEERR_BASE}/__reset`, { method: 'POST' });
  expect(reset.ok).toBe(true);
  settingsBefore = await json(owner, '/admin/wishes/settings');
});

test.afterEach(async ({ owner }) => {
  await provisionSettled(owner);
  // The wishes a test took over from Seerr are not the owner's to withdraw: Seerr declines them.
  await declineOpenWishes(owner);
  for (const mapping of await mappings(owner)) {
    if (mapping.seerrUserId !== undefined) {
      await send(owner, 'put', `/admin/wishes/mappings/accounts/${mapping.accountId}`, {
        seerrUserId: null,
      });
    }
  }
  if (settingsBefore) await send(owner, 'put', '/admin/wishes/settings', settingsBefore);
  await send(owner, 'put', '/admin/seerr', { clear: true, enabled: false });
});

const tabs = (page: Page) => page.getByRole('tablist', { name: 'Bereiche der Wünsche' });
const rowToggle = (page: Page, email: string) =>
  page.getByRole('table', { name: 'Konten und ihre Seerr-Benutzer' }).getByRole('button', {
    name: email,
    exact: true,
  });
const chooser = (page: Page, email: string) =>
  page.getByRole('combobox', { name: `Seerr-Benutzer für ${email}` });

test('tabs, a row that folds out and a Seerr user chosen and unlinked by keyboard @keyboard', async ({
  page,
  owner,
}) => {
  await connect(owner);
  await page.goto('/admin/wishes?tab=seerr-benutzer');
  await expect(page.getByRole('heading', { level: 1, name: 'Wünsche' })).toBeVisible();
  await expect(areaTab(page, 'Wünsche')).toHaveAttribute('aria-current', 'page');
  const users = tabs(page).getByRole('tab', { name: 'Seerr-Benutzer' });
  const settings = tabs(page).getByRole('tab', { name: 'Einstellungen' });
  await expect(users).toHaveAttribute('aria-selected', 'true');
  const strip = page.getByRole('group', { name: 'Zusammenfassung' });
  await expect(strip.getByText('Konten', { exact: true })).toBeVisible();

  // The tabs: one Tab stop, the arrows move and select, the address follows.
  await resetFocus(page);
  await tabTo(page, users, 40);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, settings);
  await expect(page).toHaveURL(/\/admin\/wishes\?tab=einstellungen$/);
  await expect(page.getByText('Jede Änderung gilt sofort.')).toBeVisible();
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, users);
  await expect(page).toHaveURL(/\/admin\/wishes\?tab=seerr-benutzer$/);

  // The owner's row folds out with Enter; the focus stays on its toggle.
  const toggle = rowToggle(page, OWNER.email);
  await tabTo(page, toggle, 20);
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expectFocusOn(page, toggle);
  const select = chooser(page, OWNER.email);
  await expect(select).toHaveValue('');
  // A user who belongs to nobody is chosen at once; the message says what applies now.
  await tabTo(page, select, 10);
  const wohnzimmer = USER('wohnzimmer');
  await select.selectOption(String(wohnzimmer.id));
  await expect(
    page.getByText(`${OWNER.email} fordert jetzt als «wohnzimmer» an.`, { exact: false }),
  ).toBeVisible();
  const row = page
    .getByRole('table', { name: 'Konten und ihre Seerr-Benutzer' })
    .getByRole('row')
    .filter({ has: page.getByRole('button', { name: OWNER.email, exact: true }) });
  await expect(row.getByText('Von Hand verknüpft')).toBeVisible();
  await expect(row.getByText('Anmeldung über Jellyfin, Emby oder Plex')).toBeVisible();
  await expect(select).toHaveValue(String(wohnzimmer.id));
  // No text shows a placeholder its call did not fill.
  await expect(page.locator('#content')).not.toContainText('{{');

  // «Verknüpfung lösen» acts at once, without a dialog; the user stays in Seerr.
  await provisionSettled(owner);
  const unlink = page.getByRole('button', { name: 'Verknüpfung lösen' });
  await tabTo(page, unlink, 4);
  await page.keyboard.press('Enter');
  await expect(
    page.getByText(`Verknüpfung von ${OWNER.email} gelöst.`, { exact: false }),
  ).toBeVisible();
  await expect(row.getByText('Nicht eröffnet')).toBeVisible();
  expect((await mappings(owner)).find((m) => m.email === OWNER.email)?.seerrUserId).toBeUndefined();

  // Space folds the row away again from its toggle.
  await toggle.focus();
  await page.keyboard.press('Space');
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expectFocusOn(page, toggle);
  await expectNoFocusTrap(page, 60);
});

test('a Seerr user taken meanwhile is refused at the row; ↓ and ↑ move between rows', async ({
  page,
  owner,
  playwright,
  baseURL,
}) => {
  await connect(owner);
  const anonymous = await anonymousApi(playwright, baseURL ?? '');
  const invitation = await createInvitation(owner);
  await acceptInvitation(anonymous, invitation.token, {
    email: GUEST.email,
    password: GUEST.password,
    profileName: 'Gast',
    language: 'de',
  });
  await anonymous.dispose();
  const guest = (await mappings(owner)).find((mapping) => mapping.email === GUEST.email);
  expect(guest, 'the guest account').toBeDefined();

  await page.goto('/admin/wishes?tab=seerr-benutzer');
  const ownerToggle = rowToggle(page, OWNER.email);
  const guestToggle = rowToggle(page, GUEST.email);
  await expect(guestToggle).toBeVisible();
  // ↓ and ↑ move between the rows' toggles.
  await ownerToggle.focus();
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, guestToggle);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, ownerToggle);
  await page.keyboard.press('Enter');

  // Another administrator gives «kinder» to the guest while this page still offers it.
  const kinder = USER('kinder');
  await send(owner, 'put', `/admin/wishes/mappings/accounts/${guest?.accountId}`, {
    seerrUserId: kinder.id,
  });
  const select = chooser(page, OWNER.email);
  await select.selectOption(String(kinder.id));
  await expect(page.getByText(/gehört inzwischen zu einem anderen Konto/)).toBeVisible();
  // The choice springs back, and the list read again names the account it belongs to.
  await expect(select).toHaveValue('');
  await expect(
    select.locator('option', { hasText: `kinder (ID ${kinder.id}) · gehört zu ${GUEST.email}` }),
  ).toBeDisabled();
  expect((await mappings(owner)).find((m) => m.email === OWNER.email)?.seerrUserId).toBeUndefined();
});

test('the settings take effect at once by keyboard; a refused change rolls back @keyboard', async ({
  page,
  owner,
}) => {
  // Without a passed test «Wünsche aktiv» is locked, with the reason and the way to the test.
  await page.goto('/admin/wishes?tab=einstellungen');
  const active = page.getByRole('switch', { name: 'Wünsche aktiv' });
  await expect(active).toBeDisabled();
  await expect(page.getByText(/Lässt sich erst nach einem gelungenen Test/)).toBeVisible();
  await expect(page.getByRole('link', { name: 'Zur Verbindung' }).first()).toBeVisible();

  await connect(owner);
  await page.reload();
  await expect(active).toBeEnabled();

  // A switch: Space changes it at once, the outcome line confirms, a reload shows it stored.
  const movies = page.getByRole('switch', { name: 'Filme erlauben' });
  await expect(movies).toBeChecked();
  await resetFocus(page);
  await tabTo(page, movies, 60);
  await page.keyboard.press('Space');
  await expect(page.getByText('«Filme erlauben» ist ausgeschaltet. Gilt sofort.')).toBeVisible();
  await page.reload();
  await expect(movies).not.toBeChecked();
  await movies.focus();
  await page.keyboard.press('Space');
  await expect(page.getByText('«Filme erlauben» ist eingeschaltet. Gilt sofort.')).toBeVisible();

  // A choice: the band «Regeln» folds with its summary; Enter opens it, the arrows pick
  // «Nicht erlauben» at once and back.
  const rules = page.getByRole('button', { name: 'Regeln' });
  await expect(rules).toHaveAttribute('aria-expanded', 'false');
  await tabTo(page, rules, 20);
  await page.keyboard.press('Enter');
  await expect(rules).toHaveAttribute('aria-expanded', 'true');
  const fallback = page.getByRole('radio', { name: 'Sammelbenutzer' });
  await expect(fallback).toBeChecked();
  await tabTo(page, fallback, 20);
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('radio', { name: 'Nicht erlauben' })).toBeChecked();
  await expect(
    page.getByText('«Konten ohne Seerr-Benutzer»: Nicht erlauben. Gilt sofort.'),
  ).toBeVisible();
  await page.keyboard.press('ArrowUp');
  await expect(fallback).toBeChecked();

  // «Limit ab Werk» starts closed with its summary; Enter opens it, a field saves when left.
  const limit = page.getByRole('button', { name: 'Limit ab Werk' });
  await expect(limit).toHaveAttribute('aria-expanded', 'false');
  await expect(limit).toHaveAccessibleDescription('10 Filme / 7 Tage · 5 Staffeln / 7 Tage');
  await tabTo(page, limit, 20);
  await page.keyboard.press('Enter');
  await expect(limit).toHaveAttribute('aria-expanded', 'true');
  // No text shows a placeholder its call did not fill.
  await expect(page.locator('#content')).not.toContainText('{{');
  const count = page.getByRole('textbox', { name: 'Filme: Anzahl' });
  await tabTo(page, count, 6);
  await page.keyboard.press('Control+A');
  await page.keyboard.type('12');
  await page.keyboard.press('Tab');
  await expect(
    page.getByText('Limit ab Werk: 12 Filme / 7 Tage · 5 Staffeln / 7 Tage.', { exact: false }),
  ).toBeVisible();
  await expect(limit).toHaveAccessibleDescription('12 Filme / 7 Tage · 5 Staffeln / 7 Tage');
  // A number out of range stays in its field with the reason and is not sent.
  await count.fill('101');
  await page.keyboard.press('Tab');
  await expect(count).toHaveAttribute('aria-invalid', 'true');
  await expect(count).toHaveAccessibleDescription(/von 0 bis 100/);

  // The server refuses a change: the switch goes back, the reason stands at the top.
  await page.route('**/api/v1/admin/wishes/settings', (route) =>
    route.request().method() === 'PUT'
      ? route.fulfill({
          status: 500,
          contentType: 'application/problem+json',
          body: JSON.stringify({ title: 'Internal Server Error', status: 500, code: 'internal' }),
        })
      : route.continue(),
  );
  const shows = page.getByRole('switch', { name: 'Serien erlauben' });
  await shows.focus();
  await page.keyboard.press('Space');
  await expect(page.getByRole('alert').filter({ hasText: 'Nicht geändert:' })).toBeVisible();
  await expect(shows).toBeChecked();
  await page.unroute('**/api/v1/admin/wishes/settings');
  await expectNoFocusTrap(page, 80);
});

test('an account opened in Seerr, a waiting one tried again, the password mode and «Alle eröffnen» by keyboard @keyboard', async ({
  page,
  owner,
  playwright,
  baseURL,
}) => {
  const OTHER = { email: 'wuensche-zwei@slouchcat.test', password: GUEST_PASSWORD };
  await connect(owner);
  // The guest arrives while opening is off: both accounts are without a Seerr user.
  await send(owner, 'put', '/admin/wishes/settings', { provisionAccounts: false });
  const anonymous = await anonymousApi(playwright, baseURL ?? '');
  const invitation = await createInvitation(owner);
  await acceptInvitation(anonymous, invitation.token, {
    email: OTHER.email,
    password: OTHER.password,
    profileName: 'Zwei',
    language: 'de',
  });
  await anonymous.dispose();

  try {
    await page.goto('/admin/wishes?tab=seerr-benutzer');
    const all = page.getByRole('button', { name: 'Alle eröffnen' });
    // Opening is off: the button is locked and says why, with the way to the setting.
    await expect(all).toBeDisabled();
    await expect(
      page.getByText('«Neue Konten in Seerr eröffnen» ist ausgeschaltet.'),
    ).toBeVisible();
    await send(owner, 'put', '/admin/wishes/settings', { provisionAccounts: true });
    await page.reload();
    await expect(all).toBeEnabled();

    // The owner's row: Seerr answers with an error, the account waits with the reason.
    const toggle = rowToggle(page, OWNER.email);
    await resetFocus(page);
    await tabTo(page, toggle, 60);
    await page.keyboard.press('Enter');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await seerrControl({ failCreate: true });
    const open = page.getByRole('button', { name: 'In Seerr eröffnen' }).first();
    await tabTo(page, open, 10);
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(`Noch nicht in Seerr eröffnet: ${OWNER.email} wartet`, { exact: false }),
    ).toBeVisible();
    await expect(page.getByText('Wartet auf Seerr', { exact: true })).toBeVisible();
    await expect(page.getByText(/mit einem Fehler geantwortet/)).toBeVisible();

    // «Erneut versuchen» once Seerr answers again opens the account.
    await seerrControl({ failCreate: false });
    await provisionSettled(owner);
    const retry = page.getByRole('button', { name: 'Erneut versuchen' });
    if ((await retry.count()) > 0) {
      await retry.focus();
      await expectVisibleFocus(page);
      await page.keyboard.press('Enter');
    }
    const row = page
      .getByRole('table', { name: 'Konten und ihre Seerr-Benutzer' })
      .getByRole('row')
      .filter({ has: page.getByRole('button', { name: OWNER.email, exact: true }) });
    await expect(row.getByText('Von Slouchcat eröffnet')).toBeVisible();
    await provisionSettled(owner);

    // The password mode switches at once, both ways.
    const passwordOff = page.getByRole('button', { name: 'Passwort nicht abgleichen' });
    // From the top when «Erneut versuchen» went with its account: the warning about accounts
    // without a Seerr user (every such account counts now) adds its link on the way.
    await tabTo(page, passwordOff, 14);
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(`Das Passwort von ${OWNER.email} wird nicht mehr mit Seerr abgeglichen.`),
    ).toBeVisible();
    const passwordNext = page.getByRole('button', {
      name: 'Passwort bei der nächsten Anmeldung neu übernehmen',
    });
    await passwordNext.focus();
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(`Das Passwort von ${OWNER.email} geht bei der nächsten Anmeldung an Seerr.`),
    ).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');

    // «Alle eröffnen» asks and names the counts; Escape keeps everything, Enter opens.
    await all.focus();
    await page.keyboard.press('Enter');
    const question = page.getByRole('alertdialog', { name: 'Alle Konten in Seerr eröffnen?' });
    await expect(question).toBeVisible();
    await expect(
      question.getByText('Über gleiche E-Mail verknüpfen: 0 · In Seerr neu anlegen: 1.'),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(question).toBeHidden();
    await expectFocusOn(page, all);
    await page.keyboard.press('Enter');
    await expect(question).toBeVisible();
    await question.getByRole('button', { name: 'Alle eröffnen' }).focus();
    await page.keyboard.press('Enter');
    await expect(page.getByText('Eröffnen läuft: 0 verknüpfen, 1 neu anlegen.')).toBeVisible();
    await expect
      .poll(async () => (await mappings(owner)).find((m) => m.email === OTHER.email)?.state, {
        timeout: 15_000,
      })
      .toBe('created');
    await provisionSettled(owner);
    await expectNoFocusTrap(page, 80);
  } finally {
    await seerrControl({ failCreate: false });
    await provisionSettled(owner);
    for (const mapping of await mappings(owner)) {
      if (mapping.seerrUserId !== undefined) {
        await send(owner, 'put', `/admin/wishes/mappings/accounts/${mapping.accountId}`, {
          seerrUserId: null,
        });
      }
    }
    await removeInvitedAccounts(owner);
  }
});

// M-23: «Anfragen», the first tab, with a wish made against the TMDB and Seerr mocks
// (e2e/wishes.ts stores the TMDB key for this test only).
test('«Anfragen»: the default tab, a wish row folds out with its history, «Jetzt abgleichen» by keyboard @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    const made = await owner.post('/api/v1/wishes', {
      headers: CSRF,
      data: { media: 'movie', tmdbId: 9003, source: 'detail' },
    });
    expect(made.status(), await made.text()).toBe(201);
    // A series is requested per season: «In Seerr» names its seasons' requests.
    const series = await owner.post('/api/v1/wishes', {
      headers: CSRF,
      data: { media: 'show', tmdbId: 9100, seasons: [1, 2], source: 'detail' },
    });
    expect(series.status(), await series.text()).toBe(201);
    // The rows of these two wishes by their ids: a run before on the same server (the other
    // browser, a retry) left closed wishes of both titles in the list.
    const rowOf = async (response: APIResponse) =>
      page.locator(`tr[data-row-key="${((await response.json()) as { id: number }).id}"]`);
    const filmRow = await rowOf(made);
    const filmId = ((await made.json()) as { id: number }).id;
    await expect
      .poll(async () => (await seerrRequests()).filter((r) => r.mediaId === 9100).length, {
        timeout: 30_000,
      })
      .toBeGreaterThan(0);
    await page.goto('/admin/wishes');
    const fog = (await rowOf(series)).filter({ hasText: 'Nebelhafen' });
    await expect(fog).toContainText(/Anfragen? \d+/);
    await expect(fog).not.toContainText('Noch nicht');
    const requests = tabs(page).getByRole('tab', { name: 'Anfragen' });
    await expect(requests).toHaveAttribute('aria-selected', 'true');
    const table = page.getByRole('table', { name: 'Alle Wünsche des Haushalts' });
    const row = filmRow.filter({ hasText: 'Glasfabrik (2024)' });
    await expect(row.getByText(`${OWNER.email} · ${OWNER_PROFILE}`)).toBeVisible();
    // The table fits beside the sidebar at 1280 px: no sideways scroll in its frame.
    expect(
      await table.locator('xpath=..').evaluate((frame) => frame.scrollWidth - frame.clientWidth),
    ).toBeLessThanOrEqual(0);
    // Also with wider glyphs: the fonts of CI are wider than here (see sidewaysWithWideGlyphs).
    expect(await sidewaysWithWideGlyphs(page, table)).toBeLessThanOrEqual(0);
    await expect(page.locator('#content')).not.toContainText('{{');

    // The filter's toolbar, then the row's toggle; Enter folds it out, the focus stays.
    await resetFocus(page);
    const all = page.getByRole('toolbar', { name: 'Wünsche nach Zustand filtern' });
    await tabTo(page, all.getByRole('button', { name: /^Alle/ }), 60);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, all.getByRole('button', { name: /^Offen/ }));
    await page.keyboard.press('Enter');
    await expect(row).toBeVisible();
    const toggle = row.getByRole('button', { name: /Glasfabrik \(2024\)/ });
    await tabTo(page, toggle, 10);
    await page.keyboard.press('Enter');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await expectFocusOn(page, toggle);
    await expect(page.getByRole('heading', { name: 'Verlauf' })).toBeVisible();

    // «Jetzt abgleichen» queues the sync and says so.
    const sync = page.getByRole('button', { name: 'Jetzt abgleichen' });
    await resetFocus(page);
    await tabTo(page, sync, 60);
    await page.keyboard.press('Enter');
    await expect(page.getByText('Der Abgleich mit Seerr läuft.')).toBeVisible();
    await expectVisibleFocus(page);

    // Seerr reports the film's request as failed: the wish fails, its fold-out offers «Erneut
    // versuchen», which calls retryWish (202) and says so.
    const film = (await seerrRequests()).find((r) => r.mediaId === 9003);
    if (!film) throw new Error('the film was not sent to Seerr');
    await setSeerrRequest(film.id, 4);
    await expect
      .poll(
        async () => {
          await owner.post('/api/v1/admin/seerr/sync', { headers: CSRF });
          const list = (await (await owner.get('/api/v1/admin/wishes?limit=50')).json()) as {
            items: { id: number; state: string }[];
          };
          return list.items.find((wish) => wish.id === filmId)?.state;
        },
        { timeout: 30_000, intervals: [1000] },
      )
      .toBe('failed');
    // «Offen» stays in the address (?item=open) over a reload; the failed wish is under «Alle».
    await page.goto('/admin/wishes');
    const failedRow = filmRow.filter({ hasText: 'Glasfabrik (2024)' });
    const failedToggle = failedRow.getByRole('button', { name: /Glasfabrik \(2024\)/ });
    await resetFocus(page);
    await tabTo(page, failedToggle, 60);
    await page.keyboard.press('Enter');
    const retry = page.getByRole('button', { name: 'Glasfabrik: erneut versuchen' });
    await tabTo(page, retry, 20);
    const retried = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        /\/admin\/wishes\/\d+\/retry$/.test(response.url()),
    );
    await page.keyboard.press('Enter');
    expect((await retried).status()).toBe(202);
    await expect(page.getByText('Glasfabrik wird erneut versucht.')).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');
    await expectNoFocusTrap(page, 120);
  } finally {
    await closeWishes(owner, setup);
  }
});

/** How far the frame of a table scrolls sideways (0 = it fits). */
const sideways = (table: Locator): Promise<number> =>
  table.locator('xpath=..').evaluate((frame) => frame.scrollWidth - frame.clientWidth);

/**
 * The same with every glyph 0.2em wider. The CI runners have neither Noto nor Inter and draw
 * DejaVu Sans, whose texts are wider than on a developer's machine: two tables that fit here
 * scrolled there (16 and 34 px). A table of the administration must not depend on the font of the
 * machine (other languages run wider than German, too), so the wider glyphs show that fault on any
 * machine. 0.12em was not enough: it left the requests table of stage 4 fitting where DejaVu
 * made it scroll; 0.2em made both scroll (9 and 121 px) before their cells could wrap.
 */
async function sidewaysWithWideGlyphs(page: Page, table: Locator): Promise<number> {
  const style = await page.addStyleTag({ content: '* { letter-spacing: 0.2em }' });
  try {
    return await sideways(table);
  } finally {
    await style.evaluate((element) => {
      (element as Element).remove();
    });
  }
}

/** Where the coordinator's visual check finds the screenshots (SLOUCHCAT_E2E_SHOTS), if set. */
const SHOTS = process.env.SLOUCHCAT_E2E_SHOTS;
async function shot(page: Page, name: string): Promise<void> {
  if (!SHOTS) return;
  // A dialog that just opened plays its entry animation first; endless ones (a live dot) do not
  // count.
  await page.waitForFunction(() =>
    document
      .getAnimations()
      .every(
        (animation) =>
          animation.playState !== 'running' ||
          animation.effect?.getComputedTiming().endTime === Infinity,
      ),
  );
  await page.screenshot({ path: `${SHOTS}/${name}.png` });
}

interface ListedWish {
  id: number;
  state: string;
  title: { tmdbId: number };
  progress?: { stage?: string };
  requesters: { tag?: { state: string } }[];
}

const OPEN = ['queued', 'pending', 'processing', 'partial'];
const listed = async (api: APIRequestContext) =>
  (await json<{ items: ListedWish[] }>(api, '/admin/wishes?limit=100')).items;
/** The open wish of a title (an earlier test may have left a closed one beside it). */
const wishOf = (items: ListedWish[], tmdbId: number) =>
  items.find((wish) => wish.title.tmdbId === tmdbId && OPEN.includes(wish.state));

// M-23 stage 4: «Freigeben» and «Ablehnen» passed on to Seerr, the download progress and the
// profile tags read from the Radarr mock (arr-server.mjs) by the jobs, which the test runs at once
// (`runJob`) instead of waiting for their schedule; the two filters of the client and the
// overview's two hints. Glasfabrik waits for Radarr.
test('«Anfragen» stage 4: approve, decline behind a question, progress and tags by keyboard @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(240_000);
  await page.setViewportSize({ width: 1280, height: 900 });
  let setup: WishesSetup | undefined;
  try {
    await arrReset();
    setup = await openWishes(owner);
    // Radarr: Glasvogel downloads, Kupfermond is stalled and refuses its tag, Glasfabrik is not
    // there yet.
    await arrControl('radarr', {
      queue: [
        { title: 9001, state: 'downloading' },
        { title: 9002, state: 'stalled' },
      ],
      failEditor: [9002],
      absent: [9003],
    });
    // «Testen» confirms the hosts from Seerr before any job may call them.
    await testArr(owner, 'radarr');
    await testArr(owner, 'sonarr');
    const made: Record<number, number> = {}; // tmdb id: the id of this test's wish
    for (const data of [
      { media: 'movie', tmdbId: 9001 },
      { media: 'movie', tmdbId: 9002 },
      { media: 'movie', tmdbId: 9003 },
      { media: 'show', tmdbId: 9100, seasons: [1] },
      { media: 'show', tmdbId: 9101, seasons: [1] },
    ]) {
      const response = await owner.post('/api/v1/wishes', {
        headers: CSRF,
        data: { ...data, source: 'detail' },
      });
      expect(response.status(), await response.text()).toBe(201);
      made[data.tmdbId] = ((await response.json()) as { id: number }).id;
    }
    await expect
      .poll(
        async () => {
          const items = await listed(owner);
          return [9001, 9002, 9003, 9100, 9101].map((id) => wishOf(items, id)?.state);
        },
        { timeout: 30_000 },
      )
      .toEqual(['pending', 'pending', 'pending', 'pending', 'pending']);
    for (const tmdbId of [9001, 9002, 9003]) {
      const wish = wishOf(await listed(owner), tmdbId);
      await send(owner, 'post', `/admin/wishes/${wish?.id}/approve`);
    }
    // The jobs read Radarr; a read within 45 s of the last one does nothing, so the poll runs
    // them until the rows are there.
    await expect
      .poll(
        async () => {
          await runJob(owner, 'wishes.tag');
          await runJob(owner, 'wishes.progress');
          const items = await listed(owner);
          return [9001, 9002, 9003].map((id) => {
            const wish = wishOf(items, id);
            return `${wish?.progress?.stage}/${wish?.requesters[0]?.tag?.state}`;
          });
        },
        { timeout: 120_000, intervals: [3000] },
      )
      .toEqual(['download/set', 'stalled/failed', 'undefined/pending']);

    await page.goto('/admin/wishes');
    const table = page.getByRole('table', { name: 'Alle Wünsche des Haushalts' });
    // The rows of this test's wishes by their ids: earlier tests and runs left closed wishes of
    // the same titles beside them.
    const TMDB: Record<string, number> = {
      Glasvogel: 9001,
      Kupfermond: 9002,
      Glasfabrik: 9003,
      Nebelhafen: 9100,
      Glasküste: 9101,
    };
    const row = (title: string) =>
      table.locator(`tr[data-row-key="${made[TMDB[title]]}"]`).filter({ hasText: title });
    await expect(
      row('Nebelhafen').getByRole('button', { name: 'Nebelhafen freigeben' }),
    ).toBeVisible();
    await expect(
      row('Glasküste').getByRole('button', { name: 'Glasküste ablehnen' }),
    ).toBeVisible();
    await expect(row('Glasvogel')).toContainText('Wird heruntergeladen · 45 % · noch 12 min');
    await expect(row('Kupfermond')).toContainText('Download verzögert');
    await expect(row('Kupfermond')).toContainText(/sc-\S+ · fehlgeschlagen/);
    await expect(row('Glasfabrik')).toContainText(/sc-\S+ · ausstehend/);
    // The message of Radarr stands only in the fold-out.
    await expect(page.getByText(/No files found are eligible for import/)).toHaveCount(0);
    // The table fits beside the sidebar at 1280 px: no sideways scroll in its frame.
    expect(
      await table.locator('xpath=..').evaluate((frame) => frame.scrollWidth - frame.clientWidth),
    ).toBeLessThanOrEqual(0);
    // Also with wider glyphs: the fonts of CI are wider than here (see sidewaysWithWideGlyphs).
    expect(await sidewaysWithWideGlyphs(page, table)).toBeLessThanOrEqual(0);
    await expect(page.locator('#content')).not.toContainText('{{');
    await shot(page, 'anfragen-pending-desktop');
    await page.setViewportSize({ width: 390, height: 844 });
    await shot(page, 'anfragen-pending-phone');
    await page.setViewportSize({ width: 1280, height: 900 });

    // Seerr has decided meanwhile: 409 at the row, whose fold-out opens with the reason.
    await page.route('**/api/v1/admin/wishes/*/approve', (route) =>
      route.fulfill({
        status: 409,
        contentType: 'application/problem+json',
        body: JSON.stringify({
          type: 'about:blank',
          title: 'Conflict',
          status: 409,
          code: 'wish_not_pending',
        }),
      }),
    );
    const approve = row('Nebelhafen').getByRole('button', { name: 'Nebelhafen freigeben' });
    await resetFocus(page);
    await tabTo(page, approve, 80);
    await page.keyboard.press('Enter');
    await expect(table.getByText(/^Schon entschieden/)).toBeVisible();
    const fogToggle = row('Nebelhafen').getByRole('button', { name: /^Nebelhafen \(2023\)/ });
    await expect(fogToggle).toHaveAttribute('aria-expanded', 'true');
    await page.unroute('**/api/v1/admin/wishes/*/approve');
    // Approved for real: the buttons go, the focus moves to the row's toggle.
    await expectFocusOn(page, approve);
    await page.keyboard.press('Enter');
    await expect(page.getByText('Nebelhafen ist freigegeben und wird beschafft.')).toBeVisible();
    await expectFocusOn(page, fogToggle);
    await expect(row('Nebelhafen')).toContainText('In Beschaffung');
    await expect(table.getByText('In der Verwaltung freigegeben')).toBeVisible();

    // «Ablehnen» asks first: Escape keeps the wish and returns the focus, Enter declines.
    const decline = row('Glasküste').getByRole('button', { name: 'Glasküste ablehnen' });
    await tabTo(page, decline, 60);
    await page.keyboard.press('Enter');
    const question = page.getByRole('alertdialog', { name: 'Glasküste ablehnen?' });
    await expect(question).toBeVisible();
    await expect(question).toContainText('Die Profile sehen nur «Abgelehnt»');
    await shot(page, 'anfragen-decline-question-desktop');
    await page.keyboard.press('Escape');
    await expect(question).toBeHidden();
    await expectFocusOn(page, decline);
    await page.keyboard.press('Enter');
    await question.getByRole('button', { name: 'Ablehnen' }).focus();
    await page.keyboard.press('Enter');
    await expect(page.getByText('Glasküste ist abgelehnt.')).toBeVisible();
    await expect(row('Glasküste')).toContainText('Abgelehnt');
    const coast = (await seerrRequests()).filter((r) => r.mediaId === 9101);
    expect(coast).toHaveLength(1);

    // The stalled wish folds out by keyboard: progress with Radarr's message, the tag's reason.
    const copper = row('Kupfermond').getByRole('button', { name: /^Kupfermond \(2025\)/ });
    await resetFocus(page);
    await tabTo(page, copper, 80);
    await page.keyboard.press('Enter');
    await expect(copper).toHaveAttribute('aria-expanded', 'true');
    await expectFocusOn(page, copper);
    await expect(page.getByText(/No files found are eligible for import/)).toBeVisible();
    await expect(page.getByText('Radarr hat den Aufruf als ungültig abgelehnt.')).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');
    await page.getByText(/No files found are eligible for import/).scrollIntoViewIfNeeded();
    await shot(page, 'anfragen-stalled-open-desktop');
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByText(/No files found are eligible for import/).scrollIntoViewIfNeeded();
    await shot(page, 'anfragen-stalled-open-phone');
    await page.setViewportSize({ width: 1280, height: 900 });

    // The filters of the client: «Download verzögert» and «Tag nicht gesetzt» by the arrows.
    const filters = page.getByRole('toolbar', { name: 'Wünsche nach Zustand filtern' });
    const stalled = filters.getByRole('button', { name: /^Download verzögert/ });
    await resetFocus(page);
    await tabTo(page, filters.getByRole('button', { name: /^Alle/ }), 60);
    for (let step = 0; step < 4; step += 1) await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, stalled);
    await page.keyboard.press('Enter');
    await expect(row('Glasvogel')).toHaveCount(0);
    await expect(row('Kupfermond')).toBeVisible();
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await expect(row('Kupfermond')).toBeVisible();
    await expect(row('Glasfabrik')).toHaveCount(0);
    await expectNoFocusTrap(page, 120);

    // The overview names both, with the titles and the service.
    await page.goto('/admin');
    const attention = page.getByRole('region', { name: 'Was benötigt Aufmerksamkeit?' });
    await expect(attention.getByText('1 Download verzögert')).toBeVisible();
    await expect(attention.getByText('Kupfermond (Radarr)', { exact: true })).toBeVisible();
    await expect(attention.getByText('1 Tag nicht gesetzt')).toBeVisible();
    await expect(
      attention.getByRole('link', { name: 'Verzögerte Downloads ansehen' }),
    ).toHaveAttribute('href', '/admin/wishes?tab=anfragen&item=stalled');
    await attention.getByText('1 Download verzögert').scrollIntoViewIfNeeded();
    await shot(page, 'uebersicht-hints-desktop');
    await page.setViewportSize({ width: 390, height: 844 });
    await attention.getByText('1 Download verzögert').scrollIntoViewIfNeeded();
    await shot(page, 'uebersicht-hints-phone');
    await page.setViewportSize({ width: 1280, height: 900 });
    // The hint's link opens «Anfragen» with the filter.
    await attention.getByRole('link', { name: 'Verzögerte Downloads ansehen' }).click();
    await expect(filters.getByRole('button', { name: /^Download verzögert/ })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await expect(row('Kupfermond')).toBeVisible();
    await expect(row('Glasvogel')).toHaveCount(0);
  } finally {
    // Leave no hint behind for the specs after this one: the tag set after all, the progress gone.
    await arrControl('radarr', { failEditor: [] });
    await expect
      .poll(
        async () => {
          await runJob(owner, 'wishes.tag');
          return (await listed(owner)).some((wish) =>
            wish.requesters.some((requester) => requester.tag?.state === 'failed'),
          );
        },
        { timeout: 30_000, intervals: [2000] },
      )
      .toBe(false);
    await send(owner, 'put', '/admin/wishes/settings', { showProgress: false });
    await expect
      .poll(
        async () => {
          await runJob(owner, 'wishes.progress');
          return (await listed(owner)).some((wish) => wish.progress !== undefined);
        },
        { timeout: 30_000, intervals: [2000] },
      )
      .toBe(false);
    await arrReset();
    await closeWishes(owner, setup);
  }
});

// M-24 and M-25 stage 4: the tag name of a profile changed in place (wrong characters stay, a
// name of another profile is refused, «Zurücksetzen» goes back), «n Anfragen aus Seerr
// übernehmen» with the count from Seerr, and the band «Radarr und Sonarr: Profil-Tag und
// Fortschritt» of «Einstellungen» with the prefix and the switch of the wishes pages. Last in the
// file: the take-over makes wishes of the requests made in Seerr.
test('profile tags, the take-over and the band of Radarr and Sonarr by keyboard @keyboard', async ({
  page,
  owner,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await connect(owner);
  const wohnzimmer = USER('wohnzimmer');
  const ownerMapping = (await mappings(owner)).find((m) => m.email === OWNER.email);
  await send(owner, 'put', `/admin/wishes/mappings/accounts/${ownerMapping?.accountId}`, {
    seerrUserId: wohnzimmer.id,
  });
  await provisionSettled(owner);
  // Two requests made directly in Seerr as «wohnzimmer».
  for (const mediaId of [9001, 9003]) {
    const made = await fetch(`${SEERR_BASE}/api/v1/request`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Api-Key': SEERR_API_KEY,
        'X-API-User': String(wohnzimmer.id),
      },
      body: JSON.stringify({ mediaType: 'movie', mediaId }),
    });
    expect(made.status).toBe(201);
  }
  const profiles = async () =>
    (
      await json<{
        accounts: { email: string; profiles?: { profileId: number; tagName: string }[] }[];
      }>(owner, '/admin/wishes/mappings')
    ).accounts.find((m) => m.email === OWNER.email)?.profiles ?? [];
  const [profile] = await profiles();
  if (!profile) throw new Error('the owner has a profile');
  const derived = profile.tagName;
  try {
    await page.goto('/admin/wishes?tab=seerr-benutzer');
    const table = page.getByRole('table', { name: 'Konten und ihre Seerr-Benutzer' });
    // The table fits beside the sidebar at 1280 px.
    expect(
      await table.locator('xpath=..').evaluate((frame) => frame.scrollWidth - frame.clientWidth),
    ).toBeLessThanOrEqual(0);
    // Also with wider glyphs: the fonts of CI are wider than here (see sidewaysWithWideGlyphs).
    expect(await sidewaysWithWideGlyphs(page, table)).toBeLessThanOrEqual(0);
    const toggle = rowToggle(page, OWNER.email);
    await resetFocus(page);
    await tabTo(page, toggle, 60);
    await page.keyboard.press('Enter');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');

    // The take-over names the count from Seerr first.
    const take = page.getByRole('button', { name: '2 Anfragen aus Seerr übernehmen' });
    await expect(take).toBeVisible();
    await expect(page.getByText(derived, { exact: true })).toBeVisible();
    await shot(page, 'seerr-benutzer-tags-desktop');
    await page.setViewportSize({ width: 390, height: 844 });
    await take.scrollIntoViewIfNeeded();
    await shot(page, 'seerr-benutzer-tags-phone');
    await page.setViewportSize({ width: 1280, height: 900 });
    await tabTo(page, take, 20);
    await page.keyboard.press('Enter');
    await expect(page.getByText('2 Anfragen von wohnzimmer werden übernommen.')).toBeVisible();

    // «Ändern» opens the field with the name; wrong characters stay there and are not sent.
    const change = page.getByRole('button', { name: `Tag von ${OWNER_PROFILE} ändern` });
    await tabTo(page, change, 20);
    await page.keyboard.press('Enter');
    const field = page.getByRole('textbox', { name: `Tag-Name für ${OWNER_PROFILE}` });
    await expectFocusOn(page, field);
    await expect(field).toHaveValue(derived);
    await page.keyboard.press('Control+A');
    await page.keyboard.type('Haupt!');
    await page.keyboard.press('Enter');
    await expect(field).toHaveAttribute('aria-invalid', 'true');
    // A name another profile has: refused at the field.
    await page.route('**/api/v1/admin/wishes/profiles/*/tag', (route) =>
      route.fulfill({
        status: 409,
        contentType: 'application/problem+json',
        body: JSON.stringify({
          type: 'about:blank',
          title: 'Conflict',
          status: 409,
          code: 'tag_name_taken',
        }),
      }),
    );
    await field.focus();
    await page.keyboard.press('Control+A');
    await page.keyboard.type('e2e-haupt');
    await page.keyboard.press('Enter');
    await expect(page.getByText('Ein anderes Profil hat schon diesen Tag-Namen.')).toBeVisible();
    await page.unroute('**/api/v1/admin/wishes/profiles/*/tag');
    await shot(page, 'seerr-benutzer-tag-edit-desktop');
    await field.focus();
    await page.keyboard.press('Control+A');
    await page.keyboard.type('e2e-haupt');
    await page.keyboard.press('Enter');
    await expect(page.getByText(`${OWNER_PROFILE}: Der Tag heisst jetzt e2e-haupt.`)).toBeVisible();
    await expectFocusOn(page, change);
    await expect(page.getByText('eigener Name')).toBeVisible();
    // «Zurücksetzen» goes back to the name from the prefix. «Ändern» puts the focus into the
    // field once it shows; Tab starts from there.
    await page.keyboard.press('Enter');
    await expectFocusOn(page, field);
    const reset = page.getByRole('button', {
      name: `Tag von ${OWNER_PROFILE} auf den Namen aus dem Präfix zurücksetzen`,
    });
    await tabTo(page, reset, 6);
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(`${OWNER_PROFILE}: Der Tag heisst jetzt ${derived}.`),
    ).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');

    // «Einstellungen»: the band starts closed with its summary; Enter opens it.
    await page.goto('/admin/wishes?tab=einstellungen');
    const band = page.getByRole('button', {
      name: 'Radarr und Sonarr: Profil-Tag und Fortschritt',
    });
    await expect(band).toHaveAttribute('aria-expanded', 'false');
    await expect(band).toHaveAccessibleDescription('Tag an · Präfix sc- · Fortschritt an');
    await resetFocus(page);
    await tabTo(page, band, 80);
    await page.keyboard.press('Enter');
    await expect(band).toHaveAttribute('aria-expanded', 'true');
    const tags = page.getByRole('switch', { name: 'Profil-Tag in Radarr und Sonarr setzen' });
    await tabTo(page, tags, 6);
    await page.keyboard.press('Space');
    await expect(
      page.getByText('«Profil-Tag in Radarr und Sonarr setzen» ist ausgeschaltet. Gilt sofort.'),
    ).toBeVisible();
    await page.keyboard.press('Space');
    await expect(
      page.getByText('«Profil-Tag in Radarr und Sonarr setzen» ist eingeschaltet. Gilt sofort.'),
    ).toBeVisible();
    const prefix = page.getByRole('textbox', { name: 'Präfix' });
    await tabTo(page, prefix, 6);
    await page.keyboard.press('Control+A');
    await page.keyboard.type('Tag_');
    await page.keyboard.press('Tab');
    await expect(prefix).toHaveAttribute('aria-invalid', 'true');
    await expect(prefix).toHaveAccessibleDescription(/1 bis 10 Zeichen/);
    await prefix.focus();
    await page.keyboard.press('Control+A');
    await page.keyboard.type('tag-');
    await page.keyboard.press('Tab');
    await expect(page.getByText(/^Präfix «tag-»: Neue Tags heissen etwa tag-noah/)).toBeVisible();
    await expect(band).toHaveAccessibleDescription('Tag an · Präfix tag- · Fortschritt an');
    // The switch of the wishes pages stands in the band «Woher angefordert werden darf».
    const where = page.getByRole('button', { name: 'Woher angefordert werden darf' });
    await where.focus();
    await page.keyboard.press('Enter');
    await expect(where).toHaveAttribute('aria-expanded', 'true');
    const pages = page.getByRole('switch', { name: 'Seiten Entdecken, Filme, Serien und Suche' });
    await pages.focus();
    await page.keyboard.press('Space');
    await expect(
      page.getByText('«Seiten Entdecken, Filme, Serien und Suche» ist ausgeschaltet. Gilt sofort.'),
    ).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');
    await band.scrollIntoViewIfNeeded();
    await shot(page, 'einstellungen-arr-desktop');
    await page.setViewportSize({ width: 390, height: 844 });
    await band.scrollIntoViewIfNeeded();
    await shot(page, 'einstellungen-arr-phone');
    await page.setViewportSize({ width: 1280, height: 900 });
    // The band folds away with Space; the focus stays on its toggle.
    await band.focus();
    await page.keyboard.press('Space');
    await expect(band).toHaveAttribute('aria-expanded', 'false');
    await expectFocusOn(page, band);
    await expectNoFocusTrap(page, 100);
  } finally {
    await send(owner, 'put', `/admin/wishes/profiles/${profile.profileId}/tag`, { name: null });
  }
});

// The second fix round of stage 4: a Seerr user of the same e-mail that administers Seerr is never
// linked on its own. «Alle eröffnen» names the account that needs a choice by hand; the account
// waits with the reason, and «Trotzdem verknüpfen …» links it only after a question that says what
// it means. By keyboard; the question gives the focus back.
test('a Seerr user that administers Seerr is linked only after a question @keyboard', async ({
  page,
  owner,
  playwright,
  baseURL,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const ADMIN = { email: 'wuensche-verwaltung@slouchcat.test', password: GUEST_PASSWORD };
  await connect(owner);
  await send(owner, 'put', '/admin/wishes/settings', { provisionAccounts: false });
  const anonymous = await anonymousApi(playwright, baseURL ?? '');
  const invitation = await createInvitation(owner);
  await acceptInvitation(anonymous, invitation.token, {
    email: ADMIN.email,
    password: ADMIN.password,
    profileName: 'Verwaltung',
    language: 'de',
  });
  await anonymous.dispose();
  try {
    // Seerr has a user of that e-mail with the right to manage requests.
    await seerrControl({ privileged: ADMIN.email });
    // Its id in Seerr: the server may still list the user of that e-mail an earlier run made
    // (ids go on counting), so the poll waits for this one.
    const seerrUsers = (await (
      await fetch(`${SEERR_BASE}/api/v1/user?take=100&skip=0`, {
        headers: { 'X-Api-Key': SEERR_API_KEY },
      })
    ).json()) as { results: { id: number; email: string }[] };
    const seerrAdmin = seerrUsers.results.find((user) => user.email === ADMIN.email);
    expect(seerrAdmin).toBeDefined();
    // The refresh of connect() may still run, having read the users before this one: a refresh
    // queued meanwhile joins it, so the poll queues one again until the user is there.
    await expect
      .poll(
        async () => {
          await send(owner, 'post', '/admin/wishes/mappings/users/refresh');
          return (await json<{ id: number }[]>(owner, '/admin/wishes/mappings/users')).some(
            (entry) => entry.id === seerrAdmin?.id,
          );
        },
        { timeout: 15_000, intervals: [1000] },
      )
      .toBe(true);
    await send(owner, 'put', '/admin/wishes/settings', { provisionAccounts: true });

    await page.goto('/admin/wishes?tab=seerr-benutzer');
    const all = page.getByRole('button', { name: 'Alle eröffnen' });
    await resetFocus(page);
    await tabTo(page, all, 60);
    await page.keyboard.press('Enter');
    const question = page.getByRole('alertdialog', { name: 'Alle Konten in Seerr eröffnen?' });
    await question.getByRole('button', { name: 'Alle eröffnen' }).focus();
    await page.keyboard.press('Enter');
    await expect(page.getByText(/1 Konto braucht eine Wahl von Hand\./)).toBeVisible();
    await expect
      .poll(async () => (await mappings(owner)).find((m) => m.email === ADMIN.email)?.state, {
        timeout: 15_000,
      })
      .toBe('waiting');
    await provisionSettled(owner);
    await page.reload();

    const toggle = rowToggle(page, ADMIN.email);
    await resetFocus(page);
    await tabTo(page, toggle, 80);
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(
        /^Dieser Seerr-Benutzer verwaltet Seerr\. Slouchcat verknüpft ihn nicht von selbst/,
      ),
    ).toBeVisible();
    await expect(rowToggle(page, ADMIN.email).locator('xpath=ancestor::tr')).toContainText(
      'Wartet auf eine Wahl von Hand',
    );
    // The table with an open row fits beside the sidebar at 1280 px.
    const usersTable = page.getByRole('table', { name: 'Konten und ihre Seerr-Benutzer' });
    expect(
      await usersTable
        .locator('xpath=..')
        .evaluate((frame) => frame.scrollWidth - frame.clientWidth),
    ).toBeLessThanOrEqual(0);
    // Also with wider glyphs: the fonts of CI are wider than here (see sidewaysWithWideGlyphs).
    expect(await sidewaysWithWideGlyphs(page, usersTable)).toBeLessThanOrEqual(0);
    const link = page.getByRole('button', { name: 'Trotzdem verknüpfen …' });
    await tabTo(page, link, 10);
    await page.keyboard.press('Enter');
    const ask = page.getByRole('alertdialog', { name: /^Seerr-Administrator .* verknüpfen\?$/ });
    await expect(ask).toContainText('ohne Freigabe und ohne Limit');
    await shot(page, 'seerr-benutzer-privileged-question-desktop');
    // Escape keeps everything and gives the focus back.
    await page.keyboard.press('Escape');
    await expect(ask).toBeHidden();
    await expectFocusOn(page, link);
    expect(
      (await mappings(owner)).find((m) => m.email === ADMIN.email)?.seerrUserId,
    ).toBeUndefined();
    await page.keyboard.press('Enter');
    await ask.getByRole('button', { name: 'Trotzdem verknüpfen' }).focus();
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(`${ADMIN.email} ist mit dem Seerr-Benutzer «verwaltung» verknüpft.`),
    ).toBeVisible();
    await expect
      .poll(async () => (await mappings(owner)).find((m) => m.email === ADMIN.email)?.seerrUserId)
      .toBeDefined();
    await expect(page.locator('#content')).not.toContainText('{{');
    await expectNoFocusTrap(page, 100);
  } finally {
    await provisionSettled(owner);
    for (const mapping of await mappings(owner)) {
      if (mapping.seerrUserId !== undefined) {
        await send(owner, 'put', `/admin/wishes/mappings/accounts/${mapping.accountId}`, {
          seerrUserId: null,
        });
      }
    }
    await removeInvitedAccounts(owner);
  }
});
