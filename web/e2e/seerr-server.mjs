// Local Seerr for the Playwright suite (concept 25): the administration's pages Integrationen ›
// Seerr and Medien › Wünsche talk to it instead of a real Seerr. start-server.mjs starts it next
// to the Slouchcat binary and stops it with it; `node e2e/seerr-server.mjs` runs it alone. The
// specs enter its address in the form: the e2e server gets no SLOUCHCAT_SEERR_* variable, which
// would lock the form. The answers have the shapes Seerr 3.5.1 gave in the check of the real
// instances (docs/planning/25-medienwuensche/anhang/I-pruefung-echte-instanzen.md).
//
// Below /api/v1, every call but /status needs `X-Api-Key: SEERR_API_KEY`; otherwise 403 with the
// text in `error` (I-2). `X-API-User: <id>` acts as that user (I-4); an unknown id answers the same
// 403 (I-3).
//
//   GET  /api/v1/status              {version, commitTag, …} without a key (I-1)
//   GET  /api/v1/auth/me             user 1 (ADMIN), or the user of X-API-User
//   GET  /api/v1/settings/public     localLogin on (I-6)
//   GET  /api/v1/settings/main       defaultPermissions and defaultQuotas (I-7)
//   GET  /api/v1/settings/metadatas  {"tv": "tmdb", "anime": "tmdb"} (I-8): the test warns
//   GET  /api/v1/settings/radarr     one Radarr server (I-9)
//   GET  /api/v1/settings/sonarr     one Sonarr server
//   POST /api/v1/request {}          400 from the request validator: CSRF protection is off
//   GET  /api/v1/user?take=&skip=    {pageInfo, results} (I-11)
//   GET  /api/v1/user/{id}           one user, 404 {"message": "User not found."}
//   POST /api/v1/user                a local user (201, without e-mail in the answer, I-12); the
//                                    same e-mail in any case 409 USER_EXISTS (I-14)
//   POST /__control                  {"instance": "other"} answers as another instance (user 1
//                                    with another e-mail and creation time), {"instance": "same"}
//                                    as the first one again
//   POST /__reset                    the first instance and the five users of USERS again
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

/** The api key the specs enter (a fake; the log scan of global-teardown.ts looks for it). */
export const SEERR_API_KEY = 'e2e-seerr-key-4f2a9c71d3b8';

/** Seerr's version: Seerr from 3.0 counts as tested (architecture 15.1). */
export const SEERR_VERSION = '3.5.1';

// Permission bits of Seerr (server/internal/seerr/types.go).
const ADMIN = 2;
const REQUEST = 32;
const AUTO_APPROVE = 128;
const REQUEST_MOVIE = 262144;
const MANAGE_ISSUES = 1048576;

/**
 * The users of the instance. Neutral names, e-mails at example.org. User 1 is the owner of the key
 * (ADMIN); the others sign in through Emby (userType 4) as on the owner's instance (I-10), apart
 * from one local user.
 */
export const USERS = [
  {
    id: 1,
    displayName: 'seerr-admin',
    email: 'admin@example.org',
    userType: 2,
    permissions: ADMIN,
  },
  {
    id: 2,
    displayName: 'wohnzimmer',
    email: 'wohnzimmer@example.org',
    userType: 4,
    permissions: REQUEST | AUTO_APPROVE,
  },
  {
    id: 3,
    displayName: 'kinder',
    email: 'kinder@example.org',
    userType: 4,
    permissions: REQUEST,
  },
  {
    id: 5,
    displayName: 'gast',
    email: 'gast@example.org',
    userType: 2,
    permissions: REQUEST | AUTO_APPROVE | MANAGE_ISSUES,
  },
  {
    id: 7,
    displayName: 'nur-filme',
    email: 'filme@example.org',
    userType: 4,
    permissions: REQUEST_MOVIE,
    movieQuotaLimit: 5,
    movieQuotaDays: 7,
  },
];

/** The creation time of every user of the first instance; user 1's is the fingerprint. */
const CREATED = '2026-01-10T08:00:00.000Z';
/** User 1 of «another instance» (architecture 7.4: e-mail and creation time of user 1). */
const OTHER_ADMIN = { email: 'other-admin@example.org', createdAt: '2026-06-01T09:30:00.000Z' };

const DENIED = { status: 403, error: 'You do not have permission to access this endpoint' };

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

/** A user as GET /user and /auth/me answer it (I-11). */
function userOf(user, other) {
  const first = user.id === 1 && other;
  return {
    id: user.id,
    email: first ? OTHER_ADMIN.email : user.email,
    username: user.displayName,
    plexUsername: null,
    jellyfinUsername: user.userType === 4 ? user.displayName : null,
    jellyfinUserId: user.userType === 4 ? `emby-${user.id}` : null,
    displayName: user.displayName,
    userType: user.userType,
    permissions: user.permissions,
    avatar: '',
    requestCount: 0,
    movieQuotaLimit: user.movieQuotaLimit ?? null,
    movieQuotaDays: user.movieQuotaDays ?? null,
    tvQuotaLimit: user.tvQuotaLimit ?? null,
    tvQuotaDays: user.tvQuotaDays ?? null,
    createdAt: first ? OTHER_ADMIN.createdAt : CREATED,
    updatedAt: CREATED,
  };
}

const dvr = (id, name, port) => ({
  id,
  name,
  hostname: '127.0.0.1',
  port,
  apiKey: 'e2e-dvr-key',
  useSsl: false,
  baseUrl: '',
  activeProfileId: 1,
  activeProfileName: 'HD-1080p',
  activeDirectory: '/media',
  tags: [],
  tagRequests: false,
  is4k: false,
  isDefault: true,
  externalUrl: '',
  syncEnabled: true,
  preventSearch: false,
});

