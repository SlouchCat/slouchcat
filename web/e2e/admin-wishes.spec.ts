import type { APIRequestContext, Page } from '@playwright/test';
import { acceptInvitation, anonymousApi, createInvitation } from './accounts';
import { areaTab } from './admin';
import { CSRF } from './catalog';
import { GUEST_PASSWORD, OWNER, SEERR_BASE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import { SEERR_API_KEY, USERS } from './seerr-server.mjs';

// Medien › Wünsche (concept 25, M-24 and M-25) against the real server and the Seerr mock
// (seerr-server.mjs): the tabs «Seerr-Benutzer» and «Einstellungen» (`?tab=`), the table of the
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

interface Mapping {
  accountId: number;
  email: string;
  seerrUserId?: number;
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
  await page.goto('/admin/wishes');
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
  await expect(page).toHaveURL(/\/admin\/wishes(\?tab=seerr-benutzer)?$/);

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
  await expect(row.getByText('Von Hand zugeordnet')).toBeVisible();
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

  // A choice: the arrows pick «Nicht erlauben» at once and back.
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
