import type { AdminDevice } from '@slouchcat/api-client';
import { focusManager } from '@tanstack/react-query';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';
import { adminShell, renderAdminPage } from '@/test/admin';

// W-7 devices for the main account's primary profile (concept 12, variant C): tabs «Geräte» and
// «Fähigkeiten» in the band, one section per device type in both, a device without a type under
// «Unbekannt». The server lists and enforces; the page shows the answers and sends "Vergessen".

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
  {
    id: 'dev-3',
    name: 'Firefox 140 · Windows',
    type: 'desktop',
    ownerEmail: 'gast@example.test',
    ownerProfile: 'Lena',
    createdAt: '2026-10-18T10:00:00Z',
    lastSeenAt: '2026-10-18T20:00:00Z',
    capabilities: { video: ['h264'], audio: ['aac'], hls: 'mse', hdr: false },
    activeStreams: 0,
  },
];

const rowOf = (name: string) => screen.getByRole('rowheader', { name }).closest('tr')!;
const cellsOf = (row: HTMLElement) =>
  within(row)
    .getAllByRole('cell')
    .map((cell) => cell.textContent);
const headersOf = (table: HTMLElement) =>
  within(table)
    .getAllByRole('columnheader')
    .map((cell) => cell.textContent);

describe('admin devices (W-7)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists the devices by type with owner, last contact and streams', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json(devices) });
    renderApp('/admin/devices');
    expect(await screen.findByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
    const tabs = await screen.findByRole('tablist', { name: 'Ansicht der Geräte' });
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual(['Geräte 33 Geräte', 'Fähigkeiten 22 Geräte mit Fähigkeitsprofil']);
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      'Geräte 3 Geräte',
    );
    // One section per type, in the order of the mockup; the device without a type is «Unbekannt».
    expect(
      screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent),
    ).toEqual(['Computer 22 Geräte', 'Unbekannt 11 Gerät']);
    expect(screen.getByText('1 Stream')).toBeVisible();

    const computers = screen.getByRole('table', { name: 'Geräte: Computer' });
    expect(headersOf(computers)).toEqual([
      'Gerät',
      'Eigentümer',
      'Letzter Kontakt',
      'Streams',
      'Aktionen',
    ]);
    expect(
      within(computers)
        .getAllByRole('rowheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Chrome 156 · Linux', 'Firefox 140 · Windows']);
    expect(cellsOf(rowOf('Chrome 156 · Linux'))).toEqual([
      'owner@example.test · Marco',
      expect.stringMatching(/2026/) as string,
      '1',
      'Vergessen',
    ]);
    const unknown = screen.getByRole('table', { name: 'Geräte: Unbekannt' });
    const unnamed = within(unknown).getByRole('rowheader', { name: 'Unbekanntes Gerät' });
    expect(within(unnamed.closest('tr')!).getByText('Noch keine Wiedergabe')).toBeVisible();
  });

  it('shows the capability profiles in the tab «Fähigkeiten», kept in ?tab=', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json(devices) });
    const { router } = renderApp('/admin/devices');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('tab', { name: /^Fähigkeiten/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'capabilities' }));
    const computers = screen.getByRole('table', { name: 'Fähigkeiten: Computer' });
    expect(headersOf(computers)).toEqual([
      'Gerät',
      'Container',
      'Video',
      'Audio',
      'Max. Auflösung',
      'HDR',
    ]);
    expect(cellsOf(rowOf('Chrome 156 · Linux'))).toEqual([
      'MP4, WEBM',
      'H264, HEVC',
      'AAC, OPUS',
      '3840 × 2160',
      'Ja',
    ]);
    // What a browser did not report is a dash on screen and a word for screen readers.
    const firefox = cellsOf(rowOf('Firefox 140 · Windows'));
    expect(firefox).toEqual(['–unbekannt', 'H264', 'AAC', '–unbekannt', 'Nein']);
    // A device that never reported a profile says so in its name, not only in dashes.
    const unnamed = cellsOf(rowOf('Unbekanntes Gerät, noch kein Fähigkeitsprofil'));
    expect(unnamed).toEqual(['–unbekannt', '–unbekannt', '–unbekannt', '–unbekannt', '–unbekannt']);
    // No "Vergessen" in this tab.
    expect(screen.queryByRole('button', { name: /vergessen$/ })).toBeNull();

    await user.click(screen.getByRole('tab', { name: /^Geräte/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(screen.getByRole('table', { name: 'Geräte: Computer' })).toBeVisible();
  });

  it('opens a linked tab; an unknown ?tab= shows «Geräte» and leaves the address', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json(devices) });
    const linked = renderApp('/admin/devices?tab=capabilities');
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(
      'Fähigkeiten 2 Geräte mit Fähigkeitsprofil',
    );
    linked.unmount();

    const { router } = renderApp('/admin/devices?tab=nonsense');
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(
      'Geräte 3 Geräte',
    );
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    // Replaced, not pushed: going back does not return to the unknown tab.
    expect(router.history.length).toBe(1);
  });

  it('folds a type away by its heading; it stays folded in the other tab', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json(devices) });
    renderApp('/admin/devices');
    const user = userEvent.setup();
    const computers = await screen.findByRole('button', { name: 'Computer 2 Geräte' });
    expect(computers).toHaveAttribute('aria-expanded', 'true');
    computers.focus();
    await user.keyboard('{Enter}');
    expect(computers).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('table', { name: 'Geräte: Computer' })).toBeNull();

    await user.click(screen.getByRole('tab', { name: /^Fähigkeiten/ }));
    expect(screen.getByRole('button', { name: 'Computer 2 Geräte' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(screen.getByRole('button', { name: 'Unbekannt 1 Gerät' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
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
    const second = screen.getByRole('button', { name: 'Firefox 140 · Windows vergessen' });
    first.focus();
    fireEvent.keyDown(first, { key: 'ArrowDown' });
    expect(second).toHaveFocus();
    fireEvent.keyDown(second, { key: 'ArrowUp' });
    expect(first).toHaveFocus();

    await user.click(first);
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Gerät „Chrome 156 · Linux“ vergessen?',
    });
    expect(dialog).toHaveTextContent('Ist es das Gerät, das du gerade benutzt');
    await user.click(within(dialog).getByRole('button', { name: 'Gerät vergessen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/devices/dev-1'),
      ).toBe(true),
    );
    const done = await screen.findByText('Gerät „Chrome 156 · Linux“ vergessen.');
    expect(done).toBeVisible();
    // Said in the page's status region, which is there before the first message.
    expect(done.closest('[role="status"]')).not.toBeNull();
    await waitFor(() =>
      expect(screen.queryByRole('rowheader', { name: 'Chrome 156 · Linux' })).toBeNull(),
    );
    // The button went away with its row: the focus is on the page heading, not lost.
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Geräte' })).toHaveFocus(),
    );
  });

  it('cancelling the confirmation sends nothing and keeps the focus on the button', async () => {
    const { calls } = mockApi({ ...signedIn, 'GET /admin/devices': () => json(devices) });
    renderApp('/admin/devices');
    const user = userEvent.setup();
    const forget = await screen.findByRole('button', { name: 'Chrome 156 · Linux vergessen' });
    await user.click(forget);
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Gerät „Chrome 156 · Linux“ vergessen?',
    });
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);
    await waitFor(() => expect(forget).toHaveFocus());
  });

  it('an owner without a profile and a type this build does not know are not left out', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/devices': () =>
        json([
          ...devices,
          {
            id: 'dev-4',
            name: 'Uhr',
            type: 'watch' as AdminDevice['type'],
            ownerEmail: 'gast@example.test',
            createdAt: '2026-10-18T10:00:00Z',
            lastSeenAt: '2026-10-18T20:00:00Z',
            activeStreams: 0,
          },
        ]),
    });
    renderApp('/admin/devices');
    // The unknown type is listed under «Unbekannt», with the device without a type.
    const unknown = await screen.findByRole('table', { name: 'Geräte: Unbekannt' });
    expect(
      within(unknown)
        .getAllByRole('rowheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Unbekanntes Gerät', 'Uhr']);
    expect(screen.getByRole('button', { name: 'Unbekannt 2 Geräte' })).toBeVisible();
    // The missing profile is a dash on screen and a word for screen readers.
    expect(cellsOf(rowOf('Uhr'))[0]).toBe('gast@example.test · –gast@example.test · unbekannt');
  });

  it('does not refetch on window focus while the confirmation is open', async () => {
    const { calls } = mockApi({ ...signedIn, 'GET /admin/devices': () => json(devices) });
    renderApp('/admin/devices');
    const user = userEvent.setup();
    const lists = () => calls.filter((call) => call.path === '/admin/devices').length;
    const refocus = () =>
      act(() => {
        focusManager.setFocused(false);
        focusManager.setFocused(true);
      });
    try {
      await user.click(await screen.findByRole('button', { name: 'Chrome 156 · Linux vergessen' }));
      const dialog = await screen.findByRole('alertdialog');
      const before = lists();
      refocus();
      // Give a refetch the chance it must not take.
      await act(() => new Promise((resolve) => window.setTimeout(resolve, 50)));
      expect(lists()).toBe(before);
      expect(within(dialog).getByRole('button', { name: 'Gerät vergessen' })).toBeVisible();

      await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
      await waitFor(() => expect(dialog).not.toBeInTheDocument());
      refocus();
      await waitFor(() => expect(lists()).toBeGreaterThan(before));
    } finally {
      focusManager.setFocused(undefined);
    }
  });

  it('a refused "Vergessen" shows the problem; the device stays', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/devices': () => json(devices),
      'DELETE /admin/devices/dev-1': () => problem(500, 'internal'),
    });
    renderApp('/admin/devices');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Chrome 156 · Linux vergessen' }));
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Gerät vergessen' }));
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('rowheader', { name: 'Chrome 156 · Linux' })).toBeVisible();
    expect(screen.queryByText('Gerät „Chrome 156 · Linux“ vergessen.')).toBeNull();
  });

  it('the list fails to load: the error offers «Erneut versuchen»', async () => {
    let fail = true;
    mockApi({
      ...signedIn,
      'GET /admin/devices': () => (fail ? problem(500, 'internal') : json(devices)),
    });
    renderApp('/admin/devices');
    const user = userEvent.setup();
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByRole('table', { name: 'Geräte: Computer' })).toBeVisible();
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/devices');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });

  it('links devices and streams in the admin navigation', async () => {
    mockApi({ ...signedIn, ...adminShell, 'GET /admin/devices': () => json([]) });
    const { nav } = await renderAdminPage('/admin/devices');
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
