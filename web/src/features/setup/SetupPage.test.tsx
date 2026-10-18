import { screen, waitFor } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { homePage } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, type Routes } from '@/test/api';

const setupRequired: Routes = {
  'GET /setup': () => json({ setupRequired: true }),
  'GET /healthz': () => json({ status: 'ok', version: '0.2.0', uptimeSeconds: 1 }),
};

async function fillWizard(user: UserEvent, { tmdb = '' } = {}) {
  await user.type(await screen.findByLabelText(/E-Mail-Adresse/), 'owner@example.test');
  await user.type(screen.getByLabelText('Passwort'), 'correct horse battery');
  await user.type(screen.getByLabelText('Passwort wiederholen'), 'correct horse battery');
  await user.click(screen.getByRole('radio', { name: 'Filme' }));
  await user.type(screen.getByLabelText('Ordner'), '/media/Filme');
  if (tmdb) await user.type(screen.getByLabelText('API-Lesezugriffs-Token'), tmdb);
}

describe('setup wizard', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('sends everything to the guard first: setup required redirects to /setup', async () => {
    mockApi(setupRequired);
    const { router } = renderApp('/');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Willkommen bei Slouchcat' }),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe('/setup');
    // The three steps of A-1.
    expect(screen.getByRole('heading', { name: /Hauptkonto/ })).toBeVisible();
    expect(screen.getByRole('heading', { name: /Erste Bibliothek/ })).toBeVisible();
    expect(screen.getByRole('heading', { name: /Metadaten von TMDB/ })).toBeVisible();
  });

  it('checks password length and repetition in the client', async () => {
    const { calls } = mockApi(setupRequired);
    renderApp('/setup');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('Passwort'), 'short');
    await user.type(screen.getByLabelText('Passwort wiederholen'), 'other');
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));
    expect(
      await screen.findByText('Mindestens 10 Zeichen.', { selector: '[slot="errorMessage"]' }),
    ).toBeVisible();
    expect(screen.getByText('Die Passwörter stimmen nicht überein.')).toBeVisible();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);
  });

  it('posts the request and opens the start page on 201', async () => {
    const { calls } = mockApi({
      ...setupRequired,
      'POST /setup': () => json(session, 201),
      'GET /auth/session': () => json(session),
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user, { tmdb: 'tmdb-token' });
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    const post = calls.find((call) => call.method === 'POST' && call.path === '/setup');
    expect(post?.body).toEqual({
      email: 'owner@example.test',
      password: 'correct horse battery',
      library: { name: 'Filme', kind: 'movies', path: '/media/Filme' },
      tmdbApiKey: 'tmdb-token',
      language: 'de',
    });
  });

  it('maps library_path_invalid to the path field', async () => {
    mockApi({ ...setupRequired, 'POST /setup': () => problem(422, 'library_path_invalid') });
    renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user);
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));

    const path = screen.getByLabelText('Ordner');
    await waitFor(() => expect(path).toHaveAttribute('aria-invalid', 'true'));
    expect(path).toHaveAccessibleDescription(/Diesen Ordner gibt es auf dem Server nicht/);
  });

  it.each([
    ['tmdb_key_invalid', 'TMDB hat den Schlüssel abgelehnt.'],
    ['tmdb_unreachable', 'TMDB ist gerade nicht erreichbar.'],
  ])('maps %s to the TMDB field', async (code, message) => {
    mockApi({ ...setupRequired, 'POST /setup': () => problem(422, code) });
    renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user, { tmdb: 'bad' });
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));
    const field = screen.getByLabelText('API-Lesezugriffs-Token');
    await waitFor(() => expect(field).toHaveAccessibleDescription(new RegExp(message)));
  });

  it('lists validation_failed field codes at their inputs', async () => {
    mockApi({
      ...setupRequired,
      'POST /setup': () =>
        problem(422, 'validation_failed', {
          errors: [
            { field: 'email', code: 'invalid_email' },
            { field: 'library.name', code: 'too_long' },
          ],
        }),
    });
    renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user);
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Bitte prüfe die markierten Felder.',
    );
    expect(screen.getByLabelText(/E-Mail-Adresse/)).toHaveAccessibleDescription(
      /Keine gültige E-Mail-Adresse/,
    );
    expect(screen.getByLabelText('Name')).toHaveAccessibleDescription(/Zu lang/);
  });
});
