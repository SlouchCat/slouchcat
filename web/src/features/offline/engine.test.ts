import { QueryClient } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { sessionQuery } from '@/app/session';
import { session } from '@/test/api';
import { alien } from '@/test/fixtures';
import {
  failRemovals,
  failWrites,
  readFile,
  resetOfflineFiles,
  seedFile,
  storedFiles,
} from '@/test/offlineFiles';
import {
  downloadIds,
  downloadState,
  heatDownload,
  manifest,
  offlineProfile,
  rules,
  storedDownload,
  storedProfile,
} from '@/test/offlineFixtures';
import {
  getMeta,
  putDownloads,
  putProfiles,
  queueEvents,
  queuedEvents,
  resetOfflineStore,
  setMeta,
  storedDownloads,
  storedProfiles,
} from '@/test/offlineStore';
import { removeDownload } from './actions';
import { startOfflineEngine } from './engine';
import type { OfflineEvent, OfflineSync } from './types';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));

const MEDIA = Uint8Array.from({ length: 9000 }, (_, index) => index % 251);
// ftyp at bytes 4..8, as a real init segment has it.
MEDIA.set(new TextEncoder().encode('ftyp'), 4);

interface Seen {
  method: string;
  path: string;
  range: string | null;
  ifRange: string | null;
  body: unknown;
}

/** A fetch stub for the API and the download file (Request or URL, like the app calls it). */
function server(routes: Record<string, (seen: Seen) => Response | Promise<Response>>) {
  const seen: Seen[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const request =
      input instanceof Request
        ? input
        : new Request(new URL(String(input), 'http://localhost'), init);
    const url = new URL(request.url);
    const text = request.method === 'GET' ? '' : await request.clone().text();
    const entry: Seen = {
      method: request.method,
      path: url.pathname.replace(/^\/api\/v1/, ''),
      range: request.headers.get('Range'),
      ifRange: request.headers.get('If-Range'),
      body: text ? (JSON.parse(text) as unknown) : null,
    };
    seen.push(entry);
    const key = `${entry.method} ${entry.path}`;
    const route = Object.entries(routes).find(([pattern]) =>
      new RegExp(`^${pattern.replace(/\*/g, '[^/]+')}$`).test(key),
    );
    if (!route) return Response.json({ code: 'not_found' }, { status: 404 });
    return route[1](entry);
  });
  vi.stubGlobal('fetch', fetchMock);
  return seen;
}

const syncAnswer = (overrides: Partial<OfflineSync> = {}): OfflineSync => ({
  serverTime: new Date().toISOString(),
  downloads: [],
  revoked: [],
  profiles: [offlineProfile()],
  rules,
  ...overrides,
});

const fileRoute = ({ range }: Seen) => {
  const match = /^bytes=(\d+)-(\d+)$/.exec(range ?? '');
  if (!match) return new Response(MEDIA, { status: 200 });
  const start = Number(match[1]);
  const end = Number(match[2]);
  return new Response(MEDIA.slice(start, end + 1), {
    status: 206,
    headers: { 'Content-Range': `bytes ${start}-${end}/${MEDIA.length}`, ETag: '"alien-1"' },
  });
};

const isFile = (entry: Seen) => entry.path.endsWith('/file');
const isSync = (entry: Seen) => entry.path === '/offline/sync';
const pause = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

type Route = (seen: Seen) => Response | Promise<Response>;

/** Routes of a sync that lists Alien as ready, with everything its fetch needs. */
const readyAlien = (extra: Record<string, Route> = {}): Record<string, Route> => ({
  'POST /offline/sync': () => Response.json(syncAnswer({ downloads: [downloadState()] })),
  'GET /downloads/*/manifest': () => Response.json(manifest()),
  'GET /downloads/*/file': fileRoute,
  'GET /downloads/*/subtitles/21.vtt': () => new Response('WEBVTT\n'),
  'GET /items/*/images/poster/w500': () => new Response(new Uint8Array([1, 2, 3])),
  'POST /offline/events': () =>
    Response.json({ accepted: 0, duplicates: 0, rejected: 0, results: [] }),
  ...extra,
});

let stop: (() => void) | undefined;

