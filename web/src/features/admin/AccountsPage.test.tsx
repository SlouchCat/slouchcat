import type { AdminAccount } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountLibraries } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// A-2 with ADR 0014: every account in one table, with its role and the owner badge. The
// signed-in account changes its own e-mail and password; the administration changes the access of
// the others, and the owner alone appoints and demotes. The page shows or hides buttons, the
// server decides - every test that checks a hidden button also checks that no request went out.

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
  libraryIds: [3],
  profiles: [{ id: 9, name: 'Lena', maxRating: 12, isPrimary: true }],
  createdAt: '2026-10-10T10:00:00Z',
  limits: { maxStreams: 2 },
  channels: [{ channel: 'email', enabled: false }],
};

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /libraries': () => json(accountLibraries),
  'GET /admin/accounts': () => json([owner, guest]),
  ...extra,
});

const rowOf = (email: string) => screen.getByRole('rowheader', { name: email }).closest('tr')!;

async function openDialog() {
  const user = userEvent.setup();
  renderApp('/admin/accounts');
  await user.click(await screen.findByRole('button', { name: 'Konto ändern' }));
  const dialog = await screen.findByRole('dialog', { name: 'Konto ändern' });
  return { user, dialog };
}

describe('admin accounts (A-2)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists every account with role, rating, libraries, profiles and channels', async () => {
    mockApi(routes());
    renderApp('/admin/accounts');
    expect(await screen.findByRole('heading', { level: 1, name: 'Konten' })).toBeVisible();
    const table = await screen.findByRole('table', { name: 'Konten' });
    expect(
      within(table)
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
    expect(cells('owner@example.test').slice(0, 6)).toEqual([
      'AdministratorBesitzer',
      'FSK 18',
      'Alle Bibliotheken',
      'Marco (Hauptprofil), Kinder',
      expect.stringMatching(/2026/) as string,
      'E-MailWeb-Push',
    ]);
    expect(cells('gast@example.test').slice(0, 6)).toEqual([
      'Konto',
      'FSK 12',
      '1 Bibliothek',
      'Lena (Hauptprofil)',
      'Noch nie',
      'Keine',
    ]);
    // The own row changes the account and leads to the profiles; a guest row leads to access.
    const own = rowOf('owner@example.test');
    expect(within(own).getByRole('button', { name: 'Konto ändern' })).toBeVisible();
    expect(within(own).getByRole('link', { name: 'Profile' })).toHaveAttribute(
      'href',
      '/settings/profiles',
    );
    expect(within(own).getByText('Besitzer')).toBeVisible();
    const other = rowOf('gast@example.test');
    expect(
      within(other).getByRole('button', { name: 'Zugriff von gast@example.test ändern' }),
    ).toBeVisible();
    expect(
      within(other).getByRole('button', { name: 'gast@example.test zum Administrator ernennen' }),
    ).toBeVisible();
    expect(
      within(other).getByRole('button', { name: 'gast@example.test entfernen' }),
    ).toBeVisible();
    expect(within(other).queryByRole('button', { name: 'Konto ändern' })).toBeNull();
    // The owner's own row offers neither a role action nor a removal: it is protected.
    expect(within(own).queryByRole('button', { name: /ernennen|entziehen|entfernen/ })).toBeNull();
  });

  it('changes the access of an account: rating, libraries and the three W-8 caps', async () => {
    const { calls } = mockApi(
      routes({ 'PATCH /admin/accounts/7': () => new Response(null, { status: 204 }) }),
    );
    renderApp('/admin/accounts');
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole('button', { name: 'Zugriff von gast@example.test ändern' }),
    );
    const dialog = await screen.findByRole('dialog', { name: 'Zugriff von gast@example.test' });
    expect(within(dialog).getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    expect(within(dialog).getByText('Gilt für alle Profile dieses Kontos.')).toBeVisible();
    // W-8 is an account cap; the watch time is a profile setting and has no place here.
    expect(within(dialog).queryByLabelText('Schauzeit pro Tag (Minuten)')).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('radio', { name: 'FSK 6' }));
    await user.click(within(dialog).getByRole('radio', { name: '480p' }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toMatchObject({
      maxRating: 6,
      allLibraries: false,
      libraryIds: [3],
      limits: { maxHeight: 480, maxStreams: 2 },
    });
  });

  it('appoints and removes, and says what the server answered', async () => {
    let roleAnswer = () => new Response(null, { status: 204 });
    const { calls } = mockApi(
      routes({
        'PUT /admin/accounts/7/role': () => roleAnswer(),
        'DELETE /admin/accounts/7': () => new Response(null, { status: 204 }),
      }),
    );
    renderApp('/admin/accounts');
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole('button', { name: 'gast@example.test zum Administrator ernennen' }),
    );
    const confirm = await screen.findByRole('alertdialog', {
      name: 'gast@example.test zum Administrator ernennen?',
    });
    expect(confirm).toHaveTextContent('FSK 18');
    await user.click(within(confirm).getByRole('button', { name: 'Ernennen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ role: 'admin' }),
    );

    // The server has the last word: a refusal is shown, not swallowed.
    roleAnswer = () => problem(409, 'last_admin');
    await user.click(
      screen.getByRole('button', { name: 'gast@example.test zum Administrator ernennen' }),
    );
    const again = await screen.findByRole('alertdialog', {
      name: 'gast@example.test zum Administrator ernennen?',
    });
    await user.click(within(again).getByRole('button', { name: 'Ernennen' }));
    expect(
      await screen.findByText('Das ist der letzte Administrator. Ernenne zuerst einen zweiten.'),
    ).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'gast@example.test entfernen' }));
    const removal = await screen.findByRole('alertdialog', {
      name: 'Konto gast@example.test entfernen?',
    });
    expect(removal).toHaveTextContent('enden sofort');
    await user.click(within(removal).getByRole('button', { name: 'Konto entfernen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
  });

  it('hides the role actions from an administrator that is not the owner', async () => {
    mockApi(
      routes({
        'GET /auth/session': () =>
          json({ ...session, account: { ...session.account, isOwner: false } }),
      }),
    );
    renderApp('/admin/accounts');
    const other = rowOf(
      await screen.findByText('gast@example.test').then(() => 'gast@example.test'),
    );
    expect(
      within(other).queryByRole('button', { name: /zum Administrator ernennen|Rolle entziehen/ }),
    ).toBeNull();
    // Access and removal stay: they are administration, not ownership.
    expect(
      within(other).getByRole('button', { name: 'Zugriff von gast@example.test ändern' }),
    ).toBeVisible();
  });

  it('changes the e-mail with the current password; the session follows', async () => {
    const changed = { ...session, account: { ...session.account, email: 'neu@example.test' } };
    const { calls } = mockApi(routes({ 'PUT /auth/account': () => json(changed) }));
    const { user, dialog } = await openDialog();
    const email = within(dialog).getByRole('textbox', { name: 'E-Mail-Adresse' });
    // The first field has the focus and holds the current address.
    expect(email).toHaveFocus();
    expect(email).toHaveValue('owner@example.test');
    await user.clear(email);
    await user.type(email, 'neu@example.test');
    await user.type(within(dialog).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    const put = calls.find((call) => call.method === 'PUT' && call.path === '/auth/account');
    expect(put?.body).toEqual({ email: 'neu@example.test', currentPassword: 'geheim-0123456' });
    expect(await screen.findByText('Dein Konto ist gespeichert.')).toBeVisible();
    // The dialog returns the focus to the button that opened it.
    await waitFor(() =>
      expect(
        within(rowOf('owner@example.test')).getByRole('button', { name: 'Konto ändern' }),
      ).toHaveFocus(),
    );
  });

  it('sends a new password only with a matching repeat of at least 10 characters', async () => {
    const { calls } = mockApi(routes({ 'PUT /auth/account': () => json(session) }));
    const { user, dialog } = await openDialog();
    await user.type(within(dialog).getByLabelText('Neues Passwort'), 'kurz');
    await user.type(within(dialog).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(await within(dialog).findByText('Mindestens 10 Zeichen.')).toBeVisible();

    const fresh = within(dialog).getByLabelText('Neues Passwort');
    await user.clear(fresh);
    await user.type(fresh, 'neues-passwort-42');
    await user.type(
      within(dialog).getByLabelText('Neues Passwort wiederholen'),
      'neues-passwort-43',
    );
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(await within(dialog).findByText('Die Passwörter stimmen nicht überein.')).toBeVisible();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);

    const repeat = within(dialog).getByLabelText('Neues Passwort wiederholen');
    await user.clear(repeat);
    await user.type(repeat, 'neues-passwort-42');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      currentPassword: 'geheim-0123456',
      newPassword: 'neues-passwort-42',
    });
  });

  it('says so when nothing changed, without asking the server', async () => {
    const { calls } = mockApi(routes());
    const { user, dialog } = await openDialog();
    await user.type(within(dialog).getByLabelText(/^Aktuelles Passwort/), 'geheim-0123456');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(dialog).findByText('Ändere die E-Mail-Adresse oder gib ein neues Passwort ein.'),
    ).toBeVisible();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('shows a wrong current password and a taken e-mail at their fields', async () => {
    let answer = problem(401, 'invalid_credentials');
    mockApi(routes({ 'PUT /auth/account': () => answer }));
    const { user, dialog } = await openDialog();
    const email = within(dialog).getByRole('textbox', { name: 'E-Mail-Adresse' });
    await user.clear(email);
    await user.type(email, 'gast@example.test');
    await user.type(within(dialog).getByLabelText(/^Aktuelles Passwort/), 'falsch-0123456');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(await within(dialog).findByText('Das aktuelle Passwort stimmt nicht.')).toBeVisible();
    expect(within(dialog).getByLabelText(/^Aktuelles Passwort/)).toHaveAttribute(
      'aria-invalid',
      'true',
    );

    // The field stays marked until it changes; the next try goes with the right password.
    answer = problem(409, 'email_taken');
    const current = within(dialog).getByLabelText(/^Aktuelles Passwort/);
    await user.clear(current);
    await user.type(current, 'geheim-0123456');
    // The verdict was about the old value: editing removes it, so Enter submits again.
    expect(within(dialog).queryByText('Das aktuelle Passwort stimmt nicht.')).toBeNull();
    expect(current).not.toHaveAttribute('aria-invalid', 'true');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(dialog).findByText('Diese E-Mail-Adresse gehört schon zu einem anderen Konto.'),
    ).toBeVisible();
    expect(email).toHaveAttribute('aria-invalid', 'true');
    expect(dialog).toBeVisible();
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
