import type { AdminAccount, Invitation } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountLibraries, profiles } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';
import { fill } from '@/test/fill';

// A-2 with ADR 0014 and concept 12 (variant C with KP-1): every account in one table with its
// role and the owner badge, each row folding out. The signed-in account changes its own e-mail
// and password and manages its profiles in its fold-out; the administration changes the access
// of the others inline, and the owner alone appoints and demotes. The page shows or hides
// buttons, the server decides - every test that checks a hidden button also checks that no
// request went out.

const owner: AdminAccount = {
  id: 1,
  email: 'owner@example.test',
  role: 'admin',
  isOwner: true,
  maxRating: 18,
  allLibraries: true,
  libraryIds: [],
  profiles: [
    { id: 1, name: 'Marco', maxRating: 18, isPrimary: true },
    { id: 2, name: 'Kinder', maxRating: 6, isPrimary: false },
  ],
  lastSeenAt: '2026-10-21T08:30:00Z',
  createdAt: '2026-10-01T10:00:00Z',
  limits: {},
  channels: [
    { channel: 'email', enabled: true },
    { channel: 'push', enabled: true },
    { channel: 'telegram', enabled: false },
  ],
};

const guest: AdminAccount = {
  id: 7,
  email: 'gast@example.test',
  role: 'user',
  isOwner: false,
  maxRating: 12,
  allLibraries: false,
  libraryIds: [1],
  profiles: [{ id: 9, name: 'Lena', maxRating: 12, isPrimary: true }],
  createdAt: '2026-10-10T10:00:00Z',
  limits: { maxStreams: 2 },
  channels: [{ channel: 'email', enabled: false }],
};

const helper: AdminAccount = {
  ...guest,
  id: 8,
  email: 'admin2@example.test',
  role: 'admin',
  maxRating: 18,
  allLibraries: true,
  libraryIds: [],
  profiles: [
    { id: 11, name: 'Sandra', maxRating: 18, isPrimary: true },
    { id: 12, name: 'Kino', maxRating: 16, isPrimary: false },
  ],
  limits: {},
};

const invitation = (overrides: Partial<Invitation> = {}): Invitation => ({
  id: 1,
  note: 'Oma',
  state: 'open',
  maxRating: 12,
  allLibraries: true,
  libraryIds: [],
  createdAt: '2026-10-19T10:00:00Z',
  expiresAt: '2026-10-26T10:00:00Z',
  ...overrides,
});

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /libraries': () => json(accountLibraries),
  'GET /admin/accounts': () => json([owner, guest]),
  'GET /admin/invitations': () => json([]),
  'GET /profiles': () => json(profiles),
  ...extra,
});

const table = () => screen.getByRole('table', { name: 'Konten' });
const rowOf = (email: string) =>
  within(table()).getByRole('rowheader', { name: email }).closest('tr')!;
const toggleOf = (email: string) => within(rowOf(email)).getByRole('button', { name: email });
/** The fold-out of a row: the cell the row's button controls. */
const foldOut = (email: string) =>
  document.getElementById(toggleOf(email).getAttribute('aria-controls')!)!;

/** Opens «Weitere Aktionen» of an account's row and returns the menu (S-6). */
async function openMenu(user: UserEvent, email: string) {
  await user.click(
    within(rowOf(email)).getByRole('button', { name: `Weitere Aktionen für ${email}` }),
  );
  return screen.findByRole('menu');
}

/** Picks `item` from «Weitere Aktionen» of an account's row. */
async function pick(user: UserEvent, email: string, item: string) {
  await user.click(within(await openMenu(user, email)).getByRole('menuitem', { name: item }));
}

/** Opens the row of `email` (a click on its e-mail) and returns its fold-out. */
async function openRow(user: UserEvent, email: string) {
  await user.click(toggleOf(email));
  expect(toggleOf(email)).toHaveAttribute('aria-expanded', 'true');
  return foldOut(email);
}

async function openOwn() {
  const user = userEvent.setup();
  renderApp('/admin/accounts');
  await screen.findByRole('table', { name: 'Konten' });
  const own = await openRow(user, 'owner@example.test');
  const form = within(own).getByRole('region', { name: 'Konto ändern' });
  return { user, form };
}

