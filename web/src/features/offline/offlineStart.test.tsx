import { onlineManager } from '@tanstack/react-query';
import { screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { homePage } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';
import { resetOfflineStore } from '@/test/offlineStore';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));
vi.mock('@/features/offline/engine', () => ({
  startOfflineEngine: () => () => undefined,
  kickOfflineEngine: vi.fn(),
  cancelFetch: vi.fn(),
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
  it('goes straight to the offline library when the browser is offline', async () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    onlineManager.setOnline(false);
    const { calls } = mockApi(signedIn);
    const { router } = renderApp('/');
    expect(await screen.findByRole('heading', { name: 'Offline-Bibliothek' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/offline');
    // No request waited for a network that is not there.
    expect(calls.some((call) => call.path === '/auth/session')).toBe(false);
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
    mockApi({ 'GET /auth/session': () => problem(500, 'internal') });
    renderApp('/');
    expect(await screen.findByText('Slouchcat ist gerade nicht erreichbar')).toBeVisible();
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

  it('online nothing changes: the start page opens', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { router } = renderApp('/');
    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    expect(await screen.findByRole('navigation', { name: 'Hauptnavigation' })).toBeVisible();
  });

  it('the offline player without a chosen profile or session leads to the library', async () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    onlineManager.setOnline(false);
    mockApi({});
    const { router } = renderApp('/offline/play/01JB8Z6Q4M7N9P2R3S5T7V9D01');
    expect(await screen.findByRole('heading', { name: 'Offline-Bibliothek' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/offline');
  });
});
