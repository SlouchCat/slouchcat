import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router';
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import { useEffect } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, session, type Call, type Routes } from '@/test/api';
import { sessionQuery } from '../session';
import type { RadioEngineEvents, RadioEngineFactory } from './engine';
import type { RadioListen, RadioStation } from './priority';
import { RadioProvider } from './RadioProvider';
import { useRadio, type RadioPlayer } from './useRadio';

const station: RadioStation = {
  id: 7,
  name: 'Radio Swiss Pop',
  country: 'CH',
  languages: ['de'],
  genres: ['pop'],
  kinds: ['music'],
  moods: [],
  hls: false,
  showsTitles: true,
  favorite: false,
  recommended: false,
  state: 'ok',
};

const listen: RadioListen = {
  listenId: 'L1',
  playlistUrl: '/radio/live/L1/index.m3u8?rt=token',
  heartbeatSeconds: 30,
  startOffsetSeconds: -8,
  station,
};

/** A BroadcastChannel inside one test: every instance of a name hears the others. */
class FakeChannel {
  static open: FakeChannel[] = [];
  onmessage: ((event: { data: unknown }) => void) | null = null;
  constructor(readonly name: string) {
    FakeChannel.open.push(this);
  }
  postMessage(data: unknown) {
    for (const other of FakeChannel.open) {
      if (other !== this && other.name === this.name) other.onmessage?.({ data });
    }
  }
  close() {
    FakeChannel.open = FakeChannel.open.filter((channel) => channel !== this);
  }
}

/** The player of the provider, as the probe last saw it. */
const probe: { current: RadioPlayer | null } = { current: null };

function Probe() {
  const radio = useRadio();
  useEffect(() => {
    probe.current = radio;
  });
  return (
    <output data-testid="radio">
      {radio.state.phase}/{radio.playing ? 'playing' : 'held'}/{radio.state.notice?.kind ?? '-'}
    </output>
  );
}

const player = () => {
  if (!probe.current) throw new Error('the provider did not render');
  return probe.current;
};

const status = () => screen.getByTestId('radio').textContent;

let playSpy: ReturnType<typeof vi.spyOn>;

function setup(routes: Routes = {}) {
  const engineEvents: RadioEngineEvents[] = [];
  const destroy = vi.fn();
  const engine = vi.fn<RadioEngineFactory>((_audio, _url, events) => {
    engineEvents.push(events);
    return Promise.resolve({ playingDate: () => null, toLive: vi.fn(), destroy });
  });
  const api = mockApi({
    'GET /auth/session': () => json(session),
    'POST /radio/stations/7/listen': () => json(listen, 201),
    'POST /radio/listen/L1/heartbeat': () => json({ expiresAt: Date.now() + 60_000 }),
    'DELETE /radio/listen/L1': () => new Response(null, { status: 204 }),
    'GET /radio/stations/7/now': () => json({ live: true, entries: [] }),
    ...routes,
  });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(sessionQuery.queryKey, session);
  const rootRoute = createRootRoute({
    component: () => (
      <RadioProvider engine={engine}>
        <Probe />
        <Outlet />
      </RadioProvider>
    ),
  });
  const routeTree = rootRoute.addChildren([
    createRoute({ getParentRoute: () => rootRoute, path: '/', component: () => null }),
    createRoute({ getParentRoute: () => rootRoute, path: '/play/$itemId', component: () => null }),
  ]);
  const history = createMemoryHistory({ initialEntries: ['/'] });
  const router = createRouter({ routeTree, history });
  const view = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router as never} />
    </QueryClientProvider>,
  );
  return { ...api, engine, engineEvents, destroy, queryClient, history, view };
}

const ends = (calls: Call[]) =>
  calls.filter((call) => call.method === 'DELETE' && call.path.startsWith('/radio/listen/'));
const beats = (calls: Call[]) =>
  calls.filter((call) => call.path.endsWith('/heartbeat')).map((call) => call.body);

async function startLive(context: ReturnType<typeof setup>) {
  await screen.findByTestId('radio');
  act(() => player().play(station));
  await waitFor(() => expect(status()).toBe('live/playing/-'));
  await waitFor(() => expect(context.engine).toHaveBeenCalledTimes(1));
}