describe('admin accounts (A-2, KP-1)', { timeout: 15_000 }, () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists every account with role, rating, libraries, profiles and channels', async () => {
    mockApi(routes());
    renderApp('/admin/accounts');
    expect(await screen.findByRole('heading', { level: 1, name: 'Konten' })).toBeVisible();
    await screen.findByRole('table', { name: 'Konten' });
    expect(
      within(table())
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual([
      'E-Mail',
      'Rolle',
      'Altersstufe',
      'Bibliotheken',
      'Profile',
      'Zuletzt gesehen',
      'Kanäle',
      'Aktionen',
    ]);
    const cells = (email: string) =>
      within(rowOf(email))
        .getAllByRole('cell')
        .map((cell) => cell.textContent);
    expect(cells('owner@example.test')).toEqual([
      'AdministratorBesitzer',
      'FSK 18',
      'Alle Bibliotheken',
      'Marco (Hauptprofil), Kinder',
      expect.stringMatching(/2026/) as string,
      'E-MailWeb-Push',
      '',
    ]);
    expect(cells('gast@example.test')).toEqual([
      'Konto',
      'FSK 12',
      '1 Bibliothek',
      'Lena (Hauptprofil)',
      'Noch nie',
      'Keine',
      '',
    ]);
    expect(within(rowOf('owner@example.test')).getByText('Besitzer')).toBeVisible();
    // Another account has «Weitere Aktionen»; the own row (the owner) has none (ADR 0014).
    expect(
      within(rowOf('gast@example.test')).getByRole('button', {
        name: 'Weitere Aktionen für gast@example.test',
      }),
    ).toBeVisible();
    expect(
      within(rowOf('owner@example.test')).queryByRole('button', { name: /Weitere/ }),
    ).toBeNull();
    // Every row folds out on its e-mail; all start closed.
    expect(toggleOf('owner@example.test')).toHaveAttribute('aria-expanded', 'false');
    expect(toggleOf('gast@example.test')).toHaveAttribute('aria-expanded', 'false');
  });

  it('an unknown ?tab= shows «Konten» and leaves the address', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/accounts?tab=nothing');
    expect(await screen.findByRole('tab', { name: /^Konten/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await waitFor(() => expect(router.state.location.search).toEqual({}));
  });

  it('shows four numbers above the tabs: accounts, administrators, profiles, open invitations', async () => {
    mockApi(
      routes({
        'GET /admin/accounts': () => json([owner, helper, guest]),
        'GET /admin/invitations': () =>
          json([
            invitation({ id: 1, expiresAt: '2026-10-28T10:00:00Z' }),
            invitation({ id: 2, expiresAt: '2026-10-25T10:00:00Z' }),
            invitation({ id: 3, state: 'used' }),
          ]),
      }),
    );
    renderApp('/admin/accounts');
    const strip = await screen.findByRole('group', { name: 'Zusammenfassung' });
    const tiles = within(strip)
      .getAllByRole('term')
      .map((term) => [term.textContent, term.nextElementSibling?.textContent]);
    expect(tiles).toEqual([
      ['Konten', '3'],
      ['Administratoren', '2'],
      ['Profile', '5'],
      ['Offene Einladungen', '2'],
    ]);
    expect(within(strip).getByText('davon 1 Besitzer')).toBeVisible();
    expect(within(strip).getByText('2 eigene')).toBeVisible();
    expect(within(strip).getByText(/^Nächste gültig bis 25\.10\.2026/)).toBeVisible();
    // The tabs carry their numbers.
    expect(screen.getByRole('tab', { name: 'Konten 3 Konten' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByRole('tab', { name: 'Einladungen 2 offen' })).toBeVisible();
  });

  it('folds the own account out into «Konto ändern» and «Meine Profile», nothing to appoint or remove', async () => {
    mockApi(routes());
    const { form } = await openOwn();
    const own = foldOut('owner@example.test');
    expect(within(form).getByRole('textbox', { name: 'E-Mail-Adresse' })).toHaveValue(
      'owner@example.test',
    );
    // The own profiles with every action (PRO-1 to PRO-3), names below the h3 (S-3).
    const mine = within(own).getByRole('region', { name: 'Meine Profile' });
    expect(within(mine).getByRole('heading', { level: 3, name: 'Meine Profile' })).toBeVisible();
    expect(within(mine).getByRole('button', { name: 'Profil hinzufügen' })).toBeVisible();
    const marco = await within(mine).findByRole('article', { name: 'Marco' });
    expect(within(marco).queryByRole('heading')).toBeNull();
    expect(within(marco).getByRole('button', { name: 'Bearbeiten' })).toBeVisible();
    expect(within(mine).getByRole('article', { name: 'Lena' })).toBeVisible();
    // The owner's own row offers neither a role action nor a removal: it is protected.
    expect(within(own).queryByRole('button', { name: /ernennen|entziehen|entfernen/ })).toBeNull();
    expect(
      within(rowOf('owner@example.test')).queryByRole('button', { name: /Weitere/ }),
    ).toBeNull();
  });

  it('folds another account out into its access and profile names; appoint and remove in the menu', async () => {
    mockApi(routes({ 'GET /admin/accounts': () => json([owner, helper, guest]) }));
    renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();

    const other = await openRow(user, 'gast@example.test');
    expect(
      within(other).getByRole('region', { name: 'Zugriff von gast@example.test' }),
    ).toBeVisible();
    const names = within(other).getByRole('region', { name: 'Profile von gast@example.test' });
    expect(within(names).getByText('Lena (Hauptprofil)')).toBeVisible();
    expect(within(names).getByText('Profile verwaltet jedes Konto selbst.')).toBeVisible();
    expect(within(names).queryByRole('button')).toBeNull();
    expect(within(other).queryByRole('region', { name: 'Konto ändern' })).toBeNull();
    // The fold-out keeps only the buttons of its form; appoint and remove are in the menu, the
    // removal under «Gefahr».
    expect(
      within(other)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['Zugriff speichern']);
    let menu = await openMenu(user, 'gast@example.test');
    expect(
      within(menu)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(['Zum Administrator ernennen', 'Konto entfernen']);
    expect(within(menu).getByText('Gefahr')).toBeVisible();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull());

    // Another administrator: no access form (fixed at FSK 18 and all libraries), the hint instead.
    const admin = await openRow(user, 'admin2@example.test');
    expect(within(admin).queryByRole('region', { name: /^Zugriff von/ })).toBeNull();
    expect(within(admin).getByText(/hat immer FSK 18 und alle Bibliotheken/)).toBeVisible();
    expect(within(admin).getByText('Sandra (Hauptprofil)')).toBeVisible();
    expect(within(admin).getByText('Kino')).toBeVisible();
    menu = await openMenu(user, 'admin2@example.test');
    expect(
      within(menu)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(['Rolle entziehen', 'Konto entfernen']);
  });

  it('changes the access of an account inline: rating, libraries and the three W-8 caps', async () => {
    const { calls } = mockApi(
      routes({ 'PATCH /admin/accounts/7': () => new Response(null, { status: 204 }) }),
    );
    renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();

    const other = await openRow(user, 'gast@example.test');
    const form = within(other).getByRole('region', { name: 'Zugriff von gast@example.test' });
    expect(within(form).getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    expect(within(form).getByRole('checkbox', { name: 'Filme' })).toBeChecked();
    expect(within(form).getByText('Gilt für alle Profile dieses Kontos.')).toBeVisible();
    // W-8 is an account cap; the watch time is a profile setting and has no place here.
    expect(within(form).queryByLabelText('Schauzeit pro Tag (Minuten)')).not.toBeInTheDocument();
    expect(within(form).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    await user.click(within(form).getByRole('radio', { name: 'FSK 6' }));
    await user.click(within(form).getByRole('radio', { name: '480p' }));
    expect(within(form).getByRole('button', { name: 'Verwerfen' })).toBeVisible();
    await user.click(within(form).getByRole('button', { name: 'Zugriff speichern' }));
    expect(await screen.findByText('Zugriff von gast@example.test gespeichert.')).toBeVisible();
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
      maxRating: 6,
      allLibraries: false,
      libraryIds: [1],
      limits: { maxHeight: 480, maxStreams: 2 },
    });
    // The draft is spent: «Verwerfen» goes, the focus stays on the save button.
    await waitFor(() =>
      expect(within(form).queryByRole('button', { name: 'Verwerfen' })).toBeNull(),
    );
    expect(within(form).getByRole('button', { name: 'Zugriff speichern' })).toHaveFocus();
  });

  it('«Zugriff speichern» without a change sends nothing and keeps the focus', async () => {
    const { calls } = mockApi(routes());
    renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    const other = await openRow(user, 'gast@example.test');
    const form = within(other).getByRole('region', { name: 'Zugriff von gast@example.test' });
    const save = within(form).getByRole('button', { name: 'Zugriff speichern' });
    await user.click(save);
    expect(save).toHaveFocus();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
  });

  it('keeps an access change made while its save waits for the server', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { calls } = mockApi(
      routes({
        'PATCH /admin/accounts/7': async () => {
          await answered;
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    const other = await openRow(user, 'gast@example.test');
    const form = within(other).getByRole('region', { name: 'Zugriff von gast@example.test' });
    await user.click(within(form).getByRole('radio', { name: 'FSK 6' }));
    await user.click(within(form).getByRole('button', { name: 'Zugriff speichern' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'PATCH')).toBe(true));
    // Chosen while the save runs: it stays a draft once the answer is there.
    await user.click(within(form).getByRole('radio', { name: '480p' }));
    release();
    expect(await screen.findByText('Zugriff von gast@example.test gespeichert.')).toBeVisible();
    expect(within(form).getByRole('radio', { name: '480p' })).toBeChecked();
    expect(within(form).getByRole('button', { name: 'Verwerfen' })).toBeVisible();
  });

  it('keeps an unsaved access over folding and tabs, asks before leaving, and discards', async () => {
    const { calls } = mockApi(routes());
    renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    const access = () =>
      within(foldOut('gast@example.test')).getByRole('region', {
        name: 'Zugriff von gast@example.test',
      });

    await openRow(user, 'gast@example.test');
    await user.click(within(access()).getByRole('radio', { name: 'FSK 16' }));
    // Folding the row away and out again, and another tab and back: the draft stays (S-17).
    await user.click(toggleOf('gast@example.test'));
    await openRow(user, 'gast@example.test');
    expect(within(access()).getByRole('radio', { name: 'FSK 16' })).toBeChecked();
    await user.click(screen.getByRole('tab', { name: /^Einladungen/ }));
    await user.click(screen.getByRole('tab', { name: /^Konten/ }));
    expect(within(access()).getByRole('radio', { name: 'FSK 16' })).toBeChecked();

    // Leaving the page with the draft asks; staying keeps it.
    await user.click(
      within(screen.getByRole('navigation', { name: 'Verwaltung' })).getByRole('link', {
        name: /^Geräte/,
      }),
    );
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByRole('heading', { level: 1, name: 'Konten' })).toBeVisible();

    // «Verwerfen» takes the stored values back; nothing was sent.
    await user.click(within(access()).getByRole('button', { name: 'Verwerfen' }));
    expect(within(access()).getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    expect(within(access()).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    expect(within(access()).getByRole('button', { name: 'Zugriff speichern' })).toHaveFocus();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
  });

  it('appoints and removes, and says what the server answered', async () => {
    let roleAnswer = () => new Response(null, { status: 204 });
    let listed = [owner, guest];
    const { calls } = mockApi(
      routes({
        'GET /admin/accounts': () => json(listed),
        'PUT /admin/accounts/7/role': () => roleAnswer(),
        'DELETE /admin/accounts/7': () => {
          listed = [owner];
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    await pick(user, 'gast@example.test', 'Zum Administrator ernennen');
    const confirm = await screen.findByRole('alertdialog', {
      name: 'gast@example.test zum Administrator ernennen?',
    });
    expect(confirm).toHaveTextContent('FSK 18');
    await user.click(within(confirm).getByRole('button', { name: 'Ernennen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ role: 'admin' }),
    );
    expect(await screen.findByText('gast@example.test ist jetzt Administrator.')).toBeVisible();
    // The confirmation hands the focus back to the menu button of the row.
    const more = within(rowOf('gast@example.test')).getByRole('button', {
      name: 'Weitere Aktionen für gast@example.test',
    });
    await waitFor(() => expect(more).toHaveFocus());

    // The server has the last word: a refusal is shown, not swallowed.
    roleAnswer = () => problem(409, 'last_admin');
    await pick(user, 'gast@example.test', 'Zum Administrator ernennen');
    const again = await screen.findByRole('alertdialog', {
      name: 'gast@example.test zum Administrator ernennen?',
    });
    await user.click(within(again).getByRole('button', { name: 'Ernennen' }));
    expect(
      await screen.findByText('Das ist der letzte Administrator. Ernenne zuerst einen zweiten.'),
    ).toBeVisible();

    await pick(user, 'gast@example.test', 'Konto entfernen');
    const removal = await screen.findByRole('alertdialog', {
      name: 'Konto gast@example.test entfernen?',
    });
    expect(removal).toHaveTextContent('enden sofort');
    await user.click(within(removal).getByRole('button', { name: 'Konto entfernen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
    expect(await screen.findByText('Konto gast@example.test entfernt.')).toBeVisible();
    // The row went with its buttons: the h1 takes the focus (ALL-2).
    await waitFor(() =>
      expect(within(table()).queryByRole('rowheader', { name: 'gast@example.test' })).toBeNull(),
    );
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Konten' })).toHaveFocus(),
    );
  });

  it('hides the role actions from an administrator that is not the owner', async () => {
    const { calls } = mockApi(
      routes({
        'GET /auth/session': () =>
          json({ ...session, account: { ...session.account, isOwner: false } }),
      }),
    );
    renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    const other = await openRow(user, 'gast@example.test');
    // Access and removal stay: they are administration, not ownership.
    expect(
      within(other).getByRole('region', { name: 'Zugriff von gast@example.test' }),
    ).toBeVisible();
    const menu = await openMenu(user, 'gast@example.test');
    expect(
      within(menu)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(['Konto entfernen']);
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('changes the e-mail with the current password; the session follows', async () => {
    const changed = { ...session, account: { ...session.account, email: 'neu@example.test' } };
    const { calls } = mockApi(routes({ 'PUT /auth/account': () => json(changed) }));
    const { user, form } = await openOwn();
    const email = within(form).getByRole('textbox', { name: 'E-Mail-Adresse' });
    expect(email).toHaveValue('owner@example.test');
    await fill(user, email, 'neu@example.test');
    await fill(user, within(form).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));

    expect(await screen.findByText('Dein Konto ist gespeichert.')).toBeVisible();
    const put = calls.find((call) => call.method === 'PUT' && call.path === '/auth/account');
    expect(put?.body).toEqual({ email: 'neu@example.test', currentPassword: 'geheim-0123456' });
    // The draft is spent: the current password is empty again, the focus stays on the button.
    await waitFor(() => expect(within(form).getByLabelText(/^Aktuelles Passwort/)).toHaveValue(''));
    expect(within(form).getByRole('button', { name: 'Speichern' })).toHaveFocus();
  });

  it('keeps what is typed into the own account while its save waits for the server', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    const changed = { ...session, account: { ...session.account, email: 'neu@example.test' } };
    const { calls } = mockApi(
      routes({
        'PUT /auth/account': async () => {
          await answered;
          return json(changed);
        },
      }),
    );
    const { user, form } = await openOwn();
    const email = within(form).getByRole('textbox', { name: 'E-Mail-Adresse' });
    await fill(user, email, 'neu@example.test');
    await fill(user, within(form).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'PUT')).toBe(true));
    // Typed while the save runs: it stays a draft once the answer is there.
    await user.type(email, 'x');
    release();
    expect(await screen.findByText('Dein Konto ist gespeichert.')).toBeVisible();
    expect(email).toHaveValue('neu@example.testx');
    expect(within(form).getByRole('button', { name: 'Verwerfen' })).toBeVisible();
    // The kept draft holds no password: the sent one is used up.
    expect(within(form).getByLabelText(/^Aktuelles Passwort/)).toHaveValue('');
  });

  it('sends a new password only with a matching repeat of at least 10 characters', async () => {
    const { calls } = mockApi(routes({ 'PUT /auth/account': () => json(session) }));
    const { user, form } = await openOwn();
    await fill(user, within(form).getByLabelText('Neues Passwort'), 'kurz');
    await fill(user, within(form).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    expect(await within(form).findByText('Mindestens 10 Zeichen.')).toBeVisible();

    const fresh = within(form).getByLabelText('Neues Passwort');
    await fill(user, fresh, 'neues-passwort-42');
    await fill(
      user,
      within(form).getByLabelText('Neues Passwort wiederholen'),
      'neues-passwort-43',
    );
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    expect(await within(form).findByText('Die Passwörter stimmen nicht überein.')).toBeVisible();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);

    const repeat = within(form).getByLabelText('Neues Passwort wiederholen');
    await fill(user, repeat, 'neues-passwort-42');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Dein Konto ist gespeichert.')).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      currentPassword: 'geheim-0123456',
      newPassword: 'neues-passwort-42',
    });
  });

  it('says so when nothing changed, without asking the server', async () => {
    const { calls } = mockApi(routes());
    const { user, form } = await openOwn();
    await fill(user, within(form).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(form).findByText('Ändere die E-Mail-Adresse oder gib ein neues Passwort ein.'),
    ).toBeVisible();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('shows a wrong current password and a taken e-mail at their fields', async () => {
    let answer = problem(401, 'invalid_credentials');
    mockApi(routes({ 'PUT /auth/account': () => answer }));
    const { user, form } = await openOwn();
    const email = within(form).getByRole('textbox', { name: 'E-Mail-Adresse' });
    await fill(user, email, 'gast@example.test');
    await fill(user, within(form).getByLabelText(/^Aktuelles Passwort/), 'falsch-0123456');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    expect(await within(form).findByText('Das aktuelle Passwort stimmt nicht.')).toBeVisible();
    expect(within(form).getByLabelText(/^Aktuelles Passwort/)).toHaveAttribute(
      'aria-invalid',
      'true',
    );

    // The field stays marked until it changes; the next try goes with the right password.
    answer = problem(409, 'email_taken');
    const current = within(form).getByLabelText(/^Aktuelles Passwort/);
    await fill(user, current, 'geheim-0123456');
    // The verdict was about the old value: editing removes it, so Enter submits again.
    expect(within(form).queryByText('Das aktuelle Passwort stimmt nicht.')).toBeNull();
    expect(current).not.toHaveAttribute('aria-invalid', 'true');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(form).findByText('Diese E-Mail-Adresse gehört schon zu einem anderen Konto.'),
    ).toBeVisible();
    expect(email).toHaveAttribute('aria-invalid', 'true');
    // «Verwerfen» takes the stored address back.
    await user.click(within(form).getByRole('button', { name: 'Verwerfen' }));
    expect(within(form).getByRole('textbox', { name: 'E-Mail-Adresse' })).toHaveValue(
      'owner@example.test',
    );
  });

  it('opens the own account at «Meine Profile» from `?item=me`; an unknown item leaves the address', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/accounts?item=me#meine-profile');
    await screen.findByRole('table', { name: 'Konten' });
    await waitFor(() =>
      expect(toggleOf('owner@example.test')).toHaveAttribute('aria-expanded', 'true'),
    );
    expect(toggleOf('gast@example.test')).toHaveAttribute('aria-expanded', 'false');
    const mine = within(foldOut('owner@example.test')).getByRole('region', {
      name: 'Meine Profile',
    });
    expect(mine).toHaveAttribute('id', 'meine-profile');
    expect(await within(mine).findByRole('article', { name: 'Lena' })).toBeVisible();
    // The sidebar marks «Profile», not «Konten».
    const nav = screen.getByRole('navigation', { name: 'Verwaltung' });
    expect(within(nav).getByRole('link', { name: /^Profile/ })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(nav).getByRole('link', { name: /^Konten/ })).not.toHaveAttribute('aria-current');

    await act(() => router.navigate({ href: '/admin/accounts?item=999&tab=bogus' }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(within(nav).getByRole('link', { name: /^Konten/ })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(screen.getByRole('tab', { name: /^Konten/ })).toHaveAttribute('aria-selected', 'true');
  });

  it('manages the own profiles in «Konten»: create in the dialog, the table follows', async () => {
    let listed = profiles;
    let accounts = [owner, guest];
    const { calls } = mockApi(
      routes({
        'GET /profiles': () => json(listed),
        'GET /admin/accounts': () => json(accounts),
        'POST /profiles': () => {
          const created = { ...profiles[1]!, id: 7, name: 'Kind', isPrimary: false };
          listed = [...profiles, created];
          accounts = [
            {
              ...owner,
              profiles: [
                ...owner.profiles,
                { id: 7, name: 'Kind', maxRating: 12, isPrimary: false },
              ],
            },
            guest,
          ];
          return json(created, 201);
        },
      }),
    );
    renderApp('/admin/accounts?item=me');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    const mine = await waitFor(() =>
      within(foldOut('owner@example.test')).getByRole('region', { name: 'Meine Profile' }),
    );
    await within(mine).findByRole('article', { name: 'Lena' });
    await user.click(within(mine).getByRole('button', { name: 'Profil hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Neues Profil' });
    await fill(user, within(dialog).getByLabelText('Name'), 'Kind');
    await user.click(within(dialog).getByRole('button', { name: 'Profil anlegen' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toMatchObject({ name: 'Kind' });
    expect(await within(mine).findByRole('article', { name: 'Kind' })).toBeVisible();
    await waitFor(() =>
      expect(within(rowOf('owner@example.test')).getAllByRole('cell')[3]).toHaveTextContent(
        'Marco (Hauptprofil), Kinder, Kind',
      ),
    );
  });

  it('clears the passwords when the own row folds away; the typed e-mail stays', async () => {
    mockApi(routes());
    const { user, form } = await openOwn();
    const email = within(form).getByRole('textbox', { name: 'E-Mail-Adresse' });
    await fill(user, email, 'neu@example.test');
    await fill(user, within(form).getByLabelText('Neues Passwort'), 'neues-passwort-42');
    await fill(
      user,
      within(form).getByLabelText('Neues Passwort wiederholen'),
      'neues-passwort-42',
    );
    await fill(user, within(form).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');

    await user.click(toggleOf('owner@example.test'));
    const again = within(await openRow(user, 'owner@example.test')).getByRole('region', {
      name: 'Konto ändern',
    });
    expect(within(again).getByRole('textbox', { name: 'E-Mail-Adresse' })).toHaveValue(
      'neu@example.test',
    );
    expect(within(again).getByLabelText('Neues Passwort')).toHaveValue('');
    expect(within(again).queryByLabelText('Neues Passwort wiederholen')).toBeNull();
    expect(within(again).getByLabelText(/^Aktuelles Passwort/)).toHaveValue('');
    // The e-mail is still a draft.
    expect(within(again).getByRole('button', { name: 'Verwerfen' })).toBeVisible();
  });

  it('empties the password fields after a save and keeps no password in the mutation cache', async () => {
    const { calls } = mockApi(routes({ 'PUT /auth/account': () => json(session) }));
    const { queryClient } = renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    const form = within(await openRow(user, 'owner@example.test')).getByRole('region', {
      name: 'Konto ändern',
    });
    await fill(user, within(form).getByLabelText('Neues Passwort'), 'neues-passwort-42');
    await fill(
      user,
      within(form).getByLabelText('Neues Passwort wiederholen'),
      'neues-passwort-42',
    );
    await fill(user, within(form).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(form).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Dein Konto ist gespeichert.')).toBeVisible();
    expect(calls.some((call) => call.method === 'PUT')).toBe(true);
    await waitFor(() => expect(within(form).getByLabelText('Neues Passwort')).toHaveValue(''));
    expect(within(form).queryByLabelText('Neues Passwort wiederholen')).toBeNull();
    expect(within(form).getByLabelText(/^Aktuelles Passwort/)).toHaveValue('');
    const cached = JSON.stringify(
      queryClient
        .getMutationCache()
        .getAll()
        .map((mutation) => mutation.state.variables ?? null),
    );
    expect(cached).not.toContain('geheim-0123456');
    expect(cached).not.toContain('neues-passwort-42');
  });

  it('keeps a draft when the sidebar switches from «Konten» to «Profile»: same page, no question', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/accounts');
    await screen.findByRole('table', { name: 'Konten' });
    const user = userEvent.setup();
    const access = () =>
      within(foldOut('gast@example.test')).getByRole('region', {
        name: 'Zugriff von gast@example.test',
      });
    await openRow(user, 'gast@example.test');
    await user.click(within(access()).getByRole('radio', { name: 'FSK 16' }));

    await user.click(
      within(screen.getByRole('navigation', { name: 'Verwaltung' })).getByRole('link', {
        name: /^Profile/,
      }),
    );
    await waitFor(() => expect(router.state.location.search).toEqual({ item: 'me' }));
    expect(screen.queryByRole('alertdialog')).toBeNull();
    await waitFor(() =>
      expect(toggleOf('owner@example.test')).toHaveAttribute('aria-expanded', 'true'),
    );
    expect(within(access()).getByRole('radio', { name: 'FSK 16' })).toBeChecked();
  });

  it('shows the tabs when only the account list failed, with its error and retry in «Konten»', async () => {
    let fail = true;
    mockApi(
      routes({
        'GET /admin/accounts': () => (fail ? problem(500, 'internal') : json([owner, guest])),
        'GET /admin/invitations': () => json([invitation()]),
      }),
    );
    renderApp('/admin/accounts');
    const user = userEvent.setup();
    const panel = await screen.findByRole('tabpanel', { name: /^Konten/ });
    const retry = await within(panel).findByRole('button', { name: 'Erneut versuchen' });
    expect(screen.getByRole('tab', { name: /^Einladungen/ })).toHaveTextContent('1 offen');
    fail = false;
    await user.click(retry);
    expect(await screen.findByRole('table', { name: 'Konten' })).toBeVisible();
  });

  it('is for administrators only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/accounts');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});

// K-6 with ADR 0014: an account comes into being through an invitation and nothing else, so the
// one button that creates one sits in the page header, and the links it produced in their tab.
describe('invitations (K-6, KON-1, KON-5)', { timeout: 15_000 }, () => {
  afterEach(() => vi.unstubAllGlobals());

  const invitationTable = () => screen.getByRole('table', { name: 'Bestehende Einladungen' });

  it('creates an invitation in the dialog and shows the full link once, with a copy button', async () => {
    const created = {
      ...invitation({
        id: 2,
        note: 'Tante Rosa',
        maxRating: 16,
        allLibraries: false,
        libraryIds: [2],
      }),
      url: '/invite/geheimes-token',
      token: 'geheimes-token',
    };
    let listed: Invitation[] = [];
    const { calls } = mockApi({
      ...routes(),
      'GET /admin/invitations': () => json(listed),
      'POST /admin/invitations': () => {
        listed = [created];
        return json(created, 201);
      },
    });
    const { queryClient } = renderApp('/admin/accounts');
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');

    const invite = await screen.findByRole('button', { name: 'Konto einladen' });
    await waitFor(() => expect(invite).toBeEnabled());
    await user.click(invite);
    const dialog = await screen.findByRole('dialog', { name: 'Konto einladen' });
    await fill(user, within(dialog).getByLabelText(/Für wen ist der Link/), 'Tante Rosa');
    await user.click(within(dialog).getByRole('radio', { name: 'FSK 16' }));
    await user.click(within(dialog).getByRole('radio', { name: 'Auswahl' }));
    await user.click(within(dialog).getByRole('checkbox', { name: 'Serien' }));
    await user.click(within(dialog).getByRole('button', { name: 'Einladung erstellen' }));

    const link = await within(dialog).findByLabelText('Einladungslink');
    expect(link).toHaveValue(`${window.location.origin}/invite/geheimes-token`);
    expect(link).toHaveAttribute('readonly');
    expect(within(dialog).getByText(/nur dieses eine Mal angezeigt/)).toBeVisible();
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      note: 'Tante Rosa',
      maxRating: 16,
      allLibraries: false,
      libraryIds: [2],
    });
    // The one-time link lives in the dialog only, never in the mutation cache.
    await waitFor(() =>
      expect(
        JSON.stringify(
          queryClient
            .getMutationCache()
            .getAll()
            .map((mutation) => mutation.state),
        ),
      ).not.toContain('/invite/'),
    );

    await user.click(within(dialog).getByRole('button', { name: 'Link kopieren' }));
    expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/invite/geheimes-token`);
    expect(await within(dialog).findByText('Link kopiert.')).toBeVisible();

    // "Fertig" closes the dialog onto its button; the new row is in the tab «Einladungen», the
    // link itself is not part of it.
    await user.click(within(dialog).getByRole('button', { name: 'Fertig' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Konto einladen' })).toHaveFocus(),
    );
    expect(screen.getByRole('tab', { name: /^Einladungen/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(await within(invitationTable()).findByText('Tante Rosa')).toBeVisible();
    expect(within(invitationTable()).queryByText(/geheimes-token/)).not.toBeInTheDocument();
  });

  it('selects the link for Ctrl+C when the clipboard is not available', async () => {
    mockApi({
      ...routes(),
      'POST /admin/invitations': () =>
        json({ ...invitation(), url: '/invite/abc', token: 'abc' }, 201),
    });
    renderApp('/admin/accounts');
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('not allowed'));

    const invite = await screen.findByRole('button', { name: 'Konto einladen' });
    await waitFor(() => expect(invite).toBeEnabled());
    await user.click(invite);
    await user.click(await screen.findByRole('button', { name: 'Einladung erstellen' }));
    await user.click(await screen.findByRole('button', { name: 'Link kopieren' }));
    expect(await screen.findByText(/Kopieren nicht möglich/)).toBeVisible();
    expect(screen.getByLabelText('Einladungslink')).toHaveFocus();
  });

  it('lists invitations with their state and revokes an open one after confirmation', async () => {
    let listed = [
      invitation(),
      invitation({ id: 2, note: 'Max', state: 'used', usedByEmail: 'max@example.test' }),
      invitation({ id: 3, note: undefined, state: 'expired' }),
      invitation({ id: 4, note: 'Alt', state: 'revoked' }),
    ];
    const { calls } = mockApi({
      ...routes({ 'GET /admin/invitations': () => json(listed) }),
      'DELETE /admin/invitations/1': () => {
        listed = listed.map((item) => (item.id === 1 ? { ...item, state: 'revoked' } : item));
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/accounts?tab=invitations');
    const user = userEvent.setup();

    const list = await screen.findByRole('table', { name: 'Bestehende Einladungen' });
    expect(
      within(list)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual([
      'Notiz',
      'Zustand',
      'Altersstufe',
      'Bibliotheken',
      'Gültig bis / angenommen',
      'Aktionen',
    ]);
    expect(within(list).getByText('Offen')).toBeVisible();
    expect(within(list).getByText('Benutzt')).toBeVisible();
    expect(within(list).getByText('Abgelaufen')).toBeVisible();
    expect(within(list).getByText('Widerrufen', { selector: 'span' })).toBeVisible();
    expect(within(list).getByText('Angenommen von max@example.test')).toBeVisible();
    expect(within(list).getByText('Ohne Notiz')).toBeVisible();
    // Only the open invitation can be revoked.
    expect(within(list).getAllByRole('button', { name: 'Widerrufen' })).toHaveLength(1);

    await user.click(within(list).getByRole('button', { name: 'Widerrufen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Einladung widerrufen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Widerrufen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
    expect(await screen.findByText('Einladung «Oma» widerrufen.')).toBeVisible();
    await waitFor(() =>
      expect(within(invitationTable()).queryByRole('button', { name: 'Widerrufen' })).toBeNull(),
    );
    // Its button went: the h1 takes the focus.
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Konten' })).toHaveFocus(),
    );
  });

  it('offers no invitation while the library list is missing, so no right is widened', async () => {
    const { calls } = mockApi({
      ...routes({ 'GET /libraries': () => problem(500, 'internal') }),
      'POST /admin/invitations': () => json({ ...invitation(), url: '/invite/x', token: 'x' }, 201),
    });
    renderApp('/admin/accounts');

    expect(await screen.findByText('Die Bibliotheken konnten nicht geladen werden.')).toBeVisible();
    expect(await screen.findByRole('button', { name: 'Konto einladen' })).toBeDisabled();
    // Nor an access form: a missing list never reads as «no restriction».
    await screen.findByRole('table', { name: 'Konten' });
    const other = await openRow(userEvent.setup(), 'gast@example.test');
    expect(within(other).queryByRole('region', { name: /^Zugriff von/ })).toBeNull();
    expect(calls.some((call) => call.method === 'POST' || call.method === 'PATCH')).toBe(false);
  });
});
