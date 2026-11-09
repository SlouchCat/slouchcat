import type { AdminStream, components } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// W-6 streams for the main account's primary profile (concept 12, mockup Str-D2; concept 18,
// 17.3): one list «Läuft gerade» of playbacks and radio listeners, newest first, with the kind
// filter, «Serverlast» with the streams per mode and the bandwidth. The server lists and stops;
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
    kind: 'movie',
    source: 'file',
    mode: 'transcode',
    backend: 'nvenc',
    hardware: true,
    quality: '720p',
    width: 1280,
    height: 720,
    bitrate: 4_000_000,
    startedAt: '2026-10-20T08:10:00Z',
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
    kind: 'movie',
    source: 'file',
    mode: 'direct',
    hardware: false,
    startedAt: '2026-10-20T08:00:00Z',
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
    startedAt: '2026-10-20T07:50:00Z',
    hardware: false,
    backend: 'libx264',
    bitrate: 2_500_000,
  },
  {
    ...streams[1]!,
    id: 'ps-4',
    profileName: 'Nora',
    startedAt: '2026-10-20T07:40:00Z',
    mode: 'remux',
    bitrate: 9_800_000,
  },
];

type RadioRelays = components['schemas']['RadioRelays'];

/** The radio's relays with their listeners, as `listRadioRelays` answers. */
const relays = (list: RadioRelays['relays']): RadioRelays => ({
  relays: list,
  load: {
    relays: list.length,
    maxRelays: 10,
    bufferBytes: 0,
    budgetBytes: 512_000_000,
    uploadKbps: 128,
    bufferMode: 'tmpfs',
  },
});

const swissPop: RadioRelays['relays'][number] = {
  stationId: 7,
  name: 'Radio Swiss Pop',
  codec: 'AAC',
  bitrateKbps: 128,
  transcoded: false,
  startedAt: Date.parse('2026-11-08T08:00:00Z'),
  bufferSeconds: 600,
  bufferBytes: 9_600_000,
  state: 'live',
  listeners: [
    {
      listenId: 'L1',
      profileName: 'Lena',
      accountEmail: 'gast@example.test',
      playing: true,
      since: Date.parse('2026-11-08T08:05:00Z'),
    },
    {
      listenId: 'L2',
      profileName: 'Nora',
      accountEmail: 'gast@example.test',
      playing: false,
      since: Date.parse('2026-11-08T08:06:00Z'),
    },
  ],
};

const tileOf = (profile: string) =>
  screen.getByRole('button', { name: `Stream von ${profile} beenden` }).closest('article')!;

/** The list of tiles, in its section «Läuft gerade» (the admin sidebar has lists too). */
const tiles = () => screen.getByRole('region', { name: 'Läuft gerade' });

const loadOf = (label: string) =>
  within(screen.getByRole('group', { name: 'Serverlast' }))
    .getByText(label)
    .closest('dl')!;