/**
 * Starts the Seerr mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_SEERR_PORT, default 8086); resolves
 * with its port and a `close` function.
 */
export function startSeerrServer({
  port = Number(process.env.SLOUCHCAT_E2E_SEERR_PORT ?? 8086),
} = {}) {
  let users = USERS.map((user) => ({ ...user }));
  let other = false;
  let nextId = 20;

  const routes = async (request, url, response) => {
    const path = url.pathname.replace(/^\/api\/v1/, '');
    const method = request.method ?? 'GET';
    if (method === 'GET' && path === '/status') {
      sendJson(response, 200, {
        version: SEERR_VERSION,
        commitTag: `v${SEERR_VERSION}`,
        updateAvailable: false,
        commitsBehind: 0,
        restartRequired: false,
      });
      return;
    }
    if (request.headers['x-api-key'] !== SEERR_API_KEY) {
      sendJson(response, 403, DENIED);
      return;
    }
    const actAs = request.headers['x-api-user'];
    const actor = actAs === undefined ? users[0] : users.find((user) => String(user.id) === actAs);
    if (!actor) {
      sendJson(response, 403, DENIED);
      return;
    }
    if (method === 'GET' && path === '/auth/me') {
      sendJson(response, 200, userOf(actor, other));
      return;
    }
    if (method === 'GET' && path === '/settings/public') {
      sendJson(response, 200, {
        applicationTitle: 'Seerr',
        applicationUrl: '',
        localLogin: true,
        mediaServerLogin: true,
        mediaServerType: 3,
        initialized: true,
        partialRequestsEnabled: true,
        enableSpecialEpisodes: false,
        movie4kEnabled: false,
        series4kEnabled: false,
        emailEnabled: false,
        userEmailRequired: false,
        locale: 'de',
      });
      return;
    }
    if (method === 'GET' && path === '/settings/main') {
      sendJson(response, 200, {
        applicationTitle: 'Seerr',
        defaultPermissions: REQUEST | AUTO_APPROVE | MANAGE_ISSUES,
        defaultQuotas: {
          movie: { quotaLimit: 0, quotaDays: 7 },
          tv: { quotaLimit: 0, quotaDays: 7 },
        },
        localLogin: true,
      });
      return;
    }
    if (method === 'GET' && path === '/settings/metadatas') {
      sendJson(response, 200, { tv: 'tmdb', anime: 'tmdb' });
      return;
    }
    if (method === 'GET' && path === '/settings/radarr') {
      sendJson(response, 200, [dvr(0, 'Radarr', 7878)]);
      return;
    }
    if (method === 'GET' && path === '/settings/sonarr') {
      sendJson(response, 200, [dvr(0, 'Sonarr', 8989)]);
      return;
    }
    if (method === 'POST' && path === '/request') {
      // The request validator refuses the empty body: CSRF protection is off.
      sendJson(response, 400, { message: 'request/body must have required property mediaType' });
      return;
    }
    if (method === 'GET' && path === '/user') {
      const take = Number(url.searchParams.get('take') ?? 10);
      const skip = Number(url.searchParams.get('skip') ?? 0);
      const results = users.slice(skip, skip + take).map((user) => userOf(user, other));
      sendJson(response, 200, {
        pageInfo: {
          pages: Math.max(1, Math.ceil(users.length / take)),
          pageSize: take,
          results: users.length,
          page: Math.floor(skip / take) + 1,
        },
        results,
      });
      return;
    }
    if (method === 'POST' && path === '/user') {
      const body = JSON.parse((await readBody(request)) || '{}');
      const email = String(body.email ?? '').toLowerCase();
      if (users.some((user) => user.email.toLowerCase() === email)) {
        sendJson(response, 409, {
          message: 'User already exists with submitted email.',
          errors: ['USER_EXISTS'],
        });
        return;
      }
      const user = {
        id: nextId,
        displayName: String(body.username || email.split('@')[0]),
        email,
        userType: 2,
        permissions: REQUEST | AUTO_APPROVE | MANAGE_ISSUES,
      };
      nextId += 1;
      users.push(user);
      // The answer carries no e-mail (I-12).
      sendJson(response, 201, { ...userOf(user, other), email: undefined });
      return;
    }
    const single = /^\/user\/(\d+)$/.exec(path);
    if (method === 'GET' && single) {
      const user = users.find((entry) => String(entry.id) === single[1]);
      if (user) sendJson(response, 200, userOf(user, other));
      else sendJson(response, 404, { message: 'User not found.' });
      return;
    }
    sendJson(response, 404, { message: 'Not found.' });
  };

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    void (async () => {
      if (request.method === 'POST' && url.pathname === '/__control') {
        const body = JSON.parse((await readBody(request)) || '{}');
        if (body.instance !== 'other' && body.instance !== 'same') {
          response.writeHead(400).end();
          return;
        }
        other = body.instance === 'other';
        response.writeHead(204).end();
        return;
      }
      if (request.method === 'POST' && url.pathname === '/__reset') {
        users = USERS.map((user) => ({ ...user }));
        other = false;
        nextId = 20;
        response.writeHead(204).end();
        return;
      }
      if (!url.pathname.startsWith('/api/v1/')) {
        response.writeHead(404).end();
        return;
      }
      await routes(request, url, response);
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
  const { port } = await startSeerrServer();
  console.log(`seerr mock on http://127.0.0.1:${port}`);
}
