import { onlineManager } from '@tanstack/react-query';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, onTestFinished, vi } from 'vitest';
import { formatBytes } from '@/lib/format';
import { json, mockApi, renderApp, session, signedIn } from '@/test/api';
import { resetOfflineFiles } from '@/test/offlineFiles';
import {
  downloadIds,
  episodeDownload,
  heatDownload,
  offlineProfile,
  rules,
  storedDownload,
  storedProfile,
} from '@/test/offlineFixtures';
import {
  getMeta,
  putDownloads,
  putProfiles,
  resetOfflineStore,
  storedDownloads,
} from '@/test/offlineStore';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));
vi.mock('@/features/offline/engine', () => ({
  startOfflineEngine: () => () => undefined,
  kickOfflineEngine: vi.fn(),
  cancelFetch: vi.fn(() => Promise.resolve()),
  rememberDeleted: vi.fn(() => Promise.resolve()),
  flushEventsSoon: vi.fn(),
}));
vi.mock('@/features/offline/support', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./support')>()),
  offlineSupported: () => true,
}));

const IPHONE =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Mobile/15E148 Safari/604.1';

function stubStorage(usage = 3e9, quota = 14e9) {
  Object.defineProperty(navigator, 'storage', {
    configurable: true,
    value: {
      estimate: () => Promise.resolve({ usage, quota }),
      persisted: () => Promise.resolve(true),
      persist: vi.fn(() => Promise.resolve(true)),
    },
  });
}

const routes = {
  ...signedIn,
  'GET /offline/rules': () => json(rules),
};

beforeEach(async () => {
  resetOfflineStore();
  resetOfflineFiles();
  stubStorage();
  await putDownloads([
    storedDownload(),
    heatDownload(),
    episodeDownload('e1', {
      state: 'preparing',
      complete: false,
      bytesWritten: 0,
      segmentsDone: 1,
      segmentsTotal: 4,
      manifest: undefined,
      createdAt: '2026-10-20T11:00:00Z',
    }),
  ]);
});

afterEach(() => {
  vi.restoreAllMocks();
});

const table = () => screen.findByRole('table', { name: 'Auf diesem Gerät' });

