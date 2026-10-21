// Local release manifest for the Playwright suite (M8, A-5): the update check of the e2e server
// reads it instead of GitHub (SLOUCHCAT_UPDATE_MANIFEST_URL, honoured with --dev only).
// start-server.mjs starts it next to the Slouchcat binary and stops it with it; `node
// e2e/update-server.mjs` runs it alone.
//
//   GET  /manifest.json   {version, url, notes, publishedAt} once a version is set; 404 before,
//                         like a project without a release. The e2e binary's version is a git hash,
//                         which the server counts as older than any release: a manifest from the
//                         start would put the update badge on every owner page of the suite.
//   POST /__control       {"version": "9.9.9", "url"?, "notes"?, "publishedAt"?} publishes that
//                         release; {"version": null} withdraws it (404 again)
//   GET  /__requests      [{at, userAgent}] of the manifest requests
//   POST /__reset         withdraws the release and forgets the requests
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

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

const sendJson = (response, status, body) => {
  response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
  response.end(JSON.stringify(body));
};

/**
 * Starts the manifest mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_UPDATE_PORT, default 8090);
 * resolves with its port and a `close` function.
 */
export function startUpdateServer({
  port = Number(process.env.SLOUCHCAT_E2E_UPDATE_PORT ?? 8090),
} = {}) {
  let manifest = null;
  let requests = [];

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    void (async () => {
      if (url.pathname === '/manifest.json' && request.method === 'GET') {
        requests.push({
          at: new Date().toISOString(),
          userAgent: request.headers['user-agent'] ?? '',
        });
        if (!manifest) {
          response.writeHead(404, { 'Content-Type': 'text/plain' }).end('Not Found');
          return;
        }
        sendJson(response, 200, manifest);
        return;
      }
      if (url.pathname === '/__control' && request.method === 'POST') {
        const body = JSON.parse((await readBody(request)) || '{}');
        if (body.version === null) {
          manifest = null;
        } else if (typeof body.version === 'string' && body.version !== '') {
          manifest = {
            version: body.version,
            url: body.url ?? `https://github.com/SlouchCat/slouchcat/releases/tag/v${body.version}`,
            notes: body.notes ?? `Slouchcat ${body.version}`,
            publishedAt: body.publishedAt ?? new Date().toISOString(),
          };
        } else {
          response.writeHead(400).end();
          return;
        }
        response.writeHead(204).end();
        return;
      }
      if (url.pathname === '/__requests' && request.method === 'GET') {
        sendJson(response, 200, requests);
        return;
      }
      if (url.pathname === '/__reset' && request.method === 'POST') {
        manifest = null;
        requests = [];
        response.writeHead(204).end();
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
  const { port } = await startUpdateServer();
  console.log(`update manifest mock on http://127.0.0.1:${port}/manifest.json`);
}
