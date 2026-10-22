import { QueryClient } from '@tanstack/react-query';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import i18n from '@/i18n';
import { homePage } from '@/test/fixtures';
import { json, mockApi, renderApp, session, signedIn } from '@/test/api';
import { adoptSession, clearProfileData, IMAGE_CACHE, sessionQuery } from './session';

const filled = () => {
  const client = new QueryClient();
  client.setQueryData(['home'], homePage);
  client.setQueryData(['item', 'x'], { id: 'x' });
  client.setQueryData(['session'], session);
  client.setQueryData(['setup-status'], { setupRequired: false });
  client.setQueryData(['auth', 'profiles'], [{ id: 1 }]);
  return client;
};

const keysOf = (client: QueryClient) =>
  client
    .getQueryCache()
    .getAll()
    .map((query) => String(query.queryKey[0]))
    .sort();

describe('clearProfileData', () => {
  const remove = vi.fn(() => Promise.resolve(true));

  beforeEach(() => {
    remove.mockClear();
    vi.stubGlobal('caches', { delete: remove });
  });
  afterEach(() => vi.unstubAllGlobals());

  it('forgets the catalog, the account lists and the cached images, but not session or setup', async () => {
    const client = filled();
    await clearProfileData(client);
    expect(keysOf(client)).toEqual(['session', 'setup-status']);
    expect(remove).toHaveBeenCalledWith(IMAGE_CACHE);
    expect(IMAGE_CACHE).toBe('slouchcat-images');
  });

  it('keeps the profile list of the same account for a switch', async () => {
    const client = filled();
    await clearProfileData(client, { sameAccount: true });
    expect(keysOf(client)).toEqual(['auth', 'session', 'setup-status']);
    expect(remove).toHaveBeenCalledWith(IMAGE_CACHE);
  });

  it('does not stop when there is no Cache API or the delete fails', async () => {
    vi.stubGlobal('caches', undefined);
    await expect(clearProfileData(filled())).resolves.toBeUndefined();
    vi.stubGlobal('caches', { delete: () => Promise.reject(new Error('denied')) });
    await expect(clearProfileData(filled())).resolves.toBeUndefined();
  });
});

describe('adoptSession', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('stores the session, clears the old data, and applies language (loading it) and accent', async () => {
    vi.stubGlobal('caches', { delete: vi.fn(() => Promise.resolve(true)) });
    const client = filled();
    const english = {
      ...session,
      profile: { ...session.profile!, language: 'en' as const, accent: 'teal' as const },
    };
    await adoptSession(client, english);

    expect(client.getQueryData(sessionQuery.queryKey)).toEqual(english);
    expect(client.getQueryData(['home'])).toBeUndefined();
    expect(i18n.hasResourceBundle('en', 'translation')).toBe(true);
    expect(i18n.resolvedLanguage).toBe('en');
    expect(document.documentElement.style.getPropertyValue('--sc-accent')).toBe(
      'var(--sc-color-accent-teal-base)',
    );
    document.documentElement.style.removeProperty('--sc-accent');
    document.documentElement.style.removeProperty('--sc-on-accent');
  });
});

describe('logout', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('clears the caches, resets accent and language and opens the login', async () => {
    const remove = vi.fn(() => Promise.resolve(true));
    vi.stubGlobal('caches', { delete: remove });
    let signedOut = false;
    mockApi({
      ...signedIn,
      'GET /auth/session': () => (signedOut ? new Response('{}', { status: 401 }) : json(session)),
      'POST /auth/logout': () => {
        signedOut = true;
        return new Response(null, { status: 204 });
      },
      'GET /home': () => json(homePage),
      'GET /setup': () => json({ setupRequired: false }),
    });
    const { router, queryClient } = renderApp('/');
    const user = userEvent.setup();
    // Something the profile had open that no page on screen asks for again.
    queryClient.setQueryData(['item', 'secret'], { id: 'secret' });

    await user.click(await screen.findByRole('button', { name: 'Abmelden' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(remove).toHaveBeenCalledWith(IMAGE_CACHE);
    expect(queryClient.getQueryData(['item', 'secret'])).toBeUndefined();
    expect(queryClient.getQueryData(sessionQuery.queryKey)).toBeNull();
  });

  describe('with a push subscription in this browser', () => {
    afterEach(() => {
      Reflect.deleteProperty(navigator, 'serviceWorker');
      Reflect.deleteProperty(window, 'isSecureContext');
    });

    /** A browser whose service worker holds a push subscription. */
    function subscribedBrowser() {
      const unsubscribe = vi.fn(() => Promise.resolve(true));
      const registration = {
        pushManager: { getSubscription: () => Promise.resolve({ unsubscribe }) },
      };
      Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true });
      Object.defineProperty(navigator, 'serviceWorker', {
        value: { getRegistration: () => Promise.resolve(registration) },
        configurable: true,
      });
      vi.stubGlobal('PushManager', function PushManager() {});
      vi.stubGlobal('Notification', { requestPermission: vi.fn() });
      return { unsubscribe };
    }

    const channels = json({
      channels: [
        {
          channel: 'push',
          enabled: true,
          events: ['new_content'],
          available: true,
          push: {
            subscriptions: [
              { id: 3, createdAt: '2026-10-20T10:00:00Z', thisDevice: false },
              { id: 4, createdAt: '2026-10-21T10:00:00Z', thisDevice: true },
            ],
          },
        },
      ],
    });

    it.each([
      ['removes it on the server first, then ends it in the browser', 204],
      ['ends it in the browser even when the server refuses (another profile)', 403],
    ])('%s', async (_name, status) => {
      const { unsubscribe } = subscribedBrowser();
      const { calls } = mockApi({
        ...signedIn,
        'GET /me/channels': () => channels.clone(),
        'DELETE /me/push-subscriptions/4': () =>
          status === 204
            ? new Response(null, { status })
            : json({ code: 'primary_profile_required', status }, status),
        'POST /auth/logout': () => new Response(null, { status: 204 }),
        'GET /home': () => json(homePage),
        'GET /setup': () => json({ setupRequired: false }),
      });
      const { router } = renderApp('/');
      await userEvent.setup().click(await screen.findByRole('button', { name: 'Abmelden' }));
      await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
      const order = calls
        .filter((call) => call.method !== 'GET')
        .map((call) => `${call.method} ${call.path}`);
      expect(order).toEqual(['DELETE /me/push-subscriptions/4', 'POST /auth/logout']);
      expect(unsubscribe).toHaveBeenCalledTimes(1);
    });
  });
});