function start(sessionValue: typeof session | null = session) {
  const queryClient = new QueryClient();
  queryClient.setQueryData(sessionQuery.queryKey, sessionValue);
  stop = startOfflineEngine(queryClient);
  return queryClient;
}

beforeEach(() => {
  resetOfflineStore();
  resetOfflineFiles();
});

afterEach(() => {
  stop?.();
  stop = undefined;
  vi.unstubAllGlobals();
});

describe('offline engine', () => {
  it('syncs, removes revoked downloads and fetches a ready one into storage', async () => {
    await putDownloads([heatDownload({ allowedProfileIds: [1] })]);
    seedFile(downloadIds.heat, 'media.mp4', MEDIA);
    const seen = server({
      'POST /offline/sync': ({ body }) => {
        const held = (body as { held: { id: string; complete: boolean }[] }).held;
        return Response.json(
          syncAnswer({
            revoked: held.some((entry) => entry.id === downloadIds.heat) ? [downloadIds.heat] : [],
            downloads: [downloadState()],
          }),
        );
      },
      'GET /downloads/*/manifest': () => Response.json(manifest()),
      'GET /downloads/*/file': fileRoute,
      'GET /downloads/*/subtitles/21.vtt': () =>
        new Response('WEBVTT\n\n00:01.000 --> 00:02.000\nHallo\n'),
      'GET /items/*/images/poster/w500': () => new Response(new Uint8Array([1, 2, 3])),
      'POST /offline/events': () =>
        Response.json({ accepted: 0, duplicates: 0, rejected: 0, results: [] }),
    });
    start();

    const alienDownload = () => storedDownloads().find((entry) => entry.id === downloadIds.alien);
    await vi.waitFor(() => expect(alienDownload()?.complete).toBe(true), { timeout: 3000 });
    const download = alienDownload();
    // The revoked download is gone with its files.
    expect(storedDownloads()).toHaveLength(1);
    expect(storedFiles(downloadIds.heat)).toEqual([]);
    expect(download).toMatchObject({
      id: downloadIds.alien,
      bytesWritten: 9000,
      subtitlesStored: [21],
      posterStored: true,
      item: alien,
      // Where the bytes came from, for a later resume (O1).
      fileReadyAt: '2026-10-20T10:05:00Z',
      fileEtag: '"alien-1"',
    });
    expect(
      new Uint8Array(await (await readFile(downloadIds.alien, 'media.mp4'))!.arrayBuffer()),
    ).toEqual(MEDIA);
    expect(storedFiles(downloadIds.alien).sort()).toEqual(['21.vtt', 'media.mp4', 'poster']);
    // The first sync reported the revoked one; the file came in 8-MiB ranges (here one).
    const firstSync = seen.find((entry) => entry.path === '/offline/sync');
    expect(firstSync?.body).toEqual({
      held: [{ id: downloadIds.heat, complete: true }],
      network: 'unknown',
    });
    expect(
      seen.filter((entry) => entry.path.endsWith('/file')).map((entry) => entry.range),
    ).toEqual(['bytes=0-8999']);
    // After the fetch the next sync tells the server the file is complete.
    await vi.waitFor(
      () =>
        expect(
          seen.some(
            (entry) =>
              entry.path === '/offline/sync' &&
              JSON.stringify(entry.body).includes(`"id":"${downloadIds.alien}","complete":true`),
          ),
        ).toBe(true),
      { timeout: 3000 },
    );
    expect(storedProfiles().map((profile) => profile.id)).toEqual([1]);
  });

  it('a 401 of the sync asks for the revocation list and deletes what it names (AN-4)', async () => {
    await putDownloads([storedDownload({ accountId: 5, allowedProfileIds: [9] })]);
    await putProfiles([
      storedProfile({ id: 9, accountId: 5, snapshot: offlineProfile({ id: 9 }) }),
    ]);
    const seen = server({
      'POST /offline/sync': () => Response.json({ code: 'unauthenticated' }, { status: 401 }),
      'POST /offline/revocations': ({ body }) =>
        Response.json({ revoked: (body as { ids: string[] }).ids }),
      'POST /offline/events': () => Response.json({ code: 'unauthenticated' }, { status: 401 }),
    });
    start({ ...session, account: { ...session.account, id: 5 } });
    await vi.waitFor(() => expect(storedDownloads()).toHaveLength(0), { timeout: 3000 });
    expect(seen.find((entry) => entry.path === '/offline/revocations')?.body).toEqual({
      ids: [downloadIds.alien],
    });
    expect(storedProfiles()).toEqual([]);
  });

  it('without a session it only checks the revocation list', async () => {
    await putDownloads([storedDownload()]);
    const seen = server({
      'POST /offline/revocations': () => Response.json({ revoked: [] }),
    });
    start(null);
    await vi.waitFor(() =>
      expect(seen.some((entry) => entry.path === '/offline/revocations')).toBe(true),
    );
    expect(seen.some((entry) => entry.path === '/offline/sync')).toBe(false);
    expect(storedDownloads()).toHaveLength(1);
  });

  it('uploads the queued events with sentAt and empties the queue (O-5)', async () => {
    const event: OfflineEvent = {
      profileId: 1,
      itemId: alien.id,
      playbackId: '01JB8Z6Q4M7N9P2R3S5T7V9P01',
      downloadId: downloadIds.alien,
      positionMs: 4000,
      durationMs: 8000,
      playedMs: 4000,
      seq: 1,
      state: 'paused',
      clientTime: '2026-10-20T10:00:00.000Z',
    };
    await queueEvents([
      { accountId: 1, event },
      { accountId: 2, event: { ...event, profileId: 7 } },
    ]);
    const seen = server({
      'POST /offline/sync': () => Response.json(syncAnswer()),
      'POST /offline/events': () =>
        Response.json({ accepted: 1, duplicates: 0, rejected: 0, results: ['accepted'] }),
    });
    start();
    await vi.waitFor(() => expect(queuedEvents()).toHaveLength(1), { timeout: 3000 });
    const upload = seen.find((entry) => entry.path === '/offline/events');
    expect(upload?.body).toMatchObject({ events: [event] });
    expect(typeof (upload?.body as { sentAt: string }).sentAt).toBe('string');
    // The other account's event waits for its own session.
    expect(queuedEvents()[0]?.accountId).toBe(2);
  });

  it('sends a delete made offline once it is online again', async () => {
    await putDownloads([storedDownload({ pendingDelete: true, complete: false })]);
    const seen = server({
      'DELETE /downloads/*': () => new Response(null, { status: 204 }),
      'POST /offline/sync': () => Response.json(syncAnswer()),
    });
    start();
    await vi.waitFor(() => expect(storedDownloads()).toHaveLength(0), { timeout: 3000 });
    expect(
      seen.some(
        (entry) => entry.method === 'DELETE' && entry.path === `/downloads/${downloadIds.alien}`,
      ),
    ).toBe(true);
    const sync = seen.find((entry) => entry.path === '/offline/sync');
    expect(sync?.body).toMatchObject({ held: [] });
  });

  it('asks the revocation list about another account’s downloads after a sync (W3, K7)', async () => {
    // A guest (account 2) used the device before; the owner (account 1) is signed in now.
    await putDownloads([heatDownload({ accountId: 2, allowedProfileIds: [9] })]);
    seedFile(downloadIds.heat, 'media.mp4', MEDIA);
    await putProfiles([
      storedProfile(),
      storedProfile({ id: 9, accountId: 2, snapshot: offlineProfile({ id: 9 }) }),
    ]);
    const guestEvent: OfflineEvent = {
      profileId: 9,
      itemId: alien.id,
      playbackId: '01JB8Z6Q4M7N9P2R3S5T7V9P01',
      downloadId: downloadIds.heat,
      positionMs: 1000,
      durationMs: 8000,
      playedMs: 1000,
      seq: 1,
      state: 'paused',
      clientTime: '2026-10-20T10:00:00.000Z',
    };
    await queueEvents([{ accountId: 2, event: guestEvent }]);
    const seen = server({
      'POST /offline/sync': () => Response.json(syncAnswer()),
      'POST /offline/revocations': ({ body }) =>
        Response.json({ revoked: (body as { ids: string[] }).ids }),
      'POST /offline/events': () =>
        Response.json({ accepted: 0, duplicates: 0, rejected: 0, results: [] }),
    });
    start();
    await vi.waitFor(() => expect(storedDownloads()).toHaveLength(0), { timeout: 3000 });
    // The sync named only the owner's downloads; the guest's went to the public list.
    expect(seen.find(isSync)?.body).toMatchObject({ held: [] });
    expect(seen.find((entry) => entry.path === '/offline/revocations')?.body).toEqual({
      ids: [downloadIds.heat],
    });
    expect(storedFiles(downloadIds.heat)).toEqual([]);
    expect(storedProfiles().map((profile) => profile.id)).toEqual([1]);
    // The guest's queued events can never go up any more.
    expect(queuedEvents()).toEqual([]);
  });

  it('a full storage stops the fetch as "quota" and pulls no block at the next trigger (W7)', async () => {
    failWrites(new DOMException('storage full', 'QuotaExceededError'));
    const seen = server(readyAlien());
    start();
    await vi.waitFor(() => expect(storedDownloads()[0]?.fetchError).toBe('quota'), {
      timeout: 3000,
    });
    expect(storedDownloads()[0]?.fetchErrorAt).toEqual(expect.any(Number));
    const blocks = seen.filter(isFile).length;
    expect(blocks).toBe(1);
    // A focus is another sync, but the full storage waits for FETCH_RETRY_MS.
    const syncs = seen.filter(isSync).length;
    window.dispatchEvent(new Event('focus'));
    await vi.waitFor(() => expect(seen.filter(isSync).length).toBeGreaterThan(syncs), {
      timeout: 3000,
    });
    await pause(300);
    expect(seen.filter(isFile)).toHaveLength(blocks);
    expect(storedDownloads()[0]?.fetchError).toBe('quota');
  });

  it('a delete while the file comes in waits until the fetch let go of it (W8)', async () => {
    let release = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const seen = server(
      readyAlien({
        'GET /downloads/*/file': async (entry) => {
          await gate;
          return fileRoute(entry);
        },
        'DELETE /downloads/*': () => new Response(null, { status: 204 }),
      }),
    );
    start();
    await vi.waitFor(() => expect(seen.some(isFile)).toBe(true), { timeout: 3000 });
    // The writer is open now and locks the folder, like a sync access handle.
    const removal = removeDownload(downloadIds.alien, true);
    release();
    await removal;
    expect(storedFiles(downloadIds.alien)).toEqual([]);
    expect(storedDownloads()).toEqual([]);
  });

  it('a removal that failed is tried again at the next run (W8)', async () => {
    seedFile(downloadIds.e1, 'media.mp4', MEDIA);
    failRemovals(1);
    server({ 'POST /offline/sync': () => Response.json(syncAnswer()) });
    start();
    await vi.waitFor(() => expect(storedFiles(downloadIds.e1)).toEqual([]), { timeout: 3000 });
  });

  it('another tab holding the fetch lock: this tab syncs and fetches nothing (W9)', async () => {
    const asked: string[] = [];
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: {
        request: (
          name: string,
          _options: LockOptions,
          callback: (lock: Lock | null) => Promise<void>,
        ) => {
          asked.push(name);
          return callback(name === 'slouchcat-offline-fetch' ? null : ({ name } as Lock));
        },
      },
    });
    try {
      const seen = server(readyAlien());
      start();
      await vi.waitFor(() => expect(storedDownloads()[0]?.state).toBe('ready'), {
        timeout: 3000,
      });
      await vi.waitFor(() => expect(asked).toContain('slouchcat-offline-fetch'));
      await pause(300);
      expect(asked).toContain('slouchcat-offline-sync');
      expect(seen.some((entry) => entry.path.includes('/downloads/'))).toBe(false);
    } finally {
      Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
    }
  });

  it('a sync answer does not bring back a download deleted here meanwhile (K5)', async () => {
    await setMeta('recentDeletes', { [downloadIds.alien]: Date.now() });
    const seen = server(readyAlien());
    start();
    await vi.waitFor(() => expect(seen.some(isSync)).toBe(true), { timeout: 3000 });
    await pause(300);
    expect(storedDownloads()).toEqual([]);
  });

  it('a complete download whose file is gone is fetched again (K3)', async () => {
    await putDownloads([storedDownload()]);
    const seen = server(readyAlien());
    start();
    await vi.waitFor(() => expect(seen.some(isFile)).toBe(true), { timeout: 3000 });
    await vi.waitFor(() => expect(storedDownloads()[0]?.complete).toBe(true), { timeout: 3000 });
    expect((await readFile(downloadIds.alien, 'media.mp4'))?.size).toBe(MEDIA.length);
  });

  it('a copy the server prepared again (new readyAt) starts the file over (O1)', async () => {
    // The device holds all bytes of the copy of 10:05, only the subtitle is missing.
    await putDownloads([
      storedDownload({
        complete: false,
        subtitlesStored: [],
        fileReadyAt: '2026-10-20T10:05:00Z',
        fileEtag: '"alien-0"',
      }),
    ]);
    seedFile(downloadIds.alien, 'media.mp4', new Uint8Array(MEDIA.length).fill(7));
    const seen = server(
      readyAlien({
        'POST /offline/sync': () =>
          Response.json(
            syncAnswer({ downloads: [downloadState({ readyAt: '2026-10-21T08:00:00Z' })] }),
          ),
      }),
    );
    start();
    await vi.waitFor(() => expect(storedDownloads()[0]?.complete).toBe(true), { timeout: 3000 });
    expect(
      new Uint8Array(await (await readFile(downloadIds.alien, 'media.mp4'))!.arrayBuffer()),
    ).toEqual(MEDIA);
    // From byte 0, without the old copy's ETag.
    expect(seen.filter(isFile).map((entry) => [entry.range, entry.ifRange])).toEqual([
      ['bytes=0-8999', null],
    ]);
    expect(storedDownloads()[0]).toMatchObject({
      fileReadyAt: '2026-10-21T08:00:00Z',
      fileEtag: '"alien-1"',
    });
  });
});

