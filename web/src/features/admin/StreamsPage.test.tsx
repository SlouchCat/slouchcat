import type { AdminStream } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// W-6 streams for the main account's primary profile (concept 12, mockup Str-D2): tiles with the
// backdrop, «Serverlast» with the streams per mode and the bandwidth. The server lists and stops;
// the page shows the answers and sends «Beenden».

const streams: AdminStream[] = [
  {
    id: 'ps-1',
    accountEmail: 'owner@example.test',
    profileName: 'Marco',
    device: { id: 'dev-1', name: 'Chrome 156 · Linux' },
    itemId: '01JB8Z6Q4M7N9P2R3S5T7V9W1X',
    itemTitle: 'Slouchcat Test Multi Audio',
    mediaFileId: 'mf-1',
    mode: 'transcode',
    backend: 'nvenc',
    hardware: true,
    quality: '720p',
    width: 1280,
    height: 720,
    bitrate: 4_000_000,
    startedAt: '2026-10-20T08:00:00Z',
    lastActivityAt: '2026-10-20T08:30:00Z',
    positionMs: 65_000,
    durationMs: 600_000,
    backdropUrl: '/api/v1/admin/items/01JB8Z6Q4M7N9P2R3S5T7V9W1X/images/backdrop?w=780&h=abc',
  },
  {
    id: 'ps-2',
    accountEmail: 'gast@example.test',
    profileName: 'Lena',
    itemId: '01JB8Z6Q4M7N9P2R3S5T7V9W2Y',
    itemTitle: 'Slouchcat Test H264',
    mediaFileId: 'mf-2',
    mode: 'direct',
    hardware: false,
    startedAt: '2026-10-20T08:10:00Z',
    lastActivityAt: '2026-10-20T08:30:00Z',
  },
];

/** Four streams, one per load, for the counters of «Serverlast». */
const mixed: AdminStream[] = [
  ...streams,
  {
    ...streams[0]!,
    id: 'ps-3',
    profileName: 'Sandra',
    hardware: false,
    backend: 'libx264',
    bitrate: 2_500_000,
  },
  {
    ...streams[1]!,
    id: 'ps-4',
    profileName: 'Nora',
    mode: 'remux',
    bitrate: 9_800_000,
  },
];

const tileOf = (profile: string) =>
  screen.getByRole('button', { name: `Stream von ${profile} beenden` }).closest('article')!;

/** The list of tiles, in its section «Laufende Streams» (the admin sidebar has lists too). */
const tiles = () => screen.getByRole('region', { name: 'Laufende Streams' });

const loadOf = (label: string) =>
  within(screen.getByRole('group', { name: 'Serverlast' }))
    .getByText(label)
    .closest('dl')!;

