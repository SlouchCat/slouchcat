import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { homePage } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session } from '@/test/api';

const signedOut = {
  'GET /setup': () => json({ setupRequired: false }),
  'GET /auth/session': () => problem(401, 'unauthenticated'),
  'GET /healthz': () => json({ status: 'ok', version: '0.2.0', uptimeSeconds: 1 }),
};

describe('login page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('redirects a visitor without session to the login', async () => {
    mockApi(signedOut);
    const { router } = renderApp('/');
    expect(await screen.findByRole('heading', { level: 1, name: 'Anmelden' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/login');
  });

  it('shows invalid_credentials on 401 and keeps the user on the page', async () => {
    const { calls } = mockApi({
      ...signedOut,
      'POST /auth/login': () => problem(401, 'invalid_credentials'),
    });
    renderApp('/login');
    const user = userEvent.setup();

    await user.type(await screen.findByLabelText(/E-Mail-Adresse/), 'owner@example.test');
    await user.type(screen.getByLabelText(/Passwort/), 'wrong password');
    await user.click(screen.getByRole('button', { name: 'Anmelden' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'E-Mail-Adresse oder Passwort stimmen nicht.',
    );
    const login = calls.find((call) => call.path === '/auth/login');
    expect(login?.body).toEqual({ email: 'owner@example.test', password: 'wrong password' });
  });

  it('validates empty fields in the client without calling the server', async () => {
    const { calls } = mockApi(signedOut);
    renderApp('/login');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Anmelden' }));
    expect(await screen.findAllByText('Pflichtfeld.')).toHaveLength(2);
    expect(calls.some((call) => call.path === '/auth/login')).toBe(false);
  });

  it('logs in, switches to the profile language and opens the start page', async () => {
    mockApi({
      ...signedOut,
      'POST /auth/login': () =>
        json({ ...session, profile: { ...session.profile, language: 'en' } }),
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp('/login');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText(/E-Mail-Adresse/), 'owner@example.test');
    await user.type(screen.getByLabelText(/Passwort/), 'correct horse battery');
    await user.click(screen.getByRole('button', { name: 'Anmelden' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    expect(await screen.findByRole('heading', { name: 'Continue watching' })).toBeVisible();
  });
});