describe('the device’s account (decision of 2026-10-20, W2)', () => {
  it('a sign-in makes its account the device’s and ends a sign-out lock', async () => {
    await setMeta('accountId', 2);
    await setMeta('signedOut', true);
    server({ 'POST /offline/sync': () => Response.json(syncAnswer()) });
    start();
    await vi.waitFor(async () => expect(await getMeta('signedOut')).toBe(false), {
      timeout: 3000,
    });
    expect(await getMeta('accountId')).toBe(1);
  });

  it('without a session the lock stays', async () => {
    await setMeta('accountId', 1);
    await setMeta('signedOut', true);
    await putDownloads([storedDownload()]);
    const seen = server({ 'POST /offline/revocations': () => Response.json({ revoked: [] }) });
    start(null);
    await vi.waitFor(() =>
      expect(seen.some((entry) => entry.path === '/offline/revocations')).toBe(true),
    );
    expect(await getMeta('signedOut')).toBe(true);
  });
});

describe('one fetch loop per page (K6)', () => {
  it('runs while a loop is busy start no second one', async () => {
    let fetchLocks = 0;
    let release = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: {
        request: async (
          name: string,
          _options: LockOptions,
          callback: (lock: Lock | null) => Promise<void>,
        ) => {
          if (name !== 'slouchcat-offline-fetch') return callback({ name } as Lock);
          // The first loop stays busy until the end of the test.
          fetchLocks += 1;
          await gate;
          return callback(null);
        },
      },
    });
    try {
      const seen = server({ 'POST /offline/sync': () => Response.json(syncAnswer()) });
      start();
      await vi.waitFor(() => expect(fetchLocks).toBe(1), { timeout: 3000 });
      for (let run = 0; run < 2; run += 1) {
        const syncs = seen.filter(isSync).length;
        window.dispatchEvent(new Event('focus'));
        await vi.waitFor(() => expect(seen.filter(isSync).length).toBeGreaterThan(syncs), {
          timeout: 3000,
        });
      }
      await pause(100);
      expect(fetchLocks).toBe(1);
    } finally {
      release();
      Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
    }
  });
});
