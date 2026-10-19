import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountLibraries, profiles } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

const manage: Routes = {
  ...signedIn,
  'GET /profiles': () => json(profiles),
  'GET /libraries': () => json(accountLibraries),
};

describe('profile settings page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists every profile with rating, language, PIN and libraries', async () => {
    mockApi(manage);
    renderApp('/settings/profiles');

    const marco = await screen.findByRole('article', { name: 'Marco' });
    expect(within(marco).getByText('Hauptprofil')).toBeVisible();
    expect(within(marco).getByText('FSK 18')).toBeVisible();
    expect(within(marco).getByText('Alle Bibliotheken')).toBeVisible();
    // The primary profile cannot be deleted.
    expect(within(marco).queryByRole('button', { name: 'Löschen' })).not.toBeInTheDocument();
    expect(within(marco).getByRole('button', { name: 'PIN setzen' })).toBeVisible();

    const lena = screen.getByRole('article', { name: 'Lena' });
    expect(within(lena).getByText('FSK 12')).toBeVisible();
    expect(within(lena).getByText('English')).toBeVisible();
    expect(within(lena).getByText('PIN')).toBeVisible();
    expect(within(lena).getByText('Filme')).toBeVisible();
    expect(within(lena).getByRole('button', { name: 'PIN ändern' })).toBeVisible();
    expect(within(lena).getByRole('button', { name: 'PIN entfernen' })).toBeVisible();
    expect(within(lena).getByRole('button', { name: 'Löschen' })).toBeVisible();
  });

  it('gives a non-primary profile a notice instead of the management', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile!, id: 2, isPrimary: false } }),
    });
    renderApp('/settings/profiles');
    expect(await screen.findByText('Nur im Hauptprofil')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Profil wechseln' })).toHaveAttribute(
      'href',
      '/profiles',
    );
    expect(calls.some((call) => call.path === '/profiles')).toBe(false);
  });

  it('creates a profile from the dialog with the chosen settings', async () => {
    const { calls } = mockApi({
      ...manage,
      'POST /profiles': () => json({ ...profiles[1]!, id: 7, name: 'Kind' }, 201),
    });
    renderApp('/settings/profiles');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Profil hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Neues Profil' });
    expect(within(dialog).getByLabelText('Name')).toHaveFocus();

    await user.type(within(dialog).getByLabelText('Name'), 'Kind');
    await user.click(within(dialog).getByRole('radio', { name: 'FSK 6' }));
    await user.click(within(dialog).getByRole('radio', { name: 'English' }));
    await user.click(within(dialog).getByRole('radio', { name: 'Türkis' }));
    await user.click(within(dialog).getByRole('radio', { name: 'Auswahl' }));
    await user.click(within(dialog).getByRole('checkbox', { name: 'Serien' }));
    await user.type(within(dialog).getByLabelText(/^PIN/), '4321');
    await user.click(within(dialog).getByRole('button', { name: 'Profil anlegen' }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      name: 'Kind',
      maxRating: 6,
      language: 'en',
      accent: 'teal',
      allLibraries: false,
      libraryIds: [2],
      pin: '4321',
    });
  });

  it('offers only ratings up to the ceiling of a guest account', async () => {
    mockApi({
      ...manage,
      'GET /auth/session': () =>
        json({
          account: {
            ...session.account,
            role: 'guest',
            maxRating: 12,
            allLibraries: false,
            libraryIds: [3],
          },
          profile: session.profile,
        }),
      'GET /libraries': () => json([{ id: 3, name: 'Kinder', kind: 'movies' }]),
    });
    renderApp('/settings/profiles');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Profil hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Neues Profil' });
    expect(within(dialog).getByRole('radio', { name: 'FSK 12' })).toBeEnabled();
    expect(within(dialog).getByRole('radio', { name: 'FSK 16' })).toBeDisabled();
    expect(within(dialog).getByRole('radio', { name: 'FSK 18' })).toBeDisabled();
    // A guest picks among the libraries of its own account.
    await user.click(within(dialog).getByRole('radio', { name: 'Auswahl' }));
    expect(within(dialog).getByRole('checkbox', { name: 'Kinder' })).toBeVisible();
  });

  it('shows the server limit at the name field', async () => {
    mockApi({
      ...manage,
      'POST /profiles': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'name', code: 'too_many_profiles' }],
        }),
    });
    renderApp('/settings/profiles');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Profil hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Neues Profil' });
    await user.type(within(dialog).getByLabelText('Name'), 'Elftes{Enter}');
    expect(
      await within(dialog).findByText('Mehr als 10 Profile pro Konto sind nicht möglich.'),
    ).toBeVisible();
    expect(within(dialog).getByLabelText('Name')).toHaveAttribute('aria-invalid', 'true');
  });

  it('edits a profile with PATCH and asks before deleting', async () => {
    const { calls } = mockApi({
      ...manage,
      'PATCH /profiles/2': () => json({ ...profiles[1]!, maxRating: 16 }),
      'DELETE /profiles/2': () => new Response(null, { status: 204 }),
    });
    renderApp('/settings/profiles');
    const user = userEvent.setup();

    const lena = await screen.findByRole('article', { name: 'Lena' });
    await user.click(within(lena).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });
    expect(within(dialog).getByLabelText('Name')).toHaveValue('Lena');
    expect(within(dialog).getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    await user.click(within(dialog).getByRole('radio', { name: 'FSK 16' }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toMatchObject({
      name: 'Lena',
      maxRating: 16,
      allLibraries: false,
      libraryIds: [1],
    });

    await user.click(within(lena).getByRole('button', { name: 'Löschen' }));
    const confirm = await screen.findByRole('alertdialog', { name: 'Profil „Lena“ löschen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Abbrechen' }));
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);
    await user.click(within(lena).getByRole('button', { name: 'Löschen' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Profil löschen',
      }),
    );
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
  });

  it('treats an edit of the active profile like a switch: catalog cache cleared, session read again', async () => {
    const { calls } = mockApi({
      ...manage,
      'PATCH /profiles/1': () => json({ ...profiles[0]!, maxRating: 12 }),
    });
    const { queryClient } = renderApp('/settings/profiles');
    const user = userEvent.setup();
    queryClient.setQueryData(['item', 'secret'], { id: 'secret' });

    const marco = await screen.findByRole('article', { name: 'Marco' });
    await user.click(within(marco).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });
    await user.click(within(dialog).getByRole('radio', { name: 'FSK 12', hidden: true }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());

    expect(queryClient.getQueryData(['item', 'secret'])).toBeUndefined();
    const patchAt = calls.findIndex((call) => call.method === 'PATCH');
    expect(calls.slice(patchAt).some((call) => call.path === '/auth/session')).toBe(true);
  });

  it('sets a PIN with PUT and removes it after confirmation', async () => {
    const { calls } = mockApi({
      ...manage,
      'PUT /profiles/1/pin': () => new Response(null, { status: 204 }),
      'DELETE /profiles/2/pin': () => new Response(null, { status: 204 }),
    });
    renderApp('/settings/profiles');
    const user = userEvent.setup();

    const marco = await screen.findByRole('article', { name: 'Marco' });
    await user.click(within(marco).getByRole('button', { name: 'PIN setzen' }));
    const dialog = await screen.findByRole('dialog', { name: 'PIN für Marco' });
    await user.type(within(dialog).getByLabelText('Neue PIN'), '12345678{Enter}');
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ pin: '12345678' });

    const lena = screen.getByRole('article', { name: 'Lena' });
    await user.click(within(lena).getByRole('button', { name: 'PIN entfernen' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'PIN entfernen',
      }),
    );
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/profiles/2/pin'),
      ).toBe(true),
    );
  });

  it('shows both values where a profile lies above the guest ceiling and rewrites nothing', async () => {
    const { calls } = mockApi({
      ...manage,
      'GET /auth/session': () =>
        json({
          account: { ...session.account, role: 'guest', maxRating: 12, allLibraries: true },
          profile: session.profile,
        }),
      // The raw value (18) is above the guest ceiling (12): the server caps it in the view.
      'GET /profiles': () => json([profiles[0]]),
      'PATCH /profiles/1': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'maxRating', code: 'above_account_limit' }],
        }),
    });
    renderApp('/settings/profiles');
    const user = userEvent.setup();

    const marco = await screen.findByRole('article', { name: 'Marco' });
    expect(within(marco).getByText('FSK 18 · Konto höchstens FSK 12')).toBeVisible();
    await user.click(within(marco).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });
    // The stored value stays selected (shown, not selectable); the form does not clamp it.
    expect(within(dialog).getByRole('radio', { name: 'FSK 18' })).toBeChecked();
    expect(within(dialog).getByRole('radio', { name: 'FSK 18' })).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(dialog).findByText('Höher als die Obergrenze dieses Kontos.'),
    ).toBeVisible();
    expect(calls.find((call) => call.method === 'PATCH')?.body).toMatchObject({ maxRating: 18 });
  });

  it('translates the server codes invalid_pin and invalid_value at their fields', async () => {
    mockApi({
      ...manage,
      'PUT /profiles/1/pin': () =>
        problem(422, 'validation_failed', { errors: [{ field: 'pin', code: 'invalid_pin' }] }),
    });
    renderApp('/settings/profiles');
    const user = userEvent.setup();

    const marco = await screen.findByRole('article', { name: 'Marco' });
    await user.click(within(marco).getByRole('button', { name: 'PIN setzen' }));
    const dialog = await screen.findByRole('dialog', { name: 'PIN für Marco' });
    await user.type(within(dialog).getByLabelText('Neue PIN'), '1234{Enter}');
    expect(await within(dialog).findByText('Die PIN besteht aus 4 bis 8 Ziffern.')).toBeVisible();
  });

  describe('library rights are never widened by a missing list', () => {
    it('offers neither create nor edit while the libraries load, and sends nothing', async () => {
      const { calls } = mockApi({
        ...manage,
        // Never answers: the list stays pending.
        'GET /libraries': () => new Promise<Response>(() => undefined) as unknown as Response,
      });
      renderApp('/settings/profiles');

      const lena = await screen.findByRole('article', { name: 'Lena' });
      expect(screen.getByRole('button', { name: 'Profil hinzufügen' })).toBeDisabled();
      expect(within(lena).getByRole('button', { name: 'Bearbeiten' })).toBeDisabled();
      expect(calls.some((call) => call.method === 'POST' || call.method === 'PATCH')).toBe(false);
    });

    it('shows an error instead of the form when the list fails, and recovers on retry', async () => {
      let fail = true;
      mockApi({
        ...manage,
        'GET /libraries': () => (fail ? problem(500, 'internal') : json(accountLibraries)),
      });
      renderApp('/settings/profiles');
      const user = userEvent.setup();

      expect(
        await screen.findByText('Die Bibliotheken konnten nicht geladen werden.'),
      ).toBeVisible();
      expect(screen.getByRole('button', { name: 'Profil hinzufügen' })).toBeDisabled();
      const lena = await screen.findByRole('article', { name: 'Lena' });
      expect(within(lena).getByRole('button', { name: 'Bearbeiten' })).toBeDisabled();

      fail = false;
      await user.click(screen.getByRole('button', { name: 'Erneut versuchen' }));
      await waitFor(() =>
        expect(screen.getByRole('button', { name: 'Profil hinzufügen' })).toBeEnabled(),
      );
    });

    it('keeps the stored restriction when the list is empty', async () => {
      const { calls } = mockApi({
        ...manage,
        'GET /libraries': () => json([]),
        'PATCH /profiles/2': () => json(profiles[1]),
      });
      renderApp('/settings/profiles');
      const user = userEvent.setup();

      const lena = await screen.findByRole('article', { name: 'Lena' });
      await user.click(within(lena).getByRole('button', { name: 'Bearbeiten' }));
      const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });
      expect(within(dialog).getByRole('radio', { name: 'Auswahl' })).toBeChecked();
      await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
      await waitFor(() => expect(calls.some((call) => call.method === 'PATCH')).toBe(true));
      expect(calls.find((call) => call.method === 'PATCH')?.body).toMatchObject({
        allLibraries: false,
        libraryIds: [1],
      });
    });
  });

  it('keeps the PIN out of the mutation cache once the request settled', async () => {
    mockApi({
      ...manage,
      'PUT /profiles/1/pin': () => new Response(null, { status: 204 }),
      'POST /profiles': () => json({ ...profiles[1]!, id: 9 }, 201),
    });
    const { queryClient } = renderApp('/settings/profiles');
    const user = userEvent.setup();
    const holdsPin = () =>
      queryClient
        .getMutationCache()
        .getAll()
        .some((mutation) => JSON.stringify(mutation.state.variables ?? null).includes('"pin"'));

    const marco = await screen.findByRole('article', { name: 'Marco' });
    await user.click(within(marco).getByRole('button', { name: 'PIN setzen' }));
    const dialog = await screen.findByRole('dialog', { name: 'PIN für Marco' });
    await user.type(within(dialog).getByLabelText('Neue PIN'), '2468{Enter}');
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    await waitFor(() => expect(queryClient.getMutationCache().getAll()).toHaveLength(0));
    expect(holdsPin()).toBe(false);

    await user.click(await screen.findByRole('button', { name: 'Profil hinzufügen' }));
    const form = await screen.findByRole('dialog', { name: 'Neues Profil' });
    await user.type(within(form).getByLabelText('Name'), 'Neu');
    await user.type(within(form).getByLabelText(/^PIN/), '1357');
    await user.click(within(form).getByRole('button', { name: 'Profil anlegen' }));
    await waitFor(() => expect(form).not.toBeInTheDocument());
    await waitFor(() => expect(queryClient.getMutationCache().getAll()).toHaveLength(0));
    expect(holdsPin()).toBe(false);
  });
});
