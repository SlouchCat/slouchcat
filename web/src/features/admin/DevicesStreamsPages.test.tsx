import type { AdminDevice, AdminStream } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, session, signedIn, type Routes } from '@/test/api';

// W-7 devices and W-6 streams for the main account's primary profile. The server lists and
// enforces; the pages show the answers and send "Vergessen" and "Beenden".

const devices: AdminDevice[] = [
  {
    id: 'dev-1',
    name: 'Chrome 156 · Linux',
    type: 'desktop',
    ownerEmail: 'owner@example.test',
    ownerProfile: 'Marco',
    createdAt: '2026-10-19T10:00:00Z',
    lastSeenAt: '2026-10-20T08:30:00Z',
    capabilities: {
      containers: ['mp4', 'webm'],
      video: ['h264', 'hevc'],
      audio: ['aac', 'opus'],
      hls: 'mse',
      maxWidth: 3840,
      maxHeight: 2160,
      hdr: true,
    },
    activeStreams: 1,
  },
  {
    id: 'dev-2',
    createdAt: '2026-10-19T11:00:00Z',
    lastSeenAt: '2026-10-19T11:00:00Z',
    activeStreams: 0,
  },
];

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

const rowOf = (name: string) => screen.getByRole('rowheader', { name }).closest('tr')!;

describe('admin devices (W-7)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists devices with owner, last contact and capability profile', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json(devices) });
    renderApp('/admin/devices');
    expect(await screen.findByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
    const table = await screen.findByRole('table', { name: 'Geräte' });
    expect(
      within(table)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual([
      'Gerät',
      'Typ',
      'Eigentümer',
      'Letzter Kontakt',
      'Container',
      'Video',
      'Audio',
      'Max. Auflösung',
      'HDR',
      'Streams',
      'Aktionen',
    ]);
    const chrome = rowOf('Chrome 156 · Linux');
    const cells = within(chrome)
      .getAllByRole('cell')
      .map((cell) => cell.textContent);
    expect(cells.slice(0, 8)).toEqual([
      'Computer',
      'owner@example.test · Marco',
      expect.stringMatching(/2026/) as string,
      'MP4, WEBM',
      'H264, HEVC',
      'AAC, OPUS',
      '3840 × 2160',
      'Ja',
    ]);
    expect(cells[8]).toBe('1');
    const unknown = rowOf('Unbekanntes Gerät');
    expect(within(unknown).getByText('Noch keine Wiedergabe')).toBeVisible();
  });

  it('forgets a device after the confirmation; ↑/↓ move between the rows', async () => {
    let listed = devices;
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/devices': () => json(listed),
      'DELETE /admin/devices/dev-1': () => {
        listed = devices.slice(1);
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/devices');
    const user = userEvent.setup();
    const first = await screen.findByRole('button', { name: 'Chrome 156 · Linux vergessen' });
    const second = screen.getByRole('button', { name: 'Unbekanntes Gerät vergessen' });
    first.focus();
    fireEvent.keyDown(first, { key: 'ArrowDown' });
    expect(second).toHaveFocus();
    fireEvent.keyDown(second, { key: 'ArrowUp' });
    expect(first).toHaveFocus();

    await user.click(first);
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Gerät „Chrome 156 · Linux“ vergessen?',
    });
    await user.click(within(dialog).getByRole('button', { name: 'Gerät vergessen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/devices/dev-1'),
      ).toBe(true),
    );
    expect(dialog).toHaveTextContent('Ist es das Gerät, das du gerade benutzt');
    expect(await screen.findByText('Gerät „Chrome 156 · Linux“ vergessen.')).toBeVisible();
    await waitFor(() =>
      expect(screen.queryByRole('rowheader', { name: 'Chrome 156 · Linux' })).toBeNull(),
    );
    // The button went away with its row: the focus is on the page heading, not lost.
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Geräte' }).parentElement).toHaveFocus(),
    );
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/devices');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });

  it('links devices and streams in the admin navigation', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json([]) });
    renderApp('/admin/devices');
    const nav = await screen.findByRole('navigation', { name: 'Verwaltung' });
    expect(within(nav).getByRole('link', { name: 'Geräte' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(nav).getByRole('link', { name: 'Streams' })).toHaveAttribute(
      'href',
      '/admin/streams',
    );
    expect(await screen.findByText('Noch keine Geräte.')).toBeVisible();
  });
});

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

  it('lists the running streams with profile, device, title, quality and mode', async () => {
    mockApi(routes(() => streams));
    renderApp('/admin/streams');
    const table = await screen.findByRole('table', { name: 'Laufende Streams' });
    const marco = within(table)
      .getByRole('rowheader', { name: /^Marco/ })
      .closest('tr')!;
    expect(within(marco).getByText('owner@example.test')).toBeVisible();
    expect(within(marco).getByText('Chrome 156 · Linux')).toBeVisible();
    expect(within(marco).getByText('Slouchcat Test Multi Audio')).toBeVisible();
    // The unit comes from Intl ("Mb/s" or "Mbit/s" depending on the ICU data).
    expect(within(marco).getByText(/^720p · 4\sMb(it)?\/s$/)).toBeVisible();
    expect(within(marco).getByText('Transcode (HW)')).toBeVisible();
    expect(within(marco).getByText('1:05 von 10:00')).toBeVisible();
    const lena = within(table).getByRole('rowheader', { name: /^Lena/ }).closest('tr')!;
    expect(within(lena).getByText('Unbekanntes Gerät')).toBeVisible();
    expect(within(lena).getByText('Original')).toBeVisible();
    expect(within(lena).getByText('Direct')).toBeVisible();
  });

  it('asks again every 5 s', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi(routes(() => streams));
    renderApp('/admin/streams');
    await screen.findByRole('table', { name: 'Laufende Streams' });
    const lists = () => calls.filter((call) => call.path === '/admin/streams').length;
    expect(lists()).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(lists()).toBe(2));
  });

  it('stops a stream after the confirmation; the row goes away', async () => {
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
    await user.click(await screen.findByRole('button', { name: 'Stream von Marco beenden' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Stream von Marco beenden?' });
    expect(dialog).toHaveTextContent('„Slouchcat Test Multi Audio“');
    await user.click(within(dialog).getByRole('button', { name: 'Stream beenden' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/streams/ps-1'),
      ).toBe(true),
    );
    expect(await screen.findByText('Stream von Marco beendet.')).toBeVisible();
    await waitFor(() => expect(screen.queryByRole('rowheader', { name: /^Marco/ })).toBeNull());
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { level: 1, name: 'Streams' }).parentElement,
      ).toHaveFocus(),
    );
  });

  it('does not poll while a confirmation is open, so its row stays', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi(routes(() => streams));
    renderApp('/admin/streams');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(await screen.findByRole('button', { name: 'Stream von Marco beenden' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Stream von Marco beenden?' });
    const lists = () => calls.filter((call) => call.path === '/admin/streams').length;
    const before = lists();
    await act(() => vi.advanceTimersByTimeAsync(12_000));
    expect(lists()).toBe(before);
    expect(dialog).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(lists()).toBeGreaterThan(before));
  });

  it('says when nothing is playing', async () => {
    mockApi(routes(() => []));
    renderApp('/admin/streams');
    expect(await screen.findByText('Gerade läuft nichts.')).toBeVisible();
  });
});
