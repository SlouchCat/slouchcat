import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';
import type { SeerrStatus, SeerrUser, WishAccountMapping, WishSettings } from './queries';

// Medien › Wünsche (concept 25, M-24 and M-25): the tabs «Seerr-Benutzer» and «Einstellungen»;
// the Seerr user of an account chosen by hand at once (409 at the row), the settings that take
// effect at once with the rollback of a refused change, and «Wünsche aktiv» locked before a test.

const settings = (overrides: Partial<WishSettings> = {}): WishSettings => ({
  enabled: false,
  allowMovies: true,
  allowShows: true,
  fromSearch: true,
  fromDetail: true,
  alreadyRequested: 'join',
  unmapped: 'fallback',
  newAccountsCanRequest: false,
  allowWithdraw: true,
  pinExternalHideTitle: true,
  provisionAccounts: true,
  syncPassword: true,
  defaultLimit: { movie: { limit: 10, days: 7 }, tv: { limit: 5, days: 7 } },
  ...overrides,
});

const status = (overrides: Partial<SeerrStatus> = {}): SeerrStatus => ({
  state: 'connected',
  instanceChanged: false,
  tested: true,
  counts: { open: 0, queued: 0, unmapped: 0 },
  log: [],
  ...overrides,
});

const user = (id: number, name: string, overrides: Partial<SeerrUser> = {}): SeerrUser => ({
  id,
  displayName: name,
  email: `${name}@example.org`,
  userType: 4,
  isAdmin: false,
  canRequestMovies: true,
  canRequestShows: true,
  autoApprove: true,
  shared: false,
  gone: false,
  quota: {},
  ...overrides,
});

const mapping = (overrides: Partial<WishAccountMapping> = {}): WishAccountMapping => ({
  accountId: 1,
  email: 'besitzer@example.org',
  canRequest: true,
  state: 'none',
  limit: {
    mode: 'default',
    effective: { movie: { limit: 10, days: 7 }, tv: { limit: 5, days: 7 } },
    seerrAdmin: false,
  },
  ...overrides,
});

const USERS = [
  user(2, 'wohnzimmer'),
  user(3, 'kinder', { mappedTo: { accountId: 9, email: 'gast@example.org' } }),
];

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/seerr/status': () => json(status()),
  'GET /admin/wishes/settings': () => json(settings()),
  'GET /admin/wishes/mappings': () =>
    json({ accounts: [mapping()], unlinkedUsers: [USERS[0]], opened: 0, total: 1 }),
  'GET /admin/wishes/mappings/users': () => json(USERS),
  ...extra,
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('Wünsche › Seerr-Benutzer', () => {
  it('chooses a Seerr user at once; a taken one is shown locked', async () => {
    let current = mapping();
    const { calls } = mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({ accounts: [current], unlinkedUsers: [], opened: 0, total: 1 }),
        'PUT /admin/wishes/mappings/accounts/1': () => {
          current = mapping({ seerrUserId: 2, state: 'manual', passwordState: 'external' });
          return json(current);
        },
      }),
    );
    renderApp('/admin/wishes');
    const user = userEvent.setup();
    expect(await screen.findByRole('heading', { level: 1, name: 'Wünsche' })).toBeVisible();
    const toggle = await screen.findByRole('button', { name: 'besitzer@example.org' });
    await user.click(toggle);
    const select = screen.getByRole('combobox', {
      name: 'Seerr-Benutzer für besitzer@example.org',
    });
    expect(
      within(select).getByRole('option', { name: /kinder.*gehört zu gast@example.org/ }),
    ).toBeDisabled();
    await user.selectOptions(select, '2');
    expect(
      await screen.findByText(/besitzer@example.org fordert jetzt als «wohnzimmer» an/),
    ).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ seerrUserId: 2 });
    const row = screen
      .getAllByRole('row')
      .find((entry) => within(entry).queryByText('besitzer@example.org'))!;
    expect(await within(row).findByText('Von Hand zugeordnet')).toBeVisible();
  });

  it('a user taken meanwhile answers 409 at the row and the choice springs back', async () => {
    mockApi(
      routes({
        'PUT /admin/wishes/mappings/accounts/1': () => problem(409, 'seerr_user_taken'),
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'besitzer@example.org' }));
    const select = screen.getByRole('combobox', {
      name: 'Seerr-Benutzer für besitzer@example.org',
    });
    await user.selectOptions(select, '2');
    expect(await screen.findByText(/gehört inzwischen zu einem anderen Konto/)).toBeVisible();
    await waitFor(() => expect(select).toHaveValue(''));
  });

  it('names the accounts without a Seerr user and the rule that applies to them', async () => {
    mockApi(routes({ 'GET /admin/wishes/settings': () => json(settings({ unmapped: 'deny' })) }));
    renderApp('/admin/wishes');
    expect(await screen.findByText('1 Konto ohne Seerr-Benutzer')).toBeVisible();
    expect(screen.getByText(/auf «Nicht erlauben» steht/)).toBeVisible();
  });
});

