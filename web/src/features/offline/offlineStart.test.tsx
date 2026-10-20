import { onlineManager } from '@tanstack/react-query';
import { screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { homePage } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';
import { resetOfflineStore } from '@/test/offlineStore';
import { SESSION_TIMEOUT_MS } from '@/app/router';
import { chooseOfflineProfile, offlineChoice } from './offlineSession';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));
vi.mock('@/features/offline/engine', () => ({
  startOfflineEngine: () => () => undefined,
  kickOfflineEngine: vi.fn(),
  cancelFetch: vi.fn(() => Promise.resolve()),
  rememberDeleted: vi.fn(() => Promise.resolve()),
  flushEventsSoon: vi.fn(),
}));
const support = vi.hoisted(() => ({ supported: true }));
vi.mock('@/features/offline/support', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./support')>()),
  offlineSupported: () => support.supported,
}));

beforeEach(() => {
  resetOfflineStore();
  support.supported = true;
});

afterEach(() => {
  vi.restoreAllMocks();
  onlineManager.setOnline(true);
});

describe('starting without network (C-1)', () => {
  it('goes to the offline library at once when the browser is offline', async () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    onlineManager.setOnline(false);
    const { calls } = mockApi({
      'GET /auth/session': () => {
        throw new TypeError('Failed to fetch');
      },
    });
    const started = Date.now();
    const { router } = renderApp('/');
    expect(await screen.findByRole('heading', { name: 'Offline-Bibliothek' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/offline');
    // One attempt that fails at once, no wait for the timeout.
    expect(calls.filter((call) => call.path === '/auth/session')).toHaveLength(1);
    expect(Date.now() - started).toBeLessThan(SESSION_TIMEOUT_MS);
  });

  it('a browser that only claims to be offline still opens the app (server here, K11)', async () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    onlineManager.setOnline(false);
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { router } = renderApp('/');
    expect(await screen.findByRole('navigation', { name: 'Hauptnavigation' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/');
  });

  it('a server that never answers gives the offline library after the timeout (W10)', async () => {
    mockApi({ 'GET /auth/session': () => new Promise<Response>(() => undefined) });
    const started = Date.now();
    const { router } = renderApp('/');
    expect(
      await screen.findByRole(
        'heading',
        { name: 'Offline-Bibliothek' },
        { timeout: SESSION_TIMEOUT_MS + 2000 },
      ),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe('/offline');
    expect(Date.now() - started).toBeGreaterThanOrEqual(SESSION_TIMEOUT_MS - 50);
  });

  it('a 5xx shows the error page with the way to the offline library (W10)', async () => {
    mockApi({ 'GET /auth/session': () => problem(502, 'bad_gateway') });
    renderApp('/');
    expect(await screen.findByText('Slouchcat ist gerade nicht erreichbar')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Offline-Bibliothek öffnen' })).toHaveAttribute(
      'href',
      '/offline',
    );
  });

  it('a page outside the offline library forgets the profile chosen there (W1)', async () => {
    chooseOfflineProfile({ accountId: 1, profileId: 2 });
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');
    expect(await screen.findByRole('navigation', { name: 'Hauptnavigation' })).toBeVisible();
    expect(offlineChoice()).toBeNull();
  });

  it('a preload of the app (hovering "Zur App") keeps the choice of the library (W1)', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { router } = renderApp('/offline');
    expect(await screen.findByRole('heading', { name: 'Offline-Bibliothek' })).toBeVisible();
    chooseOfflineProfile({ accountId: 1, profileId: 1 });
    await router.preloadRoute({ to: '/' });
    expect(offlineChoice()).toEqual({ accountId: 1, profileId: 1 });
  });

  it('the player from /downloads asks for the session after a reload (K9)', async () => {
    const { calls } = mockApi({ 'GET /auth/session': () => problem(401, 'unauthenticated') });
    const { router } = renderApp('/offline/play/01JB8Z6Q4M7N9P2R3S5T7V9D01?as=session');
    expect(await screen.findByRole('heading', { name: 'Offline-Bibliothek' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/offline');
    expect(calls.some((call) => call.path === '/auth/session')).toBe(true);
  });

  it('goes to the offline library when the session request fails at the network', async () => {
    mockApi({
      'GET /auth/session': () => {
        throw new TypeError('Failed to fetch');
      },
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp('/items/01JB8Z6Q4M7N9P2R3S5T7V9W2Y');
    expect(await screen.findByRole('heading', { name: 'Offline-Bibliothek' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/offline');
  });

  it('keeps the error page for other failures and where downloads are impossible', async () => {
    mockApi({ 'GET /auth/session': () => problem(404, 'not_found') });
    renderApp('/');
    expect(await screen.findByText('Slouchcat ist gerade nicht erreichbar')).toBeVisible();
    expect(screen.queryByRole('link', { name: 'Offline-Bibliothek öffnen' })).toBeNull();
  });

  it('without download storage a network failure still shows the error page', async () => {
    support.supported = false;
    mockApi({
      'GET /auth/session': () => {
        throw new TypeError('Failed to fetch');
      },
    });
    const { router } = renderApp('/');
    expect(await screen.findByText('Slouchcat ist gerade nicht erreichbar')).toBeVisible();
    expect(router.state.location.pathname).toBe('/');
  });

  it('without download storage the navigation offers no downloads page (K13)', async () => {
    support.supported = false;
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');
    const nav = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(nav).toHaveTextContent('Start');
    expect(screen.queryByRole('link', { name: 'Downloads' })).toBeNull();
  });

  it('online nothing changes: the start page opens', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { router } = renderApp('/');
    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    expect(await screen.findByRole('navigation', { name: 'Hauptnavigation' })).toBeVisible();
  });

  it('the offline player without a profile chosen in the library leads there', async () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    onlineManager.setOnline(false);
    mockApi(signedIn);
    const { router } = renderApp('/offline/play/01JB8Z6Q4M7N9P2R3S5T7V9D01');
    expect(await screen.findByRole('heading', { name: 'Offline-Bibliothek' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/offline');
  });
});