describe('admin streams (W-6)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  const routes = (list: () => AdminStream[], extra: Routes = {}): Routes => ({
    ...signedIn,
    'GET /admin/streams': () => json(list()),
    ...extra,
  });

  it('shows each stream as a tile with profile, device, title, quality, mode and position', async () => {
    mockApi(routes(() => streams));
    renderApp('/admin/streams');
    expect(await screen.findByRole('heading', { level: 1, name: 'Streams' })).toBeVisible();
    await screen.findByRole('article', { name: 'Slouchcat Test Multi Audio' });
    const list = within(tiles()).getByRole('list');
    expect(within(list).getAllByRole('listitem')).toHaveLength(2);
    const marco = screen.getByRole('article', { name: 'Slouchcat Test Multi Audio' });
    expect(within(marco).getByRole('heading', { level: 3 })).toHaveTextContent(
      'Slouchcat Test Multi Audio',
    );
    expect(within(marco).getByText('Marco')).toBeVisible();
    expect(within(marco).getByText('owner@example.test')).toBeVisible();
    // Device and quality carry their labels for screen readers.
    expect(within(marco).getByText('Gerät').closest('dt')).not.toBeNull();
    expect(within(marco).getByText('Chrome 156 · Linux')).toBeVisible();
    // The unit comes from Intl ("Mb/s" or "Mbit/s" depending on the ICU data).
    expect(within(marco).getByText(/^720p · 4\sMb(it)?\/s$/)).toBeVisible();
    expect(within(marco).getByText('Transcode (HW)')).toBeVisible();
    expect(within(marco).getByText('1:05 von 10:00')).toBeVisible();
    expect(
      within(marco).getByRole('progressbar', { name: 'Fortschritt Slouchcat Test Multi Audio' }),
    ).toHaveAttribute('aria-valuenow', '11');
    expect(within(marco).getByText(/^Seit .*2026/)).toBeVisible();
    const lena = tileOf('Lena');
    expect(within(lena).getByText('Slouchcat Test H264', { selector: 'h3' })).toBeVisible();
    expect(within(lena).getByText('gast@example.test')).toBeVisible();
    expect(within(lena).getByText('Unbekanntes Gerät')).toBeVisible();
    expect(within(lena).getByText('Original')).toBeVisible();
    expect(within(lena).getByText('Direct')).toBeVisible();
    // No report yet: no bar, the position says so.
    expect(within(lena).getByText('–')).toBeVisible();
    expect(within(lena).queryByRole('progressbar')).toBeNull();
  });

  it('shows the backdrop across, and the title on the fallback without one or when it fails', async () => {
    mockApi(routes(() => streams));
    const { container } = renderApp('/admin/streams');
    const marco = await screen.findByRole('article', { name: 'Slouchcat Test Multi Audio' });
    // The image is decoration: the tile names the title in its heading.
    const image = marco.querySelector('img')!;
    expect(image).toHaveAttribute('src', streams[0]!.backdropUrl);
    expect(image).toHaveAttribute('alt', '');
    const lena = tileOf('Lena');
    expect(lena.querySelector('img')).toBeNull();
    expect(within(lena).getAllByText('Slouchcat Test H264')).toHaveLength(2);
    fireEvent.error(image);
    expect(marco.querySelector('img')).toBeNull();
    expect(within(marco).getAllByText('Slouchcat Test Multi Audio')).toHaveLength(2);
    expect(container.querySelectorAll('img[alt]:not([alt=""])')).toHaveLength(0);
  });

  it('counts the streams per mode and sums the bit rates under «Serverlast»', async () => {
    mockApi(routes(() => mixed));
    renderApp('/admin/streams');
    expect(await screen.findByRole('heading', { level: 2, name: 'Serverlast' })).toBeVisible();
    expect(within(loadOf('Transcode HW')).getAllByRole('definition')[0]).toHaveTextContent('1');
    expect(within(loadOf('Transcode HW')).getByText('Grafikkarte')).toBeVisible();
    expect(within(loadOf('Transcode SW')).getAllByRole('definition')[0]).toHaveTextContent('1');
    expect(within(loadOf('Remux')).getAllByRole('definition')[0]).toHaveTextContent('1');
    expect(within(loadOf('Direct')).getAllByRole('definition')[0]).toHaveTextContent('1');
    // 4 + 2.5 + 9.8 Mbit/s; the direct stream has no bit rate.
    const bandwidth = within(loadOf('Bandbreite gesamt')).getAllByRole('definition');
    expect(bandwidth[0]).toHaveTextContent(/^16,3\sMb(it)?\/s$/);
    expect(bandwidth[1]).toHaveTextContent('Summe der Bitraten');
  });

  it('asks again every 5 s and says so in the header', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi(routes(() => streams));
    renderApp('/admin/streams');
    await screen.findByRole('article', { name: 'Slouchcat Test Multi Audio' });
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    const lists = () => calls.filter((call) => call.path === '/admin/streams').length;
    expect(lists()).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(lists()).toBe(2));
  });

  it('stops a stream after the confirmation; the tile goes, the h1 takes the focus', async () => {
    let listed = streams;
    const { calls } = mockApi(
      routes(() => listed, {
        'DELETE /admin/streams/ps-1': () => {
          listed = streams.slice(1);
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/streams');
    const user = userEvent.setup();
    const stopMarco = await screen.findByRole('button', { name: 'Stream von Marco beenden' });
    // The live region is there before the message, so even the first one is announced.
    const regions = screen.getAllByRole('status');
    await user.click(stopMarco);
    const dialog = await screen.findByRole('alertdialog', { name: 'Stream von Marco beenden?' });
    expect(dialog).toHaveTextContent('„Slouchcat Test Multi Audio“');
    await user.click(within(dialog).getByRole('button', { name: 'Stream beenden' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/streams/ps-1'),
      ).toBe(true),
    );
    const done = await screen.findByText('Stream von Marco beendet.');
    expect(regions).toContain(done.closest('[role="status"]'));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Stream von Marco beenden' })).toBeNull(),
    );
    expect(tileOf('Lena')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Streams' })).toHaveFocus(),
    );
  });

  it('shows the error when stopping fails and keeps the tile', async () => {
    mockApi(
      routes(() => streams, {
        'DELETE /admin/streams/ps-1': () => problem(404, 'not_found'),
      }),
    );
    renderApp('/admin/streams');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Stream von Marco beenden' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Stream von Marco beenden?' });
    await user.click(within(dialog).getByRole('button', { name: 'Stream beenden' }));
    expect(await screen.findByText('Nicht gefunden.')).toBeVisible();
    expect(tileOf('Marco')).toBeVisible();
  });

  it('does not poll while a confirmation is open, so its tile stays', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi(routes(() => streams));
    renderApp('/admin/streams');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(await screen.findByRole('button', { name: 'Stream von Marco beenden' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Stream von Marco beenden?' });
    const lists = () => calls.filter((call) => call.path === '/admin/streams').length;
    const before = lists();
    // The header no longer says «Live» while the poll waits.
    expect(screen.queryByText('Live, alle 5 Sekunden')).toBeNull();
    await act(() => vi.advanceTimersByTimeAsync(12_000));
    expect(lists()).toBe(before);
    expect(dialog).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    // Cancelled: the focus is back on the button that asked.
    expect(screen.getByRole('button', { name: 'Stream von Marco beenden' })).toHaveFocus();
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(lists()).toBeGreaterThan(before));
  });

  it('keeps the confirmation open when the stream ends meanwhile', async () => {
    let listed = streams;
    mockApi(routes(() => listed));
    const { queryClient } = renderApp('/admin/streams');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Stream von Marco beenden' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Stream von Marco beenden?' });
    listed = streams.slice(1);
    await act(() => queryClient.refetchQueries({ queryKey: ['admin', 'streams'] }));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Stream von Marco beenden' })).toBeNull(),
    );
    expect(dialog).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    // The button that asked is gone: the h1 takes the focus.
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Streams' })).toHaveFocus(),
    );
  });

  it('moves the focus to the h1 when the focused tile goes with the poll', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let listed = streams;
    mockApi(routes(() => listed));
    renderApp('/admin/streams');
    const stopMarco = await screen.findByRole('button', { name: 'Stream von Marco beenden' });
    stopMarco.focus();
    listed = streams.slice(1);
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Stream von Marco beenden' })).toBeNull(),
    );
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Streams' })).toHaveFocus(),
    );
  });

  it('steps between the tiles with the arrow keys', async () => {
    mockApi(routes(() => mixed));
    renderApp('/admin/streams');
    const marco = await screen.findByRole('button', { name: 'Stream von Marco beenden' });
    const lena = screen.getByRole('button', { name: 'Stream von Lena beenden' });
    const sandra = screen.getByRole('button', { name: 'Stream von Sandra beenden' });
    marco.focus();
    fireEvent.keyDown(marco, { key: 'ArrowDown' });
    expect(lena).toHaveFocus();
    fireEvent.keyDown(lena, { key: 'ArrowRight' });
    expect(sandra).toHaveFocus();
    fireEvent.keyDown(sandra, { key: 'ArrowLeft' });
    expect(lena).toHaveFocus();
    fireEvent.keyDown(lena, { key: 'ArrowUp' });
    expect(marco).toHaveFocus();
    // The first tile has no tile before it.
    fireEvent.keyDown(marco, { key: 'ArrowUp' });
    expect(marco).toHaveFocus();
  });

  it('says when nothing is playing; «Serverlast» shows zero', async () => {
    mockApi(routes(() => []));
    renderApp('/admin/streams');
    expect(await screen.findByText('Gerade läuft nichts.')).toBeVisible();
    expect(within(loadOf('Direct')).getAllByRole('definition')[0]).toHaveTextContent('0');
    expect(within(loadOf('Bandbreite gesamt')).getAllByRole('definition')[0]).toHaveTextContent(
      /^0\sMb(it)?\/s$/,
    );
    expect(within(tiles()).queryByRole('list')).toBeNull();
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/streams');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
