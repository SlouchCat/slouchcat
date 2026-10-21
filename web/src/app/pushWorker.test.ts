import { afterEach, describe, expect, it, vi, type Mock } from 'vitest';
import source from '../../public/push-sw.js?raw';

// public/push-sw.js runs inside the service worker (importScripts), outside the bundle. The test
// evaluates the file itself with a stand-in for the worker's `self` and `clients`.

const ORIGIN = 'https://slouchcat.test';

interface FakeWindow {
  url: string;
  focus: Mock<() => Promise<FakeWindow>>;
  navigate: Mock<(url: string) => Promise<unknown>>;
}

/** A subscription as the worker sees it: its server key and its JSON form. */
function fakeSubscription(endpoint: string, applicationServerKey: ArrayBuffer | null = null) {
  return {
    options: { applicationServerKey },
    toJSON: () => ({ endpoint, keys: { p256dh: `p256dh-${endpoint}`, auth: `auth-${endpoint}` } }),
  };
}

function worker(windows: FakeWindow[] = []) {
  const handlers: Record<string, (event: unknown) => void> = {};
  const subscribe = vi.fn((options: { applicationServerKey: ArrayBuffer }) =>
    Promise.resolve(
      fakeSubscription('https://push.example.test/new', options.applicationServerKey),
    ),
  );
  const self = {
    location: { origin: ORIGIN },
    registration: { showNotification: vi.fn(() => Promise.resolve()), pushManager: { subscribe } },
    addEventListener: (type: string, handler: (event: unknown) => void) => {
      handlers[type] = handler;
    },
  };
  const clients = {
    matchAll: vi.fn(() => Promise.resolve(windows)),
    openWindow: vi.fn(() => Promise.resolve(null)),
  };
  // eslint-disable-next-line @typescript-eslint/no-implied-eval -- the worker file under test
  const run = new Function('self', 'clients', source) as (self: unknown, clients: unknown) => void;
  run(self, clients);

  /** Dispatches an event and waits for what it passed to waitUntil. */
  const dispatch = async (type: string, event: object) => {
    let pending: Promise<unknown> = Promise.resolve();
    handlers[type]?.({ ...event, waitUntil: (promise: Promise<unknown>) => (pending = promise) });
    await pending;
  };
  const push = (text: string | null) =>
    dispatch('push', { data: text === null ? null : { text: () => text } });
  const click = (data: unknown) => {
    const notification = { data, close: vi.fn() };
    return dispatch('notificationclick', { notification }).then(() => notification);
  };
  const change = (event: object) => dispatch('pushsubscriptionchange', event);
  return { self, clients, push, click, change, subscribe };
}

function appWindow(url: string): FakeWindow {
  const window: FakeWindow = {
    url,
    focus: vi.fn(() => Promise.resolve(window)),
    navigate: vi.fn(() => Promise.resolve(null)),
  };
  return window;
}

