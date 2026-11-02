import type { AdminStream } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, signedIn, type Routes } from '@/test/api';

// W-6 streams for the main account's primary profile. The server lists and enforces; the page
// shows the answers and sends "Beenden". The devices (W-7) are in DevicesPage.test.tsx.

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