describe('Wünsche › Einstellungen', () => {
  it('a switch takes effect at once and says so', async () => {
    const { calls } = mockApi(
      routes({
        'PUT /admin/wishes/settings': () => json(settings({ allowMovies: false })),
      }),
    );
    renderApp('/admin/wishes?tab=einstellungen');
    const user = userEvent.setup();
    const movies = await screen.findByRole('switch', { name: 'Filme erlauben' });
    await user.click(movies);
    expect(
      await screen.findByText('«Filme erlauben» ist ausgeschaltet. Gilt sofort.'),
    ).toBeVisible();
    expect(movies).not.toBeChecked();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ allowMovies: false });
  });

  it('a refused change rolls the control back and names the reason', async () => {
    mockApi(routes({ 'PUT /admin/wishes/settings': () => problem(409, 'seerr_not_tested') }));
    renderApp('/admin/wishes?tab=einstellungen');
    const user = userEvent.setup();
    const shows = await screen.findByRole('switch', { name: 'Serien erlauben' });
    await user.click(shows);
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Nicht geändert: Wünsche lassen sich erst nach einem gelungenen Test',
    );
    expect(shows).toBeChecked();
  });

  it('«Wünsche aktiv» is locked before a passed test, with the way to the test', async () => {
    mockApi(
      routes({
        'GET /admin/seerr/status': () => json(status({ tested: false, state: 'untested' })),
      }),
    );
    renderApp('/admin/wishes?tab=einstellungen');
    const active = await screen.findByRole('switch', { name: 'Wünsche aktiv' });
    await waitFor(() => expect(active).toBeDisabled());
    expect(screen.getByText(/Lässt sich erst nach einem gelungenen Test/)).toBeVisible();
    expect(screen.getAllByRole('link', { name: 'Zur Verbindung' })[0]).toHaveAttribute(
      'href',
      '/admin/integrations?item=seerr',
    );
  });

  it('the default limit saves when a field is left; a wrong number is not sent', async () => {
    const { calls } = mockApi(
      routes({
        'PUT /admin/wishes/settings': () =>
          json(
            settings({
              defaultLimit: { movie: { limit: 12, days: 7 }, tv: { limit: 5, days: 7 } },
            }),
          ),
      }),
    );
    renderApp('/admin/wishes?tab=einstellungen');
    const user = userEvent.setup();
    const band = await screen.findByRole('button', { name: 'Limit ab Werk' });
    expect(band).toHaveAttribute('aria-expanded', 'false');
    await user.click(band);
    const count = screen.getByRole('textbox', { name: 'Filme: Anzahl' });
    await user.clear(count);
    await user.type(count, '101');
    await user.tab();
    expect(count).toHaveAttribute('aria-invalid', 'true');
    expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(0);
    await user.clear(count);
    await user.type(count, '12');
    await user.tab();
    expect(await screen.findByText(/Limit ab Werk: 12 Filme \/ 7 Tage/)).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      defaultLimit: { movie: { limit: 12, days: 7 }, tv: { limit: 5, days: 7 } },
    });
  });
});