describe('push service worker', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows title and text of the server message; the click opens its path', async () => {
    const { self, push } = worker();
    await push(
      JSON.stringify({ title: '[Lena] Neue Folge', body: 'S01E03 · Pilot', url: '/items/01J' }),
    );
    expect(self.registration.showNotification).toHaveBeenCalledWith('[Lena] Neue Folge', {
      body: 'S01E03 · Pilot',
      icon: '/icons/icon-192.png',
      data: { url: '/items/01J' },
    });
  });

  it('keeps only the path of an absolute link (the server may know another address)', async () => {
    const { self, push } = worker();
    await push(JSON.stringify({ title: 'T', body: 'B', url: 'https://media.example.org/x?y=1#z' }));
    expect(self.registration.showNotification).toHaveBeenCalledWith(
      'T',
      expect.objectContaining({ data: { url: '/x?y=1#z' } }),
    );
  });

  it('shows a plain text message or an empty one under the app name', async () => {
    const { self, push } = worker();
    await push('Hallo');
    expect(self.registration.showNotification).toHaveBeenLastCalledWith('Slouchcat', {
      body: 'Hallo',
      icon: '/icons/icon-192.png',
      data: { url: '/' },
    });
    await push(null);
    expect(self.registration.showNotification).toHaveBeenLastCalledWith(
      'Slouchcat',
      expect.objectContaining({ body: '', data: { url: '/' } }),
    );
  });

  it('a click focuses an open window of the app and navigates it to the link', async () => {
    const open = appWindow(`${ORIGIN}/`);
    const { clients, click } = worker([appWindow('https://other.test/'), open]);
    const notification = await click({ url: '/notifications' });
    expect(notification.close).toHaveBeenCalled();
    expect(open.focus).toHaveBeenCalled();
    expect(open.navigate).toHaveBeenCalledWith(`${ORIGIN}/notifications`);
    expect(clients.openWindow).not.toHaveBeenCalled();
  });

  it('a click opens a new window without an open one, or when navigating fails', async () => {
    const first = worker([appWindow('https://other.test/')]);
    await first.click({ url: '/items/01J' });
    expect(first.clients.openWindow).toHaveBeenCalledWith(`${ORIGIN}/items/01J`);

    const stuck = appWindow(`${ORIGIN}/`);
    stuck.navigate.mockImplementation(() => Promise.reject(new TypeError('not controlled')));
    const second = worker([stuck]);
    await second.click({ url: '/watchlist' });
    expect(second.clients.openWindow).toHaveBeenCalledWith(`${ORIGIN}/watchlist`);
  });

  it('a click without a link shows the open window as it is', async () => {
    const open = appWindow(`${ORIGIN}/`);
    const { clients, click } = worker([open]);
    await click(null);
    expect(open.focus).toHaveBeenCalled();
    expect(open.navigate).not.toHaveBeenCalled();
    expect(clients.openWindow).not.toHaveBeenCalled();
  });

  describe('a replaced subscription', () => {
    const posted = () => {
      const fetchMock = vi.fn<(url: string, init: RequestInit) => Promise<Response>>(() =>
        Promise.resolve(new Response(null, { status: 201 })),
      );
      vi.stubGlobal('fetch', fetchMock);
      return fetchMock;
    };

    it('subscribes again with the old server key and registers the new one', async () => {
      const fetchMock = posted();
      const { change, subscribe } = worker();
      const key = new Uint8Array([4, 1, 2, 3]).buffer;
      await change({ oldSubscription: fakeSubscription('https://push.example.test/old', key) });
      expect(subscribe).toHaveBeenCalledWith({ userVisibleOnly: true, applicationServerKey: key });
      expect(fetchMock).toHaveBeenCalledTimes(1);
      const [url, init] = fetchMock.mock.calls[0]!;
      expect(url).toBe('/api/v1/me/push-subscriptions');
      expect(init.method).toBe('POST');
      expect(init.credentials).toBe('same-origin');
      expect(init.headers).toMatchObject({ 'X-Requested-With': 'Slouchcat' });
      expect(JSON.parse(init.body as string)).toEqual({
        endpoint: 'https://push.example.test/new',
        keys: {
          p256dh: 'p256dh-https://push.example.test/new',
          auth: 'auth-https://push.example.test/new',
        },
      });
    });

    it('registers the subscription the browser made already', async () => {
      const fetchMock = posted();
      const { change, subscribe } = worker();
      await change({
        oldSubscription: fakeSubscription('https://push.example.test/old', new ArrayBuffer(4)),
        newSubscription: fakeSubscription('https://push.example.test/given'),
      });
      expect(subscribe).not.toHaveBeenCalled();
      expect(JSON.parse(fetchMock.mock.calls[0]?.[1].body as string)).toMatchObject({
        endpoint: 'https://push.example.test/given',
      });
    });

    it('does nothing without the old server key, and a refusal ends quietly', async () => {
      const fetchMock = posted();
      const { change, subscribe } = worker();
      await change({ oldSubscription: null });
      expect(subscribe).not.toHaveBeenCalled();
      expect(fetchMock).not.toHaveBeenCalled();

      fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));
      await expect(
        change({ newSubscription: fakeSubscription('https://push.example.test/given') }),
      ).resolves.toBeUndefined();
    });
  });
});
