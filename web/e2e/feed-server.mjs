// Local podcast feed for the Playwright suite (M6, H-2): the e2e server subscribes to it, so no
// test needs the internet. start-server.mjs starts it next to the Slouchcat binary and stops it
// with it; `node e2e/feed-server.mjs` runs it alone.
//
//   GET  /feed.xml          testdata/podcasts/feed.xml.tmpl with {{BASE}} = http://127.0.0.1:<port>,
//                           only the first `n` <item> elements (n starts at 2)
//   GET  /episode-N.mp3     testdata/media/Podcasts/episode-N.mp3 (Range requests answered)
//   GET  /cover.jpg         testdata/media/Podcasts/cover.jpg
//   POST /__control         {"episodes": n} sets n ("a new episode appears")
//
// A prefix /f/<key>/ serves the same feed with a counter of its own ({{BASE}} includes the prefix,
// `POST /__control {"feed": "<key>", "episodes": n}` sets it): a spec that needs a new episode
// subscribes a feed of its own and does not depend on the order of the specs.
import { createReadStream, existsSync, readFileSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

const TEMPLATE = fileURLToPath(new URL('../../testdata/podcasts/feed.xml.tmpl', import.meta.url));
const MEDIA = fileURLToPath(new URL('../../testdata/media/Podcasts/', import.meta.url));
const START_EPISODES = 2;
const KEY = /^[A-Za-z0-9_-]{1,80}$/;

/** The feed with its first `episodes` items, the links pointing at `base`. */
export function renderFeed(template, base, episodes) {
  let index = 0;
  return template
    .replace(/[ \t]*<item>[\s\S]*?<\/item>\n?/g, (item) => {
      index += 1;
      return index <= episodes ? item : '';
    })
    .replaceAll('{{BASE}}', base);
}

/** `bytes=start-end` of a Range header within `size`, or null for the whole file. */
function byteRange(header, size) {
  const match = /^bytes=(\d*)-(\d*)$/.exec(header ?? '');
  if (!match || (match[1] === '' && match[2] === '')) return null;
  if (match[1] === '') {
    const suffix = Math.min(Number(match[2]), size);
    return { start: size - suffix, end: size - 1 };
  }
  const start = Number(match[1]);
  const end = match[2] === '' ? size - 1 : Math.min(Number(match[2]), size - 1);
  return start <= end && start < size ? { start, end } : 'invalid';
}

function sendFile(request, response, path, type) {
  if (!existsSync(path)) {
    response.writeHead(404).end();
    return;
  }
  const size = statSync(path).size;
  const range = byteRange(request.headers.range, size);
  const headers = { 'Content-Type': type, 'Accept-Ranges': 'bytes', 'Cache-Control': 'no-store' };
  if (range === 'invalid') {
    response.writeHead(416, { ...headers, 'Content-Range': `bytes */${size}` }).end();
    return;
  }
  if (range === null) {
    response.writeHead(200, { ...headers, 'Content-Length': size });
    if (request.method === 'HEAD') response.end();
    else createReadStream(path).pipe(response);
    return;
  }
  response.writeHead(206, {
    ...headers,
    'Content-Length': range.end - range.start + 1,
    'Content-Range': `bytes ${range.start}-${range.end}/${size}`,
  });
  if (request.method === 'HEAD') response.end();
  else createReadStream(path, { start: range.start, end: range.end }).pipe(response);
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
 * Starts the feed server on 127.0.0.1:`port` (SLOUCHCAT_E2E_FEED_PORT, default 8095); resolves
 * with its port and a `close` function.
 */
export function startFeedServer({
  port = Number(process.env.SLOUCHCAT_E2E_FEED_PORT ?? 8095),
} = {}) {
  const template = readFileSync(TEMPLATE, 'utf8');
  // Episodes per feed: '' is /feed.xml, any other key /f/<key>/feed.xml.
  const episodes = new Map();
  const count = (key) => episodes.get(key) ?? START_EPISODES;

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    void (async () => {
      if (request.method === 'POST' && url.pathname === '/__control') {
        const body = JSON.parse((await readBody(request)) || '{}');
        const key = typeof body.feed === 'string' ? body.feed : '';
        if ((key !== '' && !KEY.test(key)) || !Number.isInteger(body.episodes)) {
          response.writeHead(400).end();
          return;
        }
        episodes.set(key, Math.max(0, Math.min(3, body.episodes)));
        response.writeHead(204).end();
        return;
      }
      if (request.method !== 'GET' && request.method !== 'HEAD') {
        response.writeHead(405).end();
        return;
      }
      const prefixed = /^\/f\/([^/]+)(\/.*)$/.exec(url.pathname);
      const key = prefixed ? prefixed[1] : '';
      const path = prefixed ? prefixed[2] : url.pathname;
      if (key !== '' && !KEY.test(key)) {
        response.writeHead(404).end();
        return;
      }
      if (path === '/feed.xml') {
        const base = `http://127.0.0.1:${port}${key ? `/f/${key}` : ''}`;
        const xml = renderFeed(template, base, count(key));
        response.writeHead(200, {
          'Content-Type': 'application/rss+xml; charset=utf-8',
          'Cache-Control': 'no-store',
        });
        response.end(request.method === 'HEAD' ? undefined : xml);
        return;
      }
      const episode = /^\/episode-([1-3])\.mp3$/.exec(path);
      if (episode) {
        sendFile(request, response, `${MEDIA}episode-${episode[1]}.mp3`, 'audio/mpeg');
        return;
      }
      if (path === '/cover.jpg') {
        sendFile(request, response, `${MEDIA}cover.jpg`, 'image/jpeg');
        return;
      }
      response.writeHead(404).end();
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
  const { port } = await startFeedServer();
  console.log(`feed server on http://127.0.0.1:${port}/feed.xml`);
}
