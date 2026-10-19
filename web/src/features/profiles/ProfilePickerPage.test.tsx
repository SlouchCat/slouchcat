import type { SessionInfo } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { homePage, summaries } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, type Routes } from '@/test/api';

// Signed in, no profile chosen yet (the state after a login with several profiles).
const noProfile: SessionInfo = { account: session.account };

const picker = (current: SessionInfo = noProfile): Routes => ({
  'GET /auth/session': () => json(current),
  'GET /auth/profiles': () => json(summaries),
  'GET /home': () => json(homePage),
});

const lena = {
  ...session.profile!,
  id: 2,
  name: 'Lena',
  isPrimary: false,
  language: 'en' as const,
};

describe('profile picker', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('sends a session without profile to the picker and lists every profile', async () => {
    mockApi(picker());
    const { router } = renderApp('/');

    expect(await screen.findByRole('heading', { level: 1, name: 'Wer schaut?' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/profiles');
    const list = await screen.findByRole('list', { name: 'Profile' });
    const tiles = within(list).getAllByRole('button');
    expect(tiles.map((tile) => tile.textContent)).toEqual([
      expect.stringContaining('Marco'),
      expect.stringContaining('Lena'),
      expect.stringContaining('Noah'),
    ]);
    // A PIN profile says so to screen readers; the first tile has the focus.
    expect(screen.getByRole('button', { name: /Lena.*mit PIN geschützt/ })).toBeVisible();
    expect(tiles[0]).toHaveFocus();
    // No profile is active, so there is nothing to manage yet.
    expect(screen.queryByRole('link', { name: 'Profile verwalten' })).not.toBeInTheDocument();
  });

  it('keeps the page the visitor wanted and returns to it after the pick', async () => {
    const { calls } = mockApi({
      ...picker(),
      'GET /items/*': () => problem(404, 'not_found'),
      'POST /auth/profiles/3/select': () =>
        json({ ...session, profile: { ...session.profile!, id: 3, name: 'Noah' } }),
    });
    const { router } = renderApp('/items/01JB8Z6Q4M7N9P2R3S5T7V9W1X');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: /Noah/ }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/items/01JB8Z6Q4M7N9P2R3S5T7V9W1X'),
    );
    const select = calls.find((call) => call.path === '/auth/profiles/3/select');
    expect(select?.body).toBeNull();
  });

  it('ignores a return target that leaves the app or leads back to the picker', async () => {
    for (const next of ['//evil.example/x', '/\\evil.example', '/profiles', '/profiles?x=1']) {
      mockApi({
        ...picker(),
        'POST /auth/profiles/1/select': () => json(session),
      });
      const { router, unmount } = renderApp(`/profiles?next=${encodeURIComponent(next)}`);
      const user = userEvent.setup();
      await user.click(await screen.findByRole('button', { name: /Marco/ }));
      await waitFor(() => expect(router.state.location.pathname).toBe('/'));
      expect(router.state.location.href).toBe('/');
      unmount();
      vi.unstubAllGlobals();
    }
  });

  it('selects a profile without PIN with Enter, applies its language and opens the start page', async () => {
    mockApi({
      ...picker(),
      'POST /auth/profiles/1/select': () =>
        json({ ...session, profile: { ...session.profile!, language: 'en' } }),
    });
    const { router } = renderApp('/profiles');
    const user = userEvent.setup();

    // The first tile takes focus on its own (norigin remembers the last focus across tests, so
    // this one moves it there itself).
    const marco = await screen.findByRole('button', { name: /Marco/ });
    marco.focus();
    await user.keyboard('{Enter}');

    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    expect(await screen.findByRole('heading', { name: 'Continue watching' })).toBeVisible();
  });

  it('highlights the active profile and offers "Profile verwalten" only for the primary one', async () => {
    mockApi(picker(session));
    renderApp('/profiles');
    const marco = await screen.findByRole('button', { name: /Marco/ });
    expect(marco).toHaveAttribute('aria-current', 'true');
    expect(screen.getByRole('link', { name: 'Profile verwalten' })).toHaveAttribute(
      'href',
      '/settings/profiles',
    );
  });

  it('hides "Profile verwalten" when a non-primary profile is active', async () => {
    mockApi(picker({ ...session, profile: lena }));
    renderApp('/profiles');
    expect(await screen.findByRole('button', { name: /Lena/ })).toHaveAttribute(
      'aria-current',
      'true',
    );
    expect(screen.queryByRole('link', { name: 'Profile verwalten' })).not.toBeInTheDocument();
  });

  describe('PIN dialog', () => {
    const openLena = async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      await user.click(await screen.findByRole('button', { name: /Lena/ }));
      const dialog = await screen.findByRole('dialog', { name: 'PIN für Lena' });
      return { user, dialog };
    };

    it('asks for the PIN, masks it and posts digits only', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const { calls } = mockApi({
        ...picker(),
        'POST /auth/profiles/2/select': () => json({ ...session, profile: lena }),
      });
      const { router } = renderApp('/profiles');
      const { user, dialog } = await openLena();

      const field = within(dialog).getByLabelText('PIN');
      expect(field).toHaveAttribute('type', 'password');
      expect(field).toHaveAttribute('inputmode', 'numeric');
      expect(field).toHaveFocus();
      await user.type(field, '12a34');
      expect(field).toHaveValue('1234');
      await user.keyboard('{Enter}');

      await waitFor(() => expect(router.state.location.pathname).toBe('/'));
      expect(calls.find((call) => call.path === '/auth/profiles/2/select')?.body).toEqual({
        pin: '1234',
      });
    });

    it('rejects a PIN that is not 4 to 8 digits without asking the server', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const { calls } = mockApi(picker());
      renderApp('/profiles');
      const { user, dialog } = await openLena();

      await user.type(within(dialog).getByLabelText('PIN'), '12');
      await user.click(within(dialog).getByRole('button', { name: 'Öffnen' }));
      expect(await within(dialog).findByText('Die PIN besteht aus 4 bis 8 Ziffern.')).toBeVisible();
      expect(calls.some((call) => call.method === 'POST')).toBe(false);
    });

    it('shows pin_invalid, clears the field and keeps the dialog open', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      mockApi({
        ...picker(),
        'POST /auth/profiles/2/select': () => problem(403, 'pin_invalid'),
      });
      renderApp('/profiles');
      const { user, dialog } = await openLena();

      await user.type(within(dialog).getByLabelText('PIN'), '0000{Enter}');
      expect(await within(dialog).findByRole('alert')).toHaveTextContent(
        'Die PIN stimmt nicht. Versuche es noch einmal.',
      );
      expect(within(dialog).getByLabelText('PIN')).toHaveValue('');
    });

    it('counts a pin_locked pause down and enables the button again at zero', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const { calls } = mockApi({
        ...picker(),
        'POST /auth/profiles/2/select': () =>
          Response.json(
            {
              type: 'about:blank',
              title: 'pin_locked',
              status: 429,
              code: 'pin_locked',
              retryAfterSeconds: 30,
            },
            {
              status: 429,
              headers: { 'Content-Type': 'application/problem+json', 'Retry-After': '30' },
            },
          ),
      });
      renderApp('/profiles');
      const { user, dialog } = await openLena();

      await user.type(within(dialog).getByLabelText('PIN'), '1111{Enter}');
      const status = await within(dialog).findByRole('status');
      expect(status).toHaveTextContent('Zu viele Fehlversuche. Neuer Versuch in');
      expect(status).toHaveTextContent('0:30');
      const submit = within(dialog).getByRole('button', { name: 'Öffnen' });
      expect(submit).toBeDisabled();

      await act(() => vi.advanceTimersByTimeAsync(10_000));
      expect(status).toHaveTextContent('0:20');
      expect(submit).toBeDisabled();

      // Typing and Enter during the pause do not reach the server.
      await user.type(within(dialog).getByLabelText('PIN'), '2222{Enter}');
      expect(calls.filter((call) => call.method === 'POST')).toHaveLength(1);

      await act(() => vi.advanceTimersByTimeAsync(20_500));
      await waitFor(() => expect(submit).toBeEnabled());
      expect(within(dialog).queryByRole('status')).not.toBeInTheDocument();
    });

    it('keeps the PIN out of the mutation cache, after a wrong and after the right PIN', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      let answers = 0;
      mockApi({
        ...picker(),
        'POST /auth/profiles/2/select': () =>
          (answers += 1) === 1 ? problem(403, 'pin_invalid') : json({ ...session, profile: lena }),
      });
      const { queryClient, router } = renderApp('/profiles');
      const { user, dialog } = await openLena();
      const holdsPin = () =>
        queryClient
          .getMutationCache()
          .getAll()
          .some((mutation) => JSON.stringify(mutation.state.variables ?? null).includes('"pin"'));

      await user.type(within(dialog).getByLabelText('PIN'), '9999{Enter}');
      await within(dialog).findByRole('alert');
      await act(() => vi.advanceTimersByTimeAsync(50));
      expect(queryClient.getMutationCache().getAll()).toHaveLength(0);
      expect(holdsPin()).toBe(false);

      await user.type(within(dialog).getByLabelText('PIN'), '1234{Enter}');
      await waitFor(() => expect(router.state.location.pathname).toBe('/'));
      await act(() => vi.advanceTimersByTimeAsync(50));
      expect(holdsPin()).toBe(false);
    });

    it('treats the attempt limit (too_many_attempts) like a lock: countdown, button held', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      mockApi({
        ...picker(),
        'POST /auth/profiles/2/select': () =>
          Response.json(
            { type: 'about:blank', title: 'x', status: 429, code: 'too_many_attempts' },
            {
              status: 429,
              headers: { 'Content-Type': 'application/problem+json', 'Retry-After': '12' },
            },
          ),
      });
      renderApp('/profiles');
      const { user, dialog } = await openLena();

      await user.type(within(dialog).getByLabelText('PIN'), '1111{Enter}');
      const status = await within(dialog).findByRole('status');
      expect(status).toHaveTextContent('0:12');
      // Read out once, in words, for screen readers.
      expect(status).toHaveTextContent('Gesperrt für 12 Sekunden.');
      expect(within(dialog).getByRole('button', { name: 'Öffnen' })).toBeDisabled();
    });

    it('closes with Escape and returns focus to the tile', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      mockApi(picker());
      renderApp('/profiles');
      const { user, dialog } = await openLena();

      await user.keyboard('{Escape}');
      await waitFor(() => expect(dialog).not.toBeInTheDocument());
      expect(screen.getByRole('button', { name: /Lena/ })).toHaveFocus();
    });
  });
});