describe('downloads page', () => {
  it('lists only the downloads the server allows for the active profile, with their state', async () => {
    mockApi(routes);
    renderApp('/downloads');
    const list = await table();
    const rows = within(list).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(2);
    expect(within(list).queryByText('Heat')).not.toBeInTheDocument();
    const alienRow = within(list).getByRole('rowheader', { name: /Alien/ }).closest('tr')!;
    expect(within(alienRow).getByText('Auf diesem Gerät')).toBeVisible();
    // It plays for the session's profile, never for one chosen in the offline library (W1).
    expect(within(alienRow).getByRole('link', { name: 'Alien abspielen' })).toHaveAttribute(
      'href',
      `/offline/play/${downloadIds.alien}?as=session`,
    );
    const episodeRow = within(list)
      .getByRole('rowheader', { name: /S01E01 · Pilot · Slouchcat Test Show/ })
      .closest('tr')!;
    expect(within(episodeRow).getByText('Wird vorbereitet (25 %)')).toBeVisible();
    expect(within(episodeRow).queryByRole('link')).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Hauptnavigation' })).toContainElement(
      screen.getByRole('link', { name: 'Downloads' }),
    );
    expect(screen.getByRole('link', { name: 'Downloads' })).toHaveAttribute('aria-current', 'page');
  });

  it('deletes a download on the server and on the device', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...routes,
      'DELETE /downloads/*': () => new Response(null, { status: 204 }),
    });
    renderApp('/downloads');
    const list = await table();
    await user.click(within(list).getByRole('button', { name: 'Alien löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Download Alien löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Download löschen' }));
    await waitFor(() =>
      expect(storedDownloads().map((download) => download.id)).not.toContain(downloadIds.alien),
    );
    expect(
      calls.some(
        (call) => call.method === 'DELETE' && call.path === `/downloads/${downloadIds.alien}`,
      ),
    ).toBe(true);
    expect(await screen.findByText('Alien wurde gelöscht.')).toBeVisible();
  });

  it('keeps a delete made offline for later and hides the download', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi(routes);
    renderApp('/downloads');
    const list = await table();
    // The connection drops while the page is open.
    const offline = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    onTestFinished(() => {
      offline.mockRestore();
      // The query client heard "offline"; the next test starts online again.
      onlineManager.setOnline(true);
    });
    act(() => {
      window.dispatchEvent(new Event('offline'));
    });
    expect(await screen.findByText(/Keine Verbindung zum Server/)).toBeVisible();
    await user.click(within(list).getByRole('button', { name: 'Alien löschen' }));
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Download löschen' }));
    await waitFor(() =>
      expect(storedDownloads().find((download) => download.id === downloadIds.alien)).toMatchObject(
        {
          pendingDelete: true,
        },
      ),
    );
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);
    await waitFor(() => expect(within(list).queryByText('Alien')).not.toBeInTheDocument());
  });

  it('a delete the server fails waits as pendingDelete for the next contact (K3)', async () => {
    const user = userEvent.setup();
    mockApi({
      ...routes,
      'DELETE /downloads/*': () => Response.json({ code: 'internal' }, { status: 500 }),
    });
    renderApp('/downloads');
    const list = await table();
    await user.click(within(list).getByRole('button', { name: 'Alien löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Download Alien löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Download löschen' }));
    await waitFor(() =>
      expect(storedDownloads().find((download) => download.id === downloadIds.alien)).toMatchObject(
        { pendingDelete: true, complete: false },
      ),
    );
    // No play button for files that are gone.
    await waitFor(() => expect(within(list).queryByText('Alien')).not.toBeInTheDocument());
  });

  it('signing out keeps the downloads but locks them on this device (W2)', async () => {
    const user = userEvent.setup();
    mockApi({ ...routes, 'POST /auth/logout': () => new Response(null, { status: 204 }) });
    renderApp('/downloads');
    await table();
    await user.click(screen.getByRole('button', { name: 'Abmelden' }));
    await waitFor(async () => expect(await getMeta('signedOut')).toBe(true));
    expect(storedDownloads().map((download) => download.id)).toContain(downloadIds.alien);
  });

  it('shows the storage: estimate, the downloads’ share and whether it is kept', async () => {
    mockApi(routes);
    renderApp('/downloads');
    const meter = await screen.findByRole('meter', { name: 'Speicherbelegung' });
    expect(meter).toHaveAttribute(
      'aria-valuetext',
      `${formatBytes(3e9, 'de')} von ${formatBytes(14e9, 'de')} belegt`,
    );
    const panel = screen.getByRole('region', { name: 'Speicher' });
    expect(panel).toHaveAttribute('data-usage-bytes', '3000000000');
    expect(panel).toHaveAttribute('data-quota-bytes', '14000000000');
    // Alien and Heat are complete (9000 B each); the episode holds nothing yet.
    expect(panel).toHaveAttribute('data-download-bytes', '18000');
    expect(within(panel).getByText('Der Browser behält die Downloads dauerhaft.')).toBeVisible();
    expect(
      within(panel).queryByText('Hinweis für iPhone, iPad und Safari'),
    ).not.toBeInTheDocument();
  });

  it('shows the Apple hint on iOS and hides the Wi-Fi option there (AN-13)', async () => {
    vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue(IPHONE);
    mockApi(routes);
    renderApp('/downloads');
    expect(await screen.findByText('Hinweis für iPhone, iPad und Safari')).toBeVisible();
    const form = await screen.findByRole('group', { name: 'Einstellungen' });
    expect(within(form).queryByRole('checkbox', { name: 'Nur im WLAN' })).not.toBeInTheDocument();
    expect(screen.getByText(/iPhone und iPad verraten dem Browser nicht/)).toBeVisible();
  });

  it('saves the auto-download rule of this device', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...routes,
      'PUT /offline/rules': (call) => json(call.body),
    });
    renderApp('/downloads');
    const group = await screen.findByRole('group', { name: 'Einstellungen' });
    await user.click(within(group).getByText('Automatische Downloads auf diesem Gerät'));
    const count = screen.getByRole('textbox', { name: 'Höchstens so viele ungesehene Folgen' });
    await user.clear(count);
    await user.type(count, '5');
    await user.click(screen.getByText('Hoch'));
    await user.click(screen.getByRole('button', { name: 'Regel speichern' }));
    expect(await screen.findByText(/Gespeichert/)).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      ...rules,
      enabled: true,
      maxCount: 5,
      quality: 'high',
    });
  });

  it('asks a protected profile for its PIN once, so it opens offline (AN-19)', async () => {
    const protectedSession = {
      ...session,
      profile: { ...session.profile!, hasPin: true },
    };
    await putProfiles([
      storedProfile({ snapshot: offlineProfile({ hasPin: true, pinSalt: 'a'.repeat(32) }) }),
    ]);
    mockApi({ ...routes, 'GET /auth/session': () => json(protectedSession) });
    renderApp('/downloads');
    expect(await screen.findByText('PIN für die Offline-Nutzung')).toBeVisible();
    expect(screen.getByRole('button', { name: 'PIN eingeben' })).toBeEnabled();
  });

  it('says so where the browser cannot keep downloads', async () => {
    const support = await import('./support');
    vi.spyOn(support, 'offlineSupported').mockReturnValue(false);
    mockApi(routes);
    renderApp('/downloads');
    expect(await screen.findByText('Downloads sind hier nicht möglich')).toBeVisible();
  });
});
