// Local Trakt API for the Playwright suite (M7, E-7, I-7; M11 plan 5.6): the e2e server asks it
// instead of api.trakt.tv (SLOUCHCAT_TRAKT_BASE_URL), so no test needs the internet or a real
// client id. start-server.mjs starts it next to the Slouchcat binary and stops it with it; `node
// e2e/trakt-server.mjs` runs it alone. Every list serves the library titles of TRAKT_ENTRIES (the
// box office the films only), so a spec that switches the spotlight list still sees its hits.
//
//   GET  /movies/trending             [{watchers, movie: {title, year, ids}}]
//   GET  /shows/trending              [{watchers, show: {title, year, ids}}]
//   GET  /movies/popular              [{title, year, ids}]
//   GET  /shows/popular               [{title, year, ids}]
//   GET  /movies/watched/weekly       [{watcher_count, play_count, movie: {…}}]
//   GET  /shows/watched/weekly        [{watcher_count, play_count, show: {…}}]
//   GET  /movies/collected/monthly    [{collected_count, movie: {…}}]
//   GET  /shows/collected/monthly     [{collected_count, show: {…}}]
//   GET  /movies/anticipated          [{list_count, movie: {…}}]
//   GET  /shows/anticipated           [{list_count, show: {…}}]
//   GET  /movies/boxoffice            [{revenue, movie: {…}}]
//   GET  /users/e2e/lists/best-ever/items
//                                     [{rank, type, movie | show}], the films, then the series,
//                                     then one "season" entry the server's post-filter must drop
//   POST /__control                   {"status": 503} answers every list with that status ("Trakt
//                                     is down"), {"status": 200} serves the lists again
//
// Every list request must carry `trakt-api-key` (the e2e client id `e2e`) and `trakt-api-version:
// 2`, as the real API wants; otherwise 401.
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

/** Client id the e2e server is started with (SLOUCHCAT_TRAKT_CLIENT_ID). */
export const TRAKT_CLIENT_ID = 'e2e';

/**
 * The library titles the lists point at, by TMDB id: the test media carry these ids in their
 * folder names (`{tmdb-NNNN}`, testdata/generate-media.sh). Five films and two series, so each
 * row maps seven titles, above the default minimum of five hits (spotlight_config.min_hits).
 */
export const TRAKT_ENTRIES = {
  movies: [
    { tmdb: 7001, title: 'Slouchcat Test AC3 51' },
    { tmdb: 7002, title: 'Slouchcat Test DTS' },
    { tmdb: 7003, title: 'Slouchcat Test HEVC 10bit' },
    { tmdb: 7004, title: 'Slouchcat Test Chapters' },
    { tmdb: 7005, title: 'Slouchcat Test H264 MKV Long' },
  ],
  shows: [
    { tmdb: 7101, title: 'Slouchcat Test Download Show' },
    { tmdb: 7102, title: 'Slouchcat Test Intro Show' },
  ],
};

/** The public user list the mock serves (https://trakt.tv/users/e2e/lists/best-ever). */
export const TRAKT_USER_LIST = { user: 'e2e', slug: 'best-ever' };

/** The title object of a list entry. */
function titleOf(entry, index) {
  return {
    title: entry.title,
    year: 2024,
    ids: { trakt: 900 + index, slug: `slouchcat-test-${index}`, tmdb: entry.tmdb },
  };
}

/** The fields next to the wrapped title, per list (popular lists the title flat). */
const WRAPPERS = {
  trending: (index) => ({ watchers: 100 - index }),
  'watched/weekly': (index) => ({ watcher_count: 100 - index, play_count: 500 - index }),
  'collected/monthly': (index) => ({ collected_count: 100 - index }),
  anticipated: (index) => ({ list_count: 100 - index }),
  boxoffice: (index) => ({ revenue: 1_000_000 - index * 1000 }),
};

/** A path list: `media` is movies or shows, `list` a key of WRAPPERS or popular. */
function entries(media, list) {
  return TRAKT_ENTRIES[media].map((entry, index) => {
    const title = titleOf(entry, index);
    const wrap = WRAPPERS[list];
    return wrap ? { ...wrap(index), [media === 'movies' ? 'movie' : 'show']: title } : title;
  });
}

/**
 * The user list: every entry names its type; the films and series rank 1 to 7, then a season of
 * the intro show, which carries its show and must not count as one.
 */
function userListEntries() {
  const titles = [
    ...TRAKT_ENTRIES.movies.map((entry, index) => ({
      type: 'movie',
      movie: titleOf(entry, index),
    })),
    ...TRAKT_ENTRIES.shows.map((entry, index) => ({ type: 'show', show: titleOf(entry, index) })),
  ];
  const intro = TRAKT_ENTRIES.shows[1];
  titles.push({
    type: 'season',
    season: { number: 1, ids: { trakt: 990, tmdb: 79901 } },
    show: titleOf(intro, 1),
  });
  return titles.map((entry, index) => ({ rank: index + 1, id: 800 + index, ...entry }));
}

/** The response body for a list path, or undefined for an unknown path. */
function listBody(pathname) {
  if (pathname === `/users/${TRAKT_USER_LIST.user}/lists/${TRAKT_USER_LIST.slug}/items`) {
    return userListEntries();
  }
  const match =
    /^\/(movies|shows)\/(trending|popular|watched\/weekly|collected\/monthly|anticipated)$/.exec(
      pathname,
    ) ?? /^\/(movies)\/(boxoffice)$/.exec(pathname);
  return match ? entries(match[1], match[2]) : undefined;
}

function readBody(request) {
  return new Promise((resolve, reject) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk) => {
      body += chunk;
    });
    request.on('end', () => resolve(body));
    request.on('error', reject);
  });
}

/**
 * Starts the Trakt mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_TRAKT_PORT, default 8094); resolves
 * with its port and a `close` function.
 */
export function startTraktServer({
  port = Number(process.env.SLOUCHCAT_E2E_TRAKT_PORT ?? 8094),
} = {}) {
  let status = 200;

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    void (async () => {
      if (request.method === 'POST' && url.pathname === '/__control') {
        const body = JSON.parse((await readBody(request)) || '{}');
        if (!Number.isInteger(body.status) || body.status < 200 || body.status > 599) {
          response.writeHead(400).end();
          return;
        }
        status = body.status;
        response.writeHead(204).end();
        return;
      }
      if (request.method !== 'GET') {
        response.writeHead(405).end();
        return;
      }
      const body = listBody(url.pathname);
      if (!body) {
        response.writeHead(404).end();
        return;
      }
      if (
        request.headers['trakt-api-key'] !== TRAKT_CLIENT_ID ||
        request.headers['trakt-api-version'] !== '2'
      ) {
        response.writeHead(401).end();
        return;
      }
      if (status !== 200) {
        response.writeHead(status, { 'Content-Type': 'application/json' }).end('{}');
        return;
      }
      response.writeHead(200, {
        'Content-Type': 'application/json',
        'Cache-Control': 'no-store',
      });
      response.end(JSON.stringify(body));
    })().catch(() => {
      if (!response.headersSent) response.writeHead(500);
      response.end();
    });
  });

  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', () => {
      resolve({
        port,
        close: () =>
          new Promise((done) => {
            server.closeAllConnections();
            server.close(() => done());
          }),
      });
    });
  });
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { port } = await startTraktServer();
  console.log(`trakt mock on http://127.0.0.1:${port}`);
}
