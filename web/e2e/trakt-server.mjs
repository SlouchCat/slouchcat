// Local Trakt API for the Playwright suite (M7, E-7, I-7): the e2e server asks it instead of
// api.trakt.tv (SLOUCHCAT_TRAKT_BASE_URL), so no test needs the internet or a real client id.
// start-server.mjs starts it next to the Slouchcat binary and stops it with it; `node
// e2e/trakt-server.mjs` runs it alone.
//
//   GET  /movies/trending   [{watchers, movie: {title, year, ids}}]   (TRAKT_ENTRIES.movies)
//   GET  /movies/popular    [{title, year, ids}]
//   GET  /shows/trending    [{watchers, show: {title, year, ids}}]    (TRAKT_ENTRIES.shows)
//   GET  /shows/popular     [{title, year, ids}]
//   POST /__control         {"status": 503} answers every list with that status ("Trakt is
//                           down"), {"status": 200} serves the lists again
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

/** One list entry: trending wraps the title with a watcher count, popular lists it flat. */
function entries(media, list) {
  return TRAKT_ENTRIES[media].map((entry, index) => {
    const title = {
      title: entry.title,
      year: 2024,
      ids: { trakt: 900 + index, slug: `slouchcat-test-${index}`, tmdb: entry.tmdb },
    };
    return list === 'trending'
      ? { watchers: 100 - index, [media === 'movies' ? 'movie' : 'show']: title }
      : title;
  });
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
      const match = /^\/(movies|shows)\/(trending|popular)$/.exec(url.pathname);
      if (!match) {
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
      response.end(JSON.stringify(entries(match[1], match[2])));
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
