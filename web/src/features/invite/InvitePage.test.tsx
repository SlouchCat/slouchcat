import type { InvitationPreview } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { homePage } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, type Routes } from '@/test/api';

const TOKEN = 'k7Qm2xV9aBcD4eFgH5iJ6lMnOpRsTuVw8xYz0AbCdEf';

const preview: InvitationPreview = {
  note: 'Oma',
  maxRating: 12,
  allLibraries: false,
  libraries: [
    { id: 1, name: 'Filme' },
    { id: 2, name: 'Serien' },
  ],
  expiresAt: '2026-10-26T12:00:00Z',
};

// A visitor without a session: the guard asks `/setup` only after a 401.
const visitor: Routes = {
  'GET /auth/session': () => problem(401, 'unauthenticated'),
  'GET /setup': () => json({ setupRequired: false }),
};

describe('invitation page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows note, rating ceiling and libraries before the form', async () => {
    mockApi({ ...visitor, [`GET /invitations/${TOKEN}`]: () => json(preview) });
    renderApp(`/invite/${TOKEN}`);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Einladung zu Slouchcat' }),
    ).toBeVisible();
    const summary = await screen.findByRole('region', { name: 'Für: Oma' });
    expect(within(summary).getByText('FSK 12')).toBeVisible();
    expect(within(summary).getByText('Filme')).toBeVisible();
    expect(within(summary).getByText('Serien')).toBeVisible();
    expect(screen.getByLabelText('E-Mail-Adresse')).toBeVisible();
    expect(screen.getByLabelText(/^Passwort/)).toBeVisible();
    expect(screen.getByLabelText(/^Profilname/)).toBeVisible();
    expect(screen.getByRole('radio', { name: 'Deutsch' })).toBeChecked();
  });

  it('explains a used, expired or revoked link (410)', async () => {
    mockApi({ ...visitor, [`GET /invitations/${TOKEN}`]: () => problem(410, 'invitation_gone') });
    renderApp(`/invite/${TOKEN}`);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Link nicht mehr gültig' }),
    ).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent(/schon benutzt, ist abgelaufen/);
    expect(screen.getByRole('link', { name: 'Zur Anmeldung' })).toHaveAttribute('href', '/login');
    expect(screen.queryByLabelText('E-Mail-Adresse')).not.toBeInTheDocument();
  });

  it('says so for an unknown link (404)', async () => {
    mockApi({ ...visitor, [`GET /invitations/${TOKEN}`]: () => problem(404, 'not_found') });
    renderApp(`/invite/${TOKEN}`);
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Link nicht gefunden' }),
    ).toBeVisible();
    expect(screen.queryByLabelText('E-Mail-Adresse')).not.toBeInTheDocument();
  });

  it('still shows the page when a session exists', async () => {
    mockApi({
      'GET /auth/session': () => json(session),
      [`GET /invitations/${TOKEN}`]: () => json(preview),
    });
    renderApp(`/invite/${TOKEN}`);
    expect(await screen.findByLabelText('E-Mail-Adresse')).toBeVisible();
  });

  it('checks the password length in the client, then posts and opens the start page', async () => {
    const guest = {
      account: { ...session.account, id: 5, role: 'guest' as const, maxRating: 12 as const },
      profile: { ...session.profile!, id: 9, name: 'Oma', isPrimary: true, maxRating: 12 as const },
    };
    let accepted = false;
    const { calls } = mockApi({
      'GET /auth/session': () => (accepted ? json(guest) : problem(401, 'unauthenticated')),
      'GET /setup': () => json({ setupRequired: false }),
      [`GET /invitations/${TOKEN}`]: () => json(preview),
      [`POST /invitations/${TOKEN}/accept`]: () => {
        accepted = true;
        return json(guest, 201);
      },
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp(`/invite/${TOKEN}`);
    const user = userEvent.setup();

    await user.type(await screen.findByLabelText('E-Mail-Adresse'), 'oma@example.test');
    await user.type(screen.getByLabelText(/^Passwort/), 'short');
    await user.type(screen.getByLabelText(/^Profilname/), 'Oma');
    await user.click(screen.getByRole('button', { name: 'Konto anlegen' }));
    expect(
      await screen.findByText('Mindestens 10 Zeichen.', { selector: '[slot="errorMessage"]' }),
    ).toBeVisible();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);

    await user.clear(screen.getByLabelText(/^Passwort/));
    await user.type(screen.getByLabelText(/^Passwort/), 'correct horse battery');
    await user.click(screen.getByRole('radio', { name: 'English' }));
    await user.click(screen.getByRole('button', { name: 'Konto anlegen' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      email: 'oma@example.test',
      password: 'correct horse battery',
      profileName: 'Oma',
      language: 'en',
    });
  });

  it('turns a 410 on submit into the "no longer valid" page', async () => {
    mockApi({
      ...visitor,
      [`GET /invitations/${TOKEN}`]: () => json(preview),
      [`POST /invitations/${TOKEN}/accept`]: () => problem(410, 'invitation_gone'),
    });
    renderApp(`/invite/${TOKEN}`);
    const user = userEvent.setup();

    await user.type(await screen.findByLabelText('E-Mail-Adresse'), 'oma@example.test');
    await user.type(screen.getByLabelText(/^Passwort/), 'correct horse battery');
    await user.type(screen.getByLabelText(/^Profilname/), 'Oma');
    await user.click(screen.getByRole('button', { name: 'Konto anlegen' }));

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Link nicht mehr gültig' }),
    ).toBeVisible();
  });

  it('shows an e-mail address that is taken at the field', async () => {
    mockApi({
      ...visitor,
      [`GET /invitations/${TOKEN}`]: () => json(preview),
      [`POST /invitations/${TOKEN}/accept`]: () =>
        problem(422, 'validation_failed', { errors: [{ field: 'email', code: 'already_exists' }] }),
    });
    renderApp(`/invite/${TOKEN}`);
    const user = userEvent.setup();

    await user.type(await screen.findByLabelText('E-Mail-Adresse'), 'oma@example.test');
    await user.type(screen.getByLabelText(/^Passwort/), 'correct horse battery');
    await user.type(screen.getByLabelText(/^Profilname/), 'Oma');
    await user.click(screen.getByRole('button', { name: 'Konto anlegen' }));

    expect(await screen.findByText('Diese E-Mail-Adresse ist schon vergeben.')).toBeVisible();
    expect(screen.getByLabelText('E-Mail-Adresse')).toHaveAttribute('aria-invalid', 'true');
  });
});
