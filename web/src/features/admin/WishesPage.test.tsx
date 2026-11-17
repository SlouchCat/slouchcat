import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';
import type {
  AdminWish,
  SeerrStatus,
  SeerrUser,
  WishAccountMapping,
  WishSettings,
} from './queries';

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
  profileTags: true,
  tagPrefix: 'sc-',
  showProgress: true,
  discoverPages: true,
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
    renderApp('/admin/wishes?tab=seerr-benutzer');
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
    // The limit: films and seasons on a line each, the mode apart; no placeholder left unfilled.
    expect(screen.getByText('10 Filme / 7 Tage')).toBeVisible();
    expect(screen.getByText('5 Staffeln / 7 Tage')).toBeVisible();
    // «Standard» stands in the limit column and in the fold-out.
    expect(screen.getAllByText('Standard')).toHaveLength(2);
    expect(document.body.textContent).not.toContain('{{');
  });

  it('a profile tag changes in place: wrong characters stay, a taken name is refused, reset goes back', async () => {
    let current = mapping({
      seerrUserId: 2,
      state: 'manual',
      passwordState: 'external',
      profiles: [
        { profileId: 1, name: 'Marco', tagName: 'sc-marco', tagNameCustom: false },
        { profileId: 4, name: 'Lena', tagName: 'lena-k', tagNameCustom: true },
      ],
    });
    let taken = true;
    const { calls } = mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({ accounts: [current], unlinkedUsers: [], opened: 1, total: 1 }),
        'GET /admin/wishes/mappings/users/2/requests/count': () => json({ count: 3 }),
        'POST /admin/wishes/mappings/import': () => json({ queued: 1 }, 202),
        'PUT /admin/wishes/profiles/1/tag': () => {
          if (taken) {
            taken = false;
            return problem(409, 'tag_name_taken');
          }
          current = {
            ...current,
            profiles: [
              { profileId: 1, name: 'Marco', tagName: 'sc-marco-2', tagNameCustom: true },
              ...(current.profiles ?? []).slice(1),
            ],
          };
          return json({ profileId: 1, name: 'sc-marco-2', custom: true });
        },
        'PUT /admin/wishes/profiles/4/tag': () =>
          json({ profileId: 4, name: 'sc-lena', custom: false }),
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'besitzer@example.org' }));
    expect(screen.getByText('sc-marco')).toBeVisible();
    expect(screen.getByText('eigener Name')).toBeVisible();

    // «n Anfragen aus Seerr übernehmen» with the count from Seerr; 202 says what runs.
    await user.click(
      await screen.findByRole('button', { name: '3 Anfragen aus Seerr übernehmen' }),
    );
    expect(await screen.findByText('3 Anfragen von wohnzimmer werden übernommen.')).toBeVisible();
    expect(calls.find((call) => call.path === '/admin/wishes/mappings/import')?.body).toEqual({
      seerrUserId: 2,
    });

    await user.click(screen.getByRole('button', { name: 'Tag von Marco ändern' }));
    const field = screen.getByRole('textbox', { name: 'Tag-Name für Marco' });
    await waitFor(() => expect(field).toHaveFocus());
    await user.clear(field);
    await user.type(field, 'Marco!');
    await user.click(screen.getByRole('button', { name: 'Tag von Marco speichern' }));
    expect(field).toHaveAttribute('aria-invalid', 'true');
    expect(calls.some((call) => call.path === '/admin/wishes/profiles/1/tag')).toBe(false);
    await user.clear(field);
    await user.type(field, 'sc-marco-2');
    await user.click(screen.getByRole('button', { name: 'Tag von Marco speichern' }));
    expect(await screen.findByText('Ein anderes Profil hat schon diesen Tag-Namen.')).toBeVisible();
    // Typing again clears the server's reason; the other profile has given the name up meanwhile.
    await user.clear(field);
    await user.type(field, 'sc-marco-2');
    await user.click(screen.getByRole('button', { name: 'Tag von Marco speichern' }));
    expect(await screen.findByText('Marco: Der Tag heisst jetzt sc-marco-2.')).toBeVisible();
    expect(
      calls.filter((call) => call.path === '/admin/wishes/profiles/1/tag').at(-1)?.body,
    ).toEqual({
      name: 'sc-marco-2',
    });

    // A custom name goes back to the one from the prefix.
    await user.click(screen.getByRole('button', { name: 'Tag von Lena ändern' }));
    await user.click(
      screen.getByRole('button', {
        name: 'Tag von Lena auf den Namen aus dem Präfix zurücksetzen',
      }),
    );
    expect(await screen.findByText('Lena: Der Tag heisst jetzt sc-lena.')).toBeVisible();
    expect(calls.find((call) => call.path === '/admin/wishes/profiles/4/tag')?.body).toEqual({
      name: null,
    });
    expect(document.body.textContent).not.toContain('{{');
  });

  it('a Seerr user that administers Seerr is linked only after a question that says what it means', async () => {
    const admin = user(4, 'besitzer', { email: 'besitzer@example.org', isAdmin: true });
    let current = mapping();
    let posts = 0;
    const { calls } = mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({ accounts: [current], unlinkedUsers: [admin], opened: 0, total: 1 }),
        'GET /admin/wishes/mappings/users': () => json([...USERS, admin]),
        // The first call finds the administrator and waits; the confirmed one links it.
        'POST /admin/wishes/accounts/1/provision': () => {
          posts += 1;
          current =
            posts > 1
              ? mapping({
                  seerrUserId: 4,
                  state: 'linked',
                  linkedBy: 'admin',
                  passwordState: 'admin',
                })
              : mapping({ state: 'waiting', provisionError: 'seerr_user_privileged' });
          return json(current, current.state === 'linked' ? 200 : 202);
        },
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer&item=1');
    const user$ = userEvent.setup();
    await user$.click(await screen.findByRole('button', { name: 'In Seerr eröffnen' }));
    const question = await screen.findByRole('alertdialog', {
      name: 'Seerr-Administrator besitzer verknüpfen?',
    });
    expect(within(question).getByText(/ohne Freigabe und ohne Limit/)).toBeVisible();
    await user$.click(within(question).getByRole('button', { name: 'Abbrechen' }));
    expect(
      await screen.findByText(
        /Dieser Seerr-Benutzer verwaltet Seerr\. Slouchcat verknüpft ihn nicht von selbst/,
      ),
    ).toBeVisible();
    await user$.click(screen.getByRole('button', { name: 'Trotzdem verknüpfen …' }));
    const again = await screen.findByRole('alertdialog');
    await user$.click(within(again).getByRole('button', { name: 'Trotzdem verknüpfen' }));
    expect(
      await screen.findByText(
        'besitzer@example.org ist mit dem Seerr-Benutzer «besitzer» verknüpft.',
      ),
    ).toBeVisible();
    expect(calls.filter((call) => call.path === '/admin/wishes/accounts/1/provision')).toHaveLength(
      2,
    );
    expect(document.body.textContent).not.toContain('{{');
  });

  it('«In Seerr eröffnen» names the user of the same e-mail; Seerr away, the account waits', async () => {
    const same = user(4, 'besitzer', { email: 'Besitzer@example.org' });
    const { calls } = mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({ accounts: [mapping()], unlinkedUsers: [same], opened: 0, total: 1 }),
        'GET /admin/wishes/mappings/users': () => json([...USERS, same]),
        'POST /admin/wishes/accounts/1/provision': () =>
          json(mapping({ state: 'waiting', provisionError: 'unreachable' }), 202),
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer&item=1');
    const user$ = userEvent.setup();
    expect(await screen.findByText('In Seerr gibt es schon «besitzer»')).toBeVisible();
    await user$.click(screen.getByRole('button', { name: 'In Seerr eröffnen' }));
    expect(
      await screen.findByText(/Noch nicht in Seerr eröffnet: besitzer@example.org wartet/),
    ).toBeVisible();
    expect(calls.some((call) => call.method === 'POST' && call.path.endsWith('/provision'))).toBe(
      true,
    );
  });

  it('a waiting account names its reason and offers «Erneut versuchen»', async () => {
    mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({
            accounts: [mapping({ state: 'waiting', provisionError: 'timeout' })],
            unlinkedUsers: [],
            opened: 0,
            total: 1,
          }),
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer&item=1');
    expect(await screen.findByText('Wartet auf Seerr')).toBeVisible();
    expect(screen.getByText(/nicht rechtzeitig geantwortet/)).toBeVisible();
    expect(screen.getByRole('button', { name: 'Erneut versuchen' })).toBeEnabled();
  });

  it('opening is locked while «Neue Konten in Seerr eröffnen» is off, with the reason', async () => {
    mockApi(
      routes({ 'GET /admin/wishes/settings': () => json(settings({ provisionAccounts: false })) }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer&item=1');
    expect(await screen.findByRole('button', { name: 'In Seerr eröffnen' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Alle eröffnen' })).toBeDisabled();
    expect(
      screen.getAllByText('«Neue Konten in Seerr eröffnen» ist ausgeschaltet.').length,
    ).toBeGreaterThan(0);
  });

  it('switches the password mode at once; a refusal stands at the row', async () => {
    let answer = () =>
      json(
        mapping({
          seerrUserId: 2,
          state: 'linked',
          linkedBy: 'admin',
          passwordState: 'next_login',
        }),
      );
    const { calls } = mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({
            accounts: [mapping({ seerrUserId: 2, state: 'linked', passwordState: 'off' })],
            unlinkedUsers: [],
            opened: 1,
            total: 1,
          }),
        'GET /admin/wishes/mappings/users': () => json([user(2, 'wohnzimmer', { userType: 2 })]),
        'PUT /admin/wishes/accounts/1/password': () => answer(),
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer&item=1');
    const user$ = userEvent.setup();
    const next = await screen.findByRole('button', {
      name: 'Passwort bei der nächsten Anmeldung neu übernehmen',
    });
    await user$.click(next);
    expect(
      await screen.findByText(/Das Passwort von besitzer@example.org geht bei der nächsten/),
    ).toBeVisible();
    expect(calls.find((call) => call.path.endsWith('/password'))?.body).toEqual({
      mode: 'next_login',
    });
    answer = () => problem(409, 'password_not_syncable');
    await user$.click(
      await screen.findByRole('button', {
        name: 'Passwort bei der nächsten Anmeldung neu übernehmen',
      }),
    );
    expect(await screen.findByText(/gleicht Slouchcat nicht ab: Er ist in Seerr/)).toBeVisible();
  });

  it('shows what the limit has used, the next free place and «Kein Limit» for an administrator', async () => {
    mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({
            accounts: [
              mapping({
                seerrUserId: 2,
                state: 'created',
                passwordState: 'synced',
                provisionError: 'email_taken',
                quota: {
                  unlimited: false,
                  at: '2026-11-17T08:00:00Z',
                  movie: {
                    limit: 10,
                    days: 7,
                    used: 9,
                    queued: 0,
                    remaining: 1,
                    nextFreeAt: '2026-11-20T10:00:00Z',
                  },
                  tv: { limit: 5, days: 7, used: 2, queued: 0, remaining: 3 },
                },
              }),
              mapping({
                accountId: 2,
                email: 'chef@example.org',
                seerrUserId: 3,
                state: 'linked',
                passwordState: 'admin',
                limit: {
                  mode: 'default',
                  effective: { movie: { limit: 10, days: 7 }, tv: { limit: 5, days: 7 } },
                  seerrAdmin: true,
                },
                quota: { unlimited: true, reason: 'admin' },
              }),
            ],
            unlinkedUsers: [],
            opened: 2,
            total: 2,
          }),
        'GET /admin/wishes/mappings/users': () =>
          json([user(2, 'wohnzimmer', { userType: 2 }), user(3, 'chef', { isAdmin: true })]),
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer&item=1');
    expect(await screen.findByText('Filme 9/10')).toBeVisible();
    expect(screen.getByText('Staffeln 2/5')).toBeVisible();
    expect(
      screen.getByText(/Filme: 9 von 10 in 7 Tagen · noch 1 frei · nächster Platz frei am/),
    ).toBeVisible();
    expect(screen.getByText('Staffeln: 2 von 5 in 7 Tagen · noch 3 frei')).toBeVisible();
    expect(screen.getByText('Seerr hat die neue E-Mail abgelehnt')).toBeVisible();
    expect(screen.getByText('Kein Limit')).toBeVisible();
    expect(document.body.textContent).not.toContain('{{');
  });

  it('«Alle eröffnen» asks first and names how many link and how many are opened', async () => {
    const same = user(4, 'besitzer', { email: 'besitzer@example.org' });
    const { calls } = mockApi(
      routes({
        'GET /admin/wishes/mappings': () =>
          json({
            accounts: [mapping(), mapping({ accountId: 2, email: 'neu@example.org' })],
            unlinkedUsers: [same],
            opened: 0,
            total: 2,
          }),
        'GET /admin/wishes/mappings/users': () => json([...USERS, same]),
        'POST /admin/wishes/accounts/provision': () => json({ link: 1, create: 1, manual: 0 }, 202),
      }),
    );
    renderApp('/admin/wishes?tab=seerr-benutzer');
    const user$ = userEvent.setup();
    await user$.click(await screen.findByRole('button', { name: 'Alle eröffnen' }));
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Alle Konten in Seerr eröffnen?',
    });
    expect(
      within(dialog).getByText('Über gleiche E-Mail verknüpfen: 1 · In Seerr neu anlegen: 1.'),
    ).toBeVisible();
    expect(calls.some((call) => call.path.endsWith('/accounts/provision'))).toBe(false);
    await user$.click(within(dialog).getByRole('button', { name: 'Alle eröffnen' }));
    expect(await screen.findByText('Eröffnen läuft: 1 verknüpfen, 1 neu anlegen.')).toBeVisible();
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
    renderApp('/admin/wishes?tab=seerr-benutzer');
    expect(await screen.findByText('1 Konto ohne Seerr-Benutzer')).toBeVisible();
    expect(screen.getByText(/auf «Nicht erlauben» steht/)).toBeVisible();
  });
});

describe('Wünsche › Einstellungen', () => {
  it('the band of Radarr and Sonarr: the prefix saves when left, a wrong one is not sent', async () => {
    const { calls } = mockApi(
      routes({
        'PUT /admin/wishes/settings': () => json(settings({ tagPrefix: 'tag-' })),
      }),
    );
    renderApp('/admin/wishes?tab=einstellungen');
    const user = userEvent.setup();
    const band = await screen.findByRole('button', {
      name: 'Radarr und Sonarr: Profil-Tag und Fortschritt',
    });
    expect(band).toHaveAttribute('aria-expanded', 'false');
    expect(band).toHaveAccessibleDescription('Tag an · Präfix sc- · Fortschritt an');
    await user.click(band);
    expect(
      screen.getByRole('switch', { name: 'Seiten Entdecken, Filme, Serien und Suche' }),
    ).toBeChecked();
    const prefix = screen.getByRole('textbox', { name: 'Präfix' });
    await user.clear(prefix);
    await user.type(prefix, 'Tag_');
    await user.tab();
    expect(prefix).toHaveAttribute('aria-invalid', 'true');
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
    await user.clear(prefix);
    await user.type(prefix, 'tag-');
    await user.tab();
    expect(await screen.findByText(/Präfix «tag-»: Neue Tags heissen etwa tag-noah/)).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ tagPrefix: 'tag-' });
  });

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

const adminWish = (overrides: Partial<AdminWish> = {}): AdminWish => ({
  id: 11,
  title: {
    media: 'movie',
    tmdbId: 9002,
    title: 'Kupfermond',
    year: 2025,
    posterUrl: '/api/v1/admin/titles/movie/9002/images/poster/w200',
  },
  state: 'failed',
  failReason: 'unreachable_24h',
  origin: 'slouchcat',
  seasons: [],
  requesters: [
    {
      accountId: 1,
      accountEmail: 'marco@example.org',
      profileId: 1,
      profileName: 'Marco',
      role: 'owner',
      closed: false,
    },
    {
      accountId: 2,
      accountEmail: 'lena@example.org',
      profileId: 4,
      profileName: 'Lena',
      role: 'joined',
      closed: false,
    },
  ],
  seerrUserId: 3,
  attempts: 9,
  events: [
    {
      at: '2026-11-16T10:00:00Z',
      kind: 'state_changed',
      detail: '{"from":"queued","to":"failed"}',
    },
    { at: '2026-11-15T10:00:00Z', kind: 'created' },
  ],
  createdAt: '2026-11-15T10:00:00Z',
  updatedAt: '2026-11-16T10:00:00Z',
  ...overrides,
});

describe('Wünsche › Anfragen (M-23)', () => {
  it('is the first tab and the default: the wishes with requesters, state and attempts; a row folds out', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/seerr/status': () =>
          json(status({ counts: { open: 1, queued: 1, unmapped: 0 } })),
        'GET /admin/wishes': () =>
          json({
            items: [
              adminWish(),
              adminWish({
                id: 12,
                title: { media: 'show', tmdbId: 9100, title: 'Nebelhafen', posterUrl: '' },
                state: 'pending',
                failReason: undefined,
                attempts: 1,
                seasons: [
                  { number: 1, state: 'pending', seerrRequestId: 104 },
                  { number: 2, state: 'pending', seerrRequestId: 105 },
                ],
              }),
            ],
            total: 2,
            counts: { open: 1, pending: 1, processing: 0, partial: 0, available: 0, closed: 1 },
          }),
        'POST /admin/wishes/11/retry': () => new Response(null, { status: 202 }),
        'POST /admin/seerr/sync': () => new Response(null, { status: 202 }),
      }),
    );
    renderApp('/admin/wishes');
    const tabs = await screen.findByRole('tablist');
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual(['Anfragen', 'Seerr-Benutzer', 'Einstellungen']);
    const table = await screen.findByRole('table', { name: 'Alle Wünsche des Haushalts' });
    const copper = within(table).getByRole('row', { name: /Kupfermond \(2025\)/ });
    expect(within(copper).getByText('marco@example.org · Marco')).toBeVisible();
    expect(within(copper).getByText('lena@example.org · Lena (wünscht mit)')).toBeVisible();
    expect(within(copper).getByText('Fehlgeschlagen')).toBeVisible();
    expect(within(copper).getByText('9')).toBeVisible();
    const fog = within(table).getByRole('row', { name: /Nebelhafen/ });
    expect(within(fog).getByText('Staffeln 1, 2')).toBeVisible();
    // A series is requested per season: the column names its seasons' requests.
    expect(within(fog).getByText('Anfragen 104, 105')).toBeVisible();
    expect(screen.getByText('1 Wunsch wartet auf die Übermittlung an Seerr.')).toBeVisible();

    const user$ = userEvent.setup();
    await user$.click(within(copper).getByRole('button', { name: /Kupfermond/ }));
    expect(await screen.findByText('Seerr war 24 Stunden lang nicht erreichbar.')).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Verlauf' })).toBeVisible();
    await user$.click(screen.getByRole('button', { name: 'Kupfermond: erneut versuchen' }));
    expect(await screen.findByText('Kupfermond wird erneut versucht.')).toBeVisible();
    await user$.click(screen.getByRole('button', { name: 'Jetzt abgleichen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'POST' && call.path === '/admin/seerr/sync'),
      ).toBe(true),
    );
    expect(document.body.textContent).not.toContain('{{');
  });
  it('a refused «Erneut versuchen» shows at its wish; a declined wish offers none', async () => {
    mockApi(
      routes({
        'GET /admin/seerr/status': () => json(status()),
        'GET /admin/wishes': () =>
          json({
            items: [
              adminWish(),
              adminWish({
                id: 13,
                title: { media: 'movie', tmdbId: 9001, title: 'Glasvogel', posterUrl: '' },
                state: 'declined',
                failReason: undefined,
              }),
            ],
            total: 2,
            counts: { open: 0, pending: 0, processing: 0, partial: 0, available: 0, closed: 2 },
          }),
        'POST /admin/wishes/11/retry': () => problem(409, 'wish_not_retryable'),
      }),
    );
    renderApp('/admin/wishes');
    const user = userEvent.setup();
    const table = await screen.findByRole('table', { name: 'Alle Wünsche des Haushalts' });
    await user.click(within(table).getByRole('button', { name: /Glasvogel/ }));
    expect(screen.queryByRole('button', { name: 'Glasvogel: erneut versuchen' })).toBeNull();
    await user.click(within(table).getByRole('button', { name: /Kupfermond/ }));
    const retry = await screen.findByRole('button', { name: 'Kupfermond: erneut versuchen' });
    await user.click(retry);
    const refused = await screen.findByText('Dieser Wunsch lässt sich nicht erneut versuchen.');
    // At the row's fold-out, beside its button, not at the top of the tab.
    expect(table).toContainElement(refused);
  });

  it('approves and declines a pending wish; progress, tags and the two filters of stage 4', async () => {
    const [owner] = adminWish().requesters;
    if (!owner) throw new Error('the fixture has a requester');
    const pending = adminWish({
      id: 14,
      title: { media: 'movie', tmdbId: 9003, title: 'Glasfabrik', year: 2024, posterUrl: '' },
      state: 'pending',
      failReason: undefined,
      seerrUserName: 'kinder',
      seerrRequestId: 101,
      requesters: [owner],
    });
    const stalled = adminWish({
      state: 'processing',
      failReason: undefined,
      requesters: [
        {
          ...owner,
          tag: { name: 'sc-marco', service: 'radarr', state: 'failed', lastError: 'validation' },
        },
      ],
      progress: {
        stage: 'stalled',
        service: 'radarr',
        stale: false,
        readAt: '2026-11-17T10:00:00Z',
        message: 'No files found are eligible for import; (SABnzbd)',
      },
    });
    const loading = adminWish({
      id: 15,
      title: { media: 'movie', tmdbId: 9001, title: 'Glasvogel', posterUrl: '' },
      state: 'processing',
      failReason: undefined,
      requesters: [{ ...owner, tag: { name: 'sc-marco', service: 'radarr', state: 'set' } }],
      progress: {
        stage: 'download',
        percent: 45,
        etaSeconds: 720,
        episodes: { done: 8, total: 10 },
        service: 'radarr',
        stale: false,
        readAt: '2026-11-17T10:00:00Z',
      },
    });
    const old = adminWish({
      id: 16,
      title: { media: 'movie', tmdbId: 9004, title: 'Altlicht', posterUrl: '' },
      state: 'processing',
      failReason: undefined,
      requesters: [{ ...owner, tag: { name: 'sc-marco', service: 'radarr', state: 'pending' } }],
      progress: { stage: 'search', service: 'radarr', stale: true, readAt: '2026-11-17T09:00:00Z' },
    });
    let approved = false;
    const { calls } = mockApi(
      routes({
        'GET /admin/wishes': () =>
          json({
            items: [
              approved ? { ...pending, state: 'processing' } : pending,
              stalled,
              loading,
              old,
            ],
            total: 4,
            counts: { open: 4, pending: 1, processing: 3, partial: 0, available: 0, closed: 0 },
          }),
        'POST /admin/wishes/14/approve': () => {
          if (!approved) {
            approved = true;
            return problem(409, 'wish_not_pending');
          }
          return json({ ...pending, state: 'processing' });
        },
        'POST /admin/wishes/14/decline': () => json({ ...pending, state: 'declined' }),
      }),
    );
    renderApp('/admin/wishes');
    const user$ = userEvent.setup();
    const table = await screen.findByRole('table', { name: 'Alle Wünsche des Haushalts' });
    const glass = within(table).getByRole('row', { name: /Glasfabrik/ });
    expect(within(glass).getByText('kinder')).toBeVisible();
    const copper = within(table).getByRole('row', { name: /Kupfermond/ });
    expect(within(copper).getByText('Download verzögert')).toBeVisible();
    expect(within(copper).getByText('sc-marco · fehlgeschlagen')).toBeVisible();
    // The message of Radarr stands only in the fold-out.
    expect(screen.queryByText(/No files found/)).toBeNull();
    const bird = within(table).getByRole('row', { name: /Glasvogel/ });
    expect(
      within(bird).getByText('Wird heruntergeladen · 45 % · noch 12 min · 8 von 10 Folgen da'),
    ).toBeVisible();
    expect(within(bird).getByText('sc-marco')).toBeVisible();
    const light = within(table).getByRole('row', { name: /Altlicht/ });
    expect(within(light).getByText(/^Fortschritt nicht lesbar · zuletzt gelesen /)).toBeVisible();
    expect(within(light).getByText('sc-marco · ausstehend')).toBeVisible();

    // «Ablehnen» asks first; «Abbrechen» keeps the wish.
    await user$.click(within(glass).getByRole('button', { name: 'Glasfabrik ablehnen' }));
    const question = await screen.findByRole('alertdialog', { name: 'Glasfabrik ablehnen?' });
    await user$.click(within(question).getByRole('button', { name: 'Abbrechen' }));
    expect(calls.some((call) => call.path === '/admin/wishes/14/decline')).toBe(false);

    // A refused approval opens the row with the reason.
    await user$.click(within(glass).getByRole('button', { name: 'Glasfabrik freigeben' }));
    const refused = await screen.findByText(/Schon entschieden/);
    expect(table).toContainElement(refused);
    expect(within(glass).getByRole('button', { name: /Glasfabrik/, expanded: true })).toBeVisible();

    await user$.click(within(copper).getByRole('button', { name: /Kupfermond/ }));
    expect(
      await screen.findByText('No files found are eligible for import; (SABnzbd)'),
    ).toBeVisible();
    expect(screen.getByText('Radarr hat den Aufruf als ungültig abgelehnt.')).toBeVisible();

    // The two filters of the client: only the stalled wish, only the wish with a failed tag.
    const filter = screen.getByRole('toolbar', { name: 'Wünsche nach Zustand filtern' });
    await user$.click(within(filter).getByRole('button', { name: /^Download verzögert/ }));
    await waitFor(() => expect(within(table).queryByRole('row', { name: /Glasvogel/ })).toBeNull());
    expect(within(table).getByRole('row', { name: /Kupfermond/ })).toBeVisible();
    await user$.click(within(filter).getByRole('button', { name: /^Tag nicht gesetzt/ }));
    expect(within(table).getByRole('row', { name: /Kupfermond/ })).toBeVisible();
    expect(within(table).queryByRole('row', { name: /Altlicht/ })).toBeNull();
    expect(document.body.textContent).not.toContain('{{');
  });
});
