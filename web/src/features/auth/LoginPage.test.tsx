import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { setLanguage } from '@/i18n';
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

  it('switches the page language with the language button, by keyboard (concept 22, 4.4)', async () => {
    mockApi(signedOut);
    renderApp('/login');
    const user = userEvent.setup();
    const button = await screen.findByRole('button', { name: 'Sprache: Deutsch' });
    button.focus();
    await user.keyboard('{Enter}');
    const menu = await screen.findByRole('menu', { name: 'Sprache: Deutsch' });
    // Every shipped language in its own language; the current one is checked.
    const items = within(menu).getAllByRole('menuitemradio');
    expect(items.map((item) => item.textContent)).toEqual(['Deutsch', 'English']);
    expect(within(menu).getByRole('menuitemradio', { name: 'Deutsch' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    await user.keyboard('{ArrowDown}');
    await user.keyboard('{Enter}');
    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
    expect(document.documentElement.lang).toBe('en');
    // The focus is back on the button, which names the new language.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Language: English' })).toHaveFocus(),
    );
    await setLanguage('de');
  });
});
