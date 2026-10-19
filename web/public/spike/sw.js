// Service worker of the offline spike (scope /spike/). It answers GET /spike/opfs/<name> from the
// origin private file system with real Range responses (206, Content-Range, Accept-Ranges), so a
// <video> can seek in a 1.5 GB download without reading the whole file: only File.slice() of the
// requested bytes is handed to the Response. Every other request goes to the network untouched.
// Classic script (module service workers are not available in every browser under test).
// Temporary: removed together with the spike after the M1b decision.

const SW_VERSION = 'spike-sw-1';

self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));

self.addEventListener('message', (event) => {
  if (event.data?.type === 'version')
    event.source?.postMessage({ type: 'version', version: SW_VERSION });
});

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);
  const prefix = new URL('opfs/', self.registration.scope).pathname;
  if (url.origin !== self.location.origin || !url.pathname.startsWith(prefix)) return;
  if (event.request.method !== 'GET') return;
  const name = decodeURIComponent(url.pathname.slice(prefix.length));
  event.respondWith(serveFromOpfs(name, event.request));
});

// Same logic as parseRangeRequest in lib.js (tested there); kept here because this is a classic script.
function parseRange(value, size) {
  const m = /^bytes=(\d*)-(\d*)$/.exec((value || '').trim());
  if (!m || (m[1] === '' && m[2] === '') || size === 0) return null;
  let start;
  let end;
  if (m[1] === '') {
    const suffix = Number(m[2]);
    if (suffix === 0) return null;
    start = Math.max(0, size - suffix);
    end = size - 1;
  } else {
    start = Number(m[1]);
    end = m[2] === '' ? size - 1 : Math.min(Number(m[2]), size - 1);
  }
  if (start >= size || start > end) return null;
  return { start, end };
}

async function report(message) {
  const clients = await self.clients.matchAll({ type: 'window' });
  for (const client of clients) client.postMessage(message);
}

async function serveFromOpfs(name, request) {
  const t0 = performance.now();
  if (!/^[\w.-]+$/.test(name)) return new Response('bad file name', { status: 400 });
  let file;
  try {
    const root = await navigator.storage.getDirectory();
    const fh = await root.getFileHandle(name);
    file = await fh.getFile();
  } catch (err) {
    await report({ type: 'range', name, error: `${err.name}: ${err.message}` });
    return new Response('not found in OPFS', { status: 404 });
  }
  const size = file.size;
  const headers = {
    'Content-Type': 'video/mp4',
    'Accept-Ranges': 'bytes',
    'Cache-Control': 'no-store',
  };
  const rangeValue = request.headers.get('Range');
  if (!rangeValue) {
    await report({ type: 'range', name, range: null, status: 200, bytes: size });
    return new Response(file, {
      status: 200,
      headers: { ...headers, 'Content-Length': String(size) },
    });
  }
  const r = parseRange(rangeValue, size);
  if (!r) {
    await report({ type: 'range', name, range: rangeValue, status: 416 });
    return new Response(null, {
      status: 416,
      headers: { ...headers, 'Content-Range': `bytes */${size}` },
    });
  }
  const length = r.end - r.start + 1;
  const body = file.slice(r.start, r.end + 1, 'video/mp4');
  await report({
    type: 'range',
    name,
    range: rangeValue,
    status: 206,
    bytes: length,
    ms: Math.round(performance.now() - t0),
  });
  return new Response(body, {
    status: 206,
    statusText: 'Partial Content',
    headers: {
      ...headers,
      'Content-Range': `bytes ${r.start}-${r.end}/${size}`,
      'Content-Length': String(length),
    },
  });
}