describe('admin streams (W-6)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  const routes = (list: () => AdminStream[], extra: Routes = {}): Routes => ({
    ...signedIn,
    'GET /admin/streams': () => json(list()),
    // The radio's listeners come from its relays (concept 16 stage 2, D-11); none by default.
    'GET /admin/radio/relays': () => json(relays([])),
    ...extra,
  });

  it('shows each stream as a tile with profile, device, title, quality, mode and position', async () => {
    mockApi(routes(() => streams));
    renderApp('/admin/streams');
    expect(await screen.findByRole('heading', { level: 1, name: 'Streams' })).toBeVisible();
    await screen.findByRole('article', { name: 'Marco: Slouchcat Test Multi Audio' });
    const list = within(tiles()).getByRole('list');
    expect(within(list).getAllByRole('listitem')).toHaveLength(2);
    const marco = screen.getByRole('article', { name: 'Marco: Slouchcat Test Multi Audio' });
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

  it('names why a stream is not played directly, in the words of the stream info (concept 14, 7.1)', async () => {
    mockApi(
      routes(() => [
        { ...streams[0]!, reason: 'keyframes_pending' },
        { ...streams[0]!, id: 'ps-3', profileName: 'Sandra', reason: 'keyframes_unavailable' },
        { ...streams[0]!, id: 'ps-4', profileName: 'Nora', reason: 'future_reason' },
        { ...streams[1]!, reason: 'container' },
      ]),
    );
    renderApp('/admin/streams');
    await screen.findByRole('article', { name: 'Marco: Slouchcat Test Multi Audio' });
    const marco = tileOf('Marco');
    expect(within(marco).getByText('Grund').closest('dt')).not.toBeNull();
    expect(within(marco).getByText('Keyframe-Index fehlt noch')).toBeVisible();
    expect(within(tileOf('Sandra')).getByText('Kein Keyframe-Index möglich')).toBeVisible();
    // A code this client does not know yet shows as it is.
    expect(within(tileOf('Nora')).getByText('future_reason')).toBeVisible();
    // Direct play has no reason line.
    expect(within(tileOf('Lena')).queryByText('Grund')).toBeNull();
    expect(within(tileOf('Lena')).queryByText('Container')).toBeNull();
  });

  it('shows the backdrop across, and the title on the fallback without one or when it fails', async () => {
    mockApi(routes(() => streams));
    const { container } = renderApp('/admin/streams');
    const marco = await screen.findByRole('article', { name: 'Marco: Slouchcat Test Multi Audio' });
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
    // The direct stream reports no bit rate: the note says the sum covers the known ones.
    expect(bandwidth[1]).toHaveTextContent('Summe der bekannten Bitraten');
  });

  it('names the plain sum when every stream reports its bit rate', async () => {
    mockApi(routes(() => [streams[0]!]));
    renderApp('/admin/streams');
    expect(await screen.findByText('Summe der Bitraten')).toBeVisible();
    expect(screen.queryByText('Summe der bekannten Bitraten')).toBeNull();
  });

  it('marks the list busy while it loads first', async () => {
    let release: (response: Response) => void = () => undefined;
    mockApi({
      ...signedIn,
      'GET /admin/streams': () =>
        new Promise<Response>((resolve) => {
          release = resolve;
        }),
    });
    renderApp('/admin/streams');
    const region = await screen.findByRole('region', { name: 'Läuft gerade' });
    expect(region).toHaveAttribute('aria-busy', 'true');
    release(json(streams));
    await waitFor(() => expect(region).toHaveAttribute('aria-busy', 'false'));
    expect(within(region).getAllByRole('article')).toHaveLength(2);
  });

  it('asks again every 5 s and says so in the header', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi(routes(() => streams));
    renderApp('/admin/streams');
    await screen.findByRole('article', { name: 'Marco: Slouchcat Test Multi Audio' });
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

  it('steps between the tiles: ←/→ in reading order, ↑/↓ in the same column', async () => {
    // Two columns, as the grid lays them out: Marco Lena / Sandra Nora.
    const computed = window.getComputedStyle.bind(window);
    vi.spyOn(window, 'getComputedStyle').mockImplementation((element, pseudo) => {
      const style = computed(element, pseudo);
      if (!element.querySelector('[data-stream]')) return style;
      return new Proxy(style, {
        get: (target, property) => {
          if (property === 'gridTemplateColumns') return '320px 320px';
          const value: unknown = Reflect.get(target, property, target);
          return typeof value === 'function' ? (value as () => unknown).bind(target) : value;
        },
      });
    });
    mockApi(routes(() => mixed));
    renderApp('/admin/streams');
    const marco = await screen.findByRole('button', { name: 'Stream von Marco beenden' });
    const lena = screen.getByRole('button', { name: 'Stream von Lena beenden' });
    const sandra = screen.getByRole('button', { name: 'Stream von Sandra beenden' });
    const nora = screen.getByRole('button', { name: 'Stream von Nora beenden' });
    marco.focus();
    fireEvent.keyDown(marco, { key: 'ArrowDown' });
    expect(sandra).toHaveFocus();
    fireEvent.keyDown(sandra, { key: 'ArrowRight' });
    expect(nora).toHaveFocus();
    fireEvent.keyDown(nora, { key: 'ArrowUp' });
    expect(lena).toHaveFocus();
    fireEvent.keyDown(lena, { key: 'ArrowLeft' });
    expect(marco).toHaveFocus();
    fireEvent.keyDown(marco, { key: 'ArrowRight' });
    expect(lena).toHaveFocus();
    // No tile below the last row, none before the first: the focus stays.
    fireEvent.keyDown(lena, { key: 'ArrowUp' });
    expect(lena).toHaveFocus();
    fireEvent.keyDown(lena, { key: 'ArrowDown' });
    expect(nora).toHaveFocus();
    fireEvent.keyDown(nora, { key: 'ArrowDown' });
    expect(nora).toHaveFocus();
    marco.focus();
    fireEvent.keyDown(marco, { key: 'ArrowLeft' });
    expect(marco).toHaveFocus();
  });

  it('announces the same message again for a second stop', async () => {
    // Started after the first: the list is newest first.
    const second: AdminStream = {
      ...streams[0]!,
      id: 'ps-5',
      itemTitle: 'Slouchcat Test H264',
      startedAt: '2026-10-20T08:05:00Z',
    };
    let listed = [streams[0]!, second];
    mockApi(
      routes(() => listed, {
        'DELETE /admin/streams/ps-1': () => {
          listed = [second];
          return new Response(null, { status: 204 });
        },
        'DELETE /admin/streams/ps-5': () => {
          listed = [];
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/streams');
    const user = userEvent.setup();
    const stopFirst = async () => {
      await user.click(
        (await screen.findAllByRole('button', { name: 'Stream von Marco beenden' }))[0]!,
      );
      const dialog = await screen.findByRole('alertdialog', { name: 'Stream von Marco beenden?' });
      await user.click(within(dialog).getByRole('button', { name: 'Stream beenden' }));
    };
    await stopFirst();
    const first = await screen.findByText('Stream von Marco beendet.');
    await waitFor(() => expect(screen.getAllByRole('article')).toHaveLength(1));
    await stopFirst();
    await screen.findByText('Gerade läuft nichts.');
    // A new node in the live region, so screen readers say it again.
    const again = await screen.findByText('Stream von Marco beendet.');
    expect(again).not.toBe(first);
    expect(first).not.toBeInTheDocument();
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

  describe('every kind in one list (concept 18, 17.3)', () => {
    /** An extra, an audiobook of twelve parts and a podcast episode from the feed. */
    const kinds: AdminStream[] = [
      {
        ...streams[1]!,
        id: 'ps-x',
        profileName: 'Kim',
        kind: 'extra',
        itemTitle: 'Making-of',
        parentTitle: 'Oppenheimer',
        startedAt: '2026-10-20T08:20:00Z',
        backdropUrl: '/api/v1/admin/items/film/images/backdrop/w780?v=x',
      },
      {
        ...streams[1]!,
        id: 'ps-b',
        profileName: 'Kids',
        kind: 'audiobook',
        itemTitle: 'Moos und Stein',
        chapterTitle: 'Kapitel 7: Der Sturm',
        partCount: 12,
        audioCodec: 'aac',
        audioBitrate: 64_000,
        mode: 'remux',
        positionMs: 13_270_000,
        durationMs: 31_560_000,
        posterUrl: '/api/v1/admin/items/book/images/poster/w200?v=b',
        startedAt: '2026-10-20T08:30:00Z',
      },
      {
        ...streams[1]!,
        id: 'ps-p',
        profileName: 'Lena',
        kind: 'podcast_episode',
        source: 'feed',
        itemTitle: 'Folge vom 9. November',
        parentTitle: 'Apropos',
        audioCodec: 'mp3',
        posterUrl: '/api/v1/admin/items/pod/images/poster/w200?v=p',
        startedAt: '2026-10-20T08:40:00Z',
      },
    ];
    const radio = relays([
      {
        ...swissPop,
        listeners: [{ ...swissPop.listeners[0]!, since: Date.parse('2026-10-20T08:50:00Z') }],
      },
    ]);
    const filterButton = (name: RegExp) =>
      within(screen.getByRole('toolbar', { name: 'Nach Art filtern' })).getByRole('button', {
        name,
      });

    it('names the kind of each tile, newest first across playbacks and the radio', async () => {
      mockApi(
        routes(() => [...streams, ...kinds], { 'GET /admin/radio/relays': () => json(radio) }),
      );
      renderApp('/admin/streams');
      await screen.findByRole('article', { name: 'Lena: Radio Swiss Pop' });
      expect(
        within(tiles())
          .getAllByRole('article')
          .map((tile) => tile.getAttribute('aria-label')),
      ).toEqual([
        'Lena: Radio Swiss Pop',
        'Lena: Apropos · Folge vom 9. November',
        'Kids: Moos und Stein · Kapitel 7: Der Sturm',
        'Kim: Oppenheimer · Making-of',
        'Marco: Slouchcat Test Multi Audio',
        'Lena: Slouchcat Test H264',
      ]);
      const badge = (name: string) =>
        within(screen.getByRole('article', { name })).getByText(
          /^(Film|Serie|Extra|Hörbuch|Podcast|Radio)$/,
          { selector: 'span' },
        );
      expect(badge('Marco: Slouchcat Test Multi Audio')).toHaveTextContent('Film');
      expect(badge('Kim: Oppenheimer · Making-of')).toHaveTextContent('Extra');
      expect(badge('Kids: Moos und Stein · Kapitel 7: Der Sturm')).toHaveTextContent('Hörbuch');
      expect(badge('Lena: Apropos · Folge vom 9. November')).toHaveTextContent('Podcast');
      expect(badge('Lena: Radio Swiss Pop')).toHaveTextContent('Radio');
    });

    it('shows an audiobook with its cover, chapter, format and the whole book', async () => {
      mockApi(routes(() => kinds));
      renderApp('/admin/streams');
      const book = await screen.findByRole('article', {
        name: 'Kids: Moos und Stein · Kapitel 7: Der Sturm',
      });
      expect(within(book).getByText('Format').closest('dt')).not.toBeNull();
      expect(within(book).getByText(/^AAC · 64\skbit\/s · 12 Teile$/)).toBeVisible();
      expect(within(book).getByText('3:41:10 von 8:46:00')).toBeVisible();
      expect(within(book).getByText('Remux')).toBeVisible();
      // The square cover: on its blurred ground and in the middle, never the backdrop.
      const images = [...book.querySelectorAll('img')];
      expect(images.length).toBeGreaterThan(0);
      for (const image of images) expect(image).toHaveAttribute('src', kinds[1]!.posterUrl);
      const podcast = screen.getByRole('article', {
        name: 'Lena: Apropos · Folge vom 9. November',
      });
      expect(within(podcast).getByText('Quelle').closest('dt')).not.toBeNull();
      expect(within(podcast).getByText('Feed')).toBeVisible();
      expect(within(podcast).getByText(/^MP3$/)).toBeVisible();
      const extra = screen.getByRole('article', { name: 'Kim: Oppenheimer · Making-of' });
      expect(extra.querySelector('img')).toHaveAttribute('src', kinds[0]!.backdropUrl);
    });

    it('filters by kind with counts; the choice holds across the poll, also when it runs out', async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      let listed = [...streams, ...kinds];
      mockApi(routes(() => listed, { 'GET /admin/radio/relays': () => json(radio) }));
      renderApp('/admin/streams');
      const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
      await screen.findByRole('article', { name: 'Lena: Radio Swiss Pop' });
      expect(filterButton(/^Alle/)).toHaveTextContent('Alle 6');
      expect(filterButton(/^Video/)).toHaveTextContent('Video 3');
      expect(filterButton(/^Hörbücher/)).toHaveTextContent('Hörbücher 1');
      expect(filterButton(/^Podcasts/)).toHaveTextContent('Podcasts 1');
      expect(filterButton(/^Radio/)).toHaveTextContent('Radio 1');
      expect(filterButton(/^Alle/)).toHaveAttribute('aria-pressed', 'true');
      await user.click(filterButton(/^Video/));
      expect(filterButton(/^Video/)).toHaveAttribute('aria-pressed', 'true');
      expect(filterButton(/^Alle/)).toHaveAttribute('aria-pressed', 'false');
      expect(within(tiles()).getAllByRole('article')).toHaveLength(3);
      // Pressed again, it stays.
      await user.click(filterButton(/^Video/));
      expect(filterButton(/^Video/)).toHaveAttribute('aria-pressed', 'true');
      await user.click(filterButton(/^Podcasts/));
      expect(
        within(tiles())
          .getAllByRole('article')
          .map((tile) => tile.getAttribute('aria-label')),
      ).toEqual(['Lena: Apropos · Folge vom 9. November']);
      listed = [...streams, kinds[0]!, kinds[1]!];
      await act(() => vi.advanceTimersByTimeAsync(5000));
      expect(await screen.findByText('Gerade läuft nichts dieser Art.')).toBeVisible();
      expect(filterButton(/^Podcasts/)).toHaveAttribute('aria-pressed', 'true');
      expect(filterButton(/^Podcasts/)).toHaveTextContent('Podcasts 0');
      expect(filterButton(/^Alle/)).toHaveTextContent('Alle 5');
      expect(screen.queryByText('Gerade läuft nichts.')).toBeNull();
    });

    it('moves with ←/→ between the filter buttons; Enter presses one', async () => {
      mockApi(routes(() => streams, { 'GET /admin/radio/relays': () => json(radio) }));
      renderApp('/admin/streams');
      const user = userEvent.setup();
      await screen.findByRole('article', { name: 'Lena: Radio Swiss Pop' });
      filterButton(/^Alle/).focus();
      await user.keyboard('{ArrowRight}');
      expect(filterButton(/^Video/)).toHaveFocus();
      await user.keyboard('{ArrowRight}');
      expect(filterButton(/^Hörbücher/)).toHaveFocus();
      await user.keyboard('{ArrowLeft}');
      expect(filterButton(/^Video/)).toHaveFocus();
      await user.keyboard('{Enter}');
      expect(filterButton(/^Video/)).toHaveAttribute('aria-pressed', 'true');
      // Tab out of the toolbar into the tiles needs the browser's own Tab: e2e checks it.
    });

    it('keeps the playbacks when the radio fails, with a notice and «–» for its counts', async () => {
      mockApi(routes(() => kinds, { 'GET /admin/radio/relays': () => problem(500, 'internal') }));
      renderApp('/admin/streams');
      await screen.findByRole('article', { name: 'Kim: Oppenheimer · Making-of' });
      expect(
        screen.getByText('Radiohörer fehlen in der Liste, bis die Anfrage wieder gelingt.'),
      ).toBeVisible();
      expect(within(tiles()).getAllByRole('article')).toHaveLength(3);
      expect(filterButton(/^Radio/)).toHaveTextContent('Radio –');
      expect(filterButton(/^Alle/)).toHaveTextContent('Alle –');
      expect(filterButton(/^Video/)).toHaveTextContent('Video 1');
      expect(within(loadOf('Radio')).getAllByRole('definition')[0]).toHaveTextContent('–');
    });

    it('keeps the radio when the playbacks fail', async () => {
      mockApi({
        ...signedIn,
        'GET /admin/streams': () => problem(500, 'internal'),
        'GET /admin/radio/relays': () => json(radio),
      });
      renderApp('/admin/streams');
      await screen.findByRole('article', { name: 'Lena: Radio Swiss Pop' });
      expect(
        screen.getByText('Wiedergaben fehlen in der Liste, bis die Anfrage wieder gelingt.'),
      ).toBeVisible();
      expect(filterButton(/^Video/)).toHaveTextContent('Video –');
      expect(filterButton(/^Radio/)).toHaveTextContent('Radio 1');
      // The kinds of the failed half are unknown: no text claims they are empty.
      const user = userEvent.setup();
      await user.click(filterButton(/^Video/));
      expect(screen.queryByText('Gerade läuft nichts dieser Art.')).toBeNull();
    });

    it('names the device of a listener and a listener whose relay failed', async () => {
      mockApi(
        routes(() => [], {
          'GET /admin/radio/relays': () =>
            json(
              relays([
                {
                  ...swissPop,
                  state: 'failed',
                  logo: { url: '/api/v1/admin/radio/images/7/logo/w256?v=l' },
                  listeners: [
                    { ...swissPop.listeners[0]!, device: { id: 'd', name: 'Küche (iPhone)' } },
                  ],
                },
              ]),
            ),
        }),
      );
      renderApp('/admin/streams');
      const lena = await screen.findByRole('article', { name: 'Lena: Radio Swiss Pop' });
      expect(within(lena).getByText('Küche (iPhone)')).toBeVisible();
      expect(within(lena).getByText('Relais gestört')).toBeVisible();
      expect(within(lena).queryByText('Hört')).toBeNull();
      expect(lena.querySelector('img')).toHaveAttribute(
        'src',
        '/api/v1/admin/radio/images/7/logo/w256?v=l',
      );
    });
  });

  describe('radio listeners (concept 16 stage 2, D-11)', () => {
    it('shows a tile per listener in the one list and counts them under «Serverlast»', async () => {
      mockApi(routes(() => streams, { 'GET /admin/radio/relays': () => json(relays([swissPop])) }));
      renderApp('/admin/streams');
      const lena = await within(
        await screen.findByRole('region', { name: 'Läuft gerade' }),
      ).findByRole('article', { name: 'Lena: Radio Swiss Pop' });
      expect(within(lena).getByText('gast@example.test')).toBeVisible();
      expect(within(lena).getByText(/AAC · 128\skbit\/s · umgepackt/)).toBeVisible();
      expect(within(lena).getByText('2 Hörer dieses Senders')).toBeVisible();
      expect(within(lena).getByText('Hört')).toBeVisible();
      expect(within(lena).getByText('Radio', { selector: 'span' })).toBeVisible();
      expect(
        within(tiles()).getByRole('article', { name: 'Nora: Radio Swiss Pop' }),
      ).toHaveTextContent('Pausiert');
      expect(screen.queryByRole('region', { name: 'Radio' })).toBeNull();
      expect(within(loadOf('Radio')).getAllByRole('definition')[0]).toHaveTextContent('2');
    });

    it('stops a listener after the confirmation through the radio', async () => {
      let listed = [swissPop];
      const { calls } = mockApi(
        routes(() => [], {
          'GET /admin/radio/relays': () => json(relays(listed)),
          'DELETE /admin/radio/listeners/L1': () => {
            listed = [{ ...swissPop, listeners: swissPop.listeners.slice(1) }];
            return new Response(null, { status: 204 });
          },
        }),
      );
      renderApp('/admin/streams');
      const user = userEvent.setup();
      await user.click(await screen.findByRole('button', { name: 'Radio von Lena beenden' }));
      const dialog = await screen.findByRole('alertdialog', { name: 'Radio von Lena beenden?' });
      expect(dialog).toHaveTextContent('Radio Swiss Pop');
      await user.click(within(dialog).getByRole('button', { name: 'Radio beenden' }));
      expect(await screen.findByText('Radio von Lena beendet.')).toBeVisible();
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/radio/listeners/L1'),
      ).toBe(true);
      await waitFor(() =>
        expect(screen.queryByRole('article', { name: 'Lena: Radio Swiss Pop' })).toBeNull(),
      );
      await waitFor(() =>
        expect(screen.getByRole('heading', { level: 1, name: 'Streams' })).toHaveFocus(),
      );
      // The page says nothing plays only when no radio listener is left either.
      expect(screen.queryByText('Gerade läuft nichts.')).toBeNull();
    });

    it('steps with the arrow keys between the radio tiles too', async () => {
      mockApi(routes(() => [], { 'GET /admin/radio/relays': () => json(relays([swissPop])) }));
      renderApp('/admin/streams');
      // Nora started listening later: her tile comes first.
      const lena = await screen.findByRole('button', { name: 'Radio von Lena beenden' });
      const nora = screen.getByRole('button', { name: 'Radio von Nora beenden' });
      nora.focus();
      fireEvent.keyDown(nora, { key: 'ArrowRight' });
      expect(lena).toHaveFocus();
      fireEvent.keyDown(lena, { key: 'ArrowLeft' });
      expect(nora).toHaveFocus();
    });
  });
});