describe('RadioProvider', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.stubGlobal('BroadcastChannel', FakeChannel);
    playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined);
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => undefined);
    probe.current = null;
    FakeChannel.open = [];
  });
  afterEach(async () => {
    // The provider ends its session one task after it unmounts: that request belongs to this
    // test, not to the fetch stub of the next one.
    cleanup();
    await act(() => vi.advanceTimersByTimeAsync(10));
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('opens a listen session and attaches its playlist to the engine', async () => {
    const context = setup();
    await startLive(context);
    expect(context.calls.some((call) => call.path === '/radio/stations/7/listen')).toBe(true);
    expect(context.engine.mock.calls[0]?.[1]).toBe('/api/v1/radio/live/L1/index.m3u8?rt=token');
    await waitFor(() => expect(playSpy).toHaveBeenCalled());
  });

  it('sends a heartbeat every 30 s, also paused, and one when the listener pauses', async () => {
    const context = setup();
    await startLive(context);
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(beats(context.calls)).toEqual([{ playing: true }]);
    act(() => player().toggle());
    await waitFor(() =>
      expect(beats(context.calls)).toEqual([{ playing: true }, { playing: false }]),
    );
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(beats(context.calls).at(-1)).toEqual({ playing: false });
    expect(status()).toBe('live/held/-');
  });

  it('stops with the watch limit notice when the heartbeat answers 403', async () => {
    const context = setup({
      'POST /radio/listen/L1/heartbeat': () =>
        problem(403, 'watch_limit_reached', { extensions: { limitMinutes: 60, usedMinutes: 61 } }),
    });
    await startLive(context);
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    await waitFor(() => expect(status()).toBe('idle/held/watchLimit'));
    expect(player().state.notice).toEqual({
      kind: 'watchLimit',
      limitMinutes: 60,
      usedMinutes: 61,
    });
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(ends(context.calls)).toHaveLength(1);
  });

  it('stops with radio_unreachable when the heartbeat answers 410', async () => {
    const context = setup({
      'POST /radio/listen/L1/heartbeat': () => problem(410, 'radio_unreachable'),
    });
    await startLive(context);
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    await waitFor(() => expect(status()).toBe('idle/held/problem'));
    expect(player().state.notice).toEqual({ kind: 'problem', code: 'radio_unreachable' });
  });

  it.each([401, 404])(
    'stops cleanly with «ended» when the heartbeat answers %i',
    async (httpStatus) => {
      const context = setup({
        'POST /radio/listen/L1/heartbeat': () =>
          problem(httpStatus, httpStatus === 401 ? 'unauthorized' : 'not_found'),
      });
      await startLive(context);
      await act(() => vi.advanceTimersByTimeAsync(30_000));
      await waitFor(() => expect(status()).toBe('idle/held/ended'));
      expect(player().state.listen).toBeNull();
      expect(player().state.station?.id).toBe(station.id);
      // The stream is detached and the session ended once; no further beat follows.
      expect(context.destroy).toHaveBeenCalled();
      await act(() => vi.advanceTimersByTimeAsync(10));
      expect(ends(context.calls)).toHaveLength(1);
      await act(() => vi.advanceTimersByTimeAsync(60_000));
      expect(beats(context.calls)).toHaveLength(1);
    },
  );

  it('drops the late answer of station A after a quick switch to station B', async () => {
    const other: RadioStation = { ...station, id: 8, name: 'Kanal K' };
    const otherListen: RadioListen = {
      ...listen,
      listenId: 'L2',
      playlistUrl: '/radio/live/L2/index.m3u8?rt=token',
      station: other,
    };
    let answerA: (response: Response) => void = () => {};
    const context = setup({
      'POST /radio/stations/7/listen': () =>
        new Promise<Response>((resolve) => {
          answerA = resolve;
        }),
      'POST /radio/stations/8/listen': () => json(otherListen, 201),
      'POST /radio/listen/L2/heartbeat': () => json({ expiresAt: Date.now() + 60_000 }),
      'GET /radio/stations/8/now': () => json({ live: true, entries: [] }),
    });
    await screen.findByTestId('radio');
    act(() => player().play(station));
    await waitFor(() => expect(status()).toBe('starting/held/-'));
    act(() => player().play(other));
    await waitFor(() => expect(status()).toBe('live/playing/-'));
    expect(player().state.station?.id).toBe(other.id);

    // A's answer comes after B started: its session is ended at once, B plays on.
    await act(async () => {
      answerA(json(listen, 201));
      await vi.advanceTimersByTimeAsync(10);
    });
    await waitFor(() =>
      expect(ends(context.calls)).toEqual([expect.objectContaining({ path: '/radio/listen/L1' })]),
    );
    expect(status()).toBe('live/playing/-');
    expect(player().state.listen?.listenId).toBe('L2');
    expect(context.engine).toHaveBeenCalledTimes(1);
    expect(context.engine.mock.calls[0]?.[1]).toBe('/api/v1/radio/live/L2/index.m3u8?rt=token');
  });

  it('pauses a station that still starts: it opens held', async () => {
    let answer: (response: Response) => void = () => {};
    const context = setup({
      'POST /radio/stations/7/listen': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
    });
    await screen.findByTestId('radio');
    act(() => player().play(station));
    await waitFor(() => expect(status()).toBe('starting/held/-'));
    act(() => player().toggle());
    await waitFor(() => expect(player().state.holds).toEqual(['user']));
    await act(async () => {
      answer(json(listen, 201));
      await vi.advanceTimersByTimeAsync(10);
    });
    await waitFor(() => expect(status()).toBe('live/held/-'));
    expect(player().state.holds).toEqual(['user']);
    // Pressed again, it plays.
    act(() => player().toggle());
    await waitFor(() => expect(status()).toBe('live/playing/-'));
    expect(ends(context.calls)).toEqual([]);
  });

  it('shows the code of a start the server refused', async () => {
    setup({
      'POST /radio/stations/7/listen': () =>
        problem(409, 'radio_capacity', { extensions: { running: 10, max: 10 } }),
    });
    await screen.findByTestId('radio');
    act(() => player().play(station));
    await waitFor(() => expect(status()).toBe('idle/held/problem'));
    expect(player().state.notice).toEqual({ kind: 'problem', code: 'radio_capacity' });
    expect(player().state.station?.id).toBe(station.id);
  });

  it('ends the session with a keepalive DELETE when the provider unmounts', async () => {
    const context = setup();
    await startLive(context);
    context.view.unmount();
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(ends(context.calls)).toEqual([
      expect.objectContaining({ path: '/radio/listen/L1', keepalive: true }),
    ]);
    expect(context.destroy).toHaveBeenCalled();
  });

  it('ends the session on pagehide, but not for the back-forward cache', async () => {
    const context = setup();
    await startLive(context);
    act(() => {
      window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }));
    });
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(ends(context.calls)).toEqual([]);
    act(() => {
      window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: false }));
    });
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(ends(context.calls)).toEqual([
      expect.objectContaining({ path: '/radio/listen/L1', keepalive: true }),
    ]);
  });

  it('pauses when another tab claims the radio and claims it when it plays', async () => {
    const context = setup();
    const other = new FakeChannel('slouchcat-radio');
    const heard: unknown[] = [];
    other.onmessage = (event) => heard.push(event.data);
    await startLive(context);
    expect(heard).toEqual([expect.objectContaining({ type: 'claim' })]);
    act(() => other.postMessage({ type: 'claim', tabId: 'other-tab' }));
    await waitFor(() => expect(status()).toBe('live/held/-'));
    expect(player().state.holds).toEqual(['tab']);
    act(() => player().toggle());
    await waitFor(() => expect(status()).toBe('live/playing/-'));
    expect(heard).toHaveLength(2);
  });

  it('stops when a title starts playing (/play/$itemId)', async () => {
    const context = setup();
    await startLive(context);
    act(() => context.history.push('/play/01HZ'));
    await waitFor(() => expect(status()).toBe('idle/held/-'));
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(ends(context.calls)).toHaveLength(1);
  });

  it('stops on a profile change and on logout', async () => {
    const context = setup();
    await startLive(context);
    const other = { ...session, profile: { ...session.profile!, id: 2, name: 'Lena' } };
    act(() => {
      context.queryClient.setQueryData(sessionQuery.queryKey, other);
    });
    await waitFor(() => expect(status()).toBe('idle/held/-'));

    act(() => player().play(station));
    await waitFor(() => expect(status()).toBe('live/playing/-'));
    act(() => {
      context.queryClient.setQueryData(sessionQuery.queryKey, null);
    });
    await waitFor(() => expect(status()).toBe('idle/held/-'));
  });

  it('pauses for a trailer and plays on after it closed', async () => {
    const context = setup();
    await startLive(context);
    let release = () => {};
    act(() => {
      release = player().hold('trailer');
    });
    await waitFor(() => expect(status()).toBe('live/held/-'));
    act(() => release());
    await waitFor(() => expect(status()).toBe('live/playing/-'));
    expect(ends(context.calls)).toEqual([]);
  });

  it('stops when the engine reports the session revoked', async () => {
    const context = setup();
    await startLive(context);
    act(() => context.engineEvents[0]?.onRevoked());
    await waitFor(() => expect(status()).toBe('idle/held/ended'));
  });
});
