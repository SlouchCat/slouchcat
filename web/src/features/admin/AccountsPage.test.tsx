import type { AdminAccount } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// A-2: the main account and the guests in one table; the main account changes its own e-mail
// and password. The server checks the current password and the e-mail; the page shows the answer.

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
  channels: [{ channel: 'email', enabled: false }],
};

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
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

  it('lists the main account and the guests with rating, libraries, profiles and channels', async () => {
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
      'Administrator',
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
    expect(
      within(rowOf('gast@example.test')).getByRole('link', {
        name: 'Zugriff von gast@example.test verwalten',
      }),
    ).toHaveAttribute('href', '/admin/access');
    expect(
      within(rowOf('gast@example.test')).queryByRole('button', { name: 'Konto ändern' }),
    ).toBeNull();
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

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/accounts');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
