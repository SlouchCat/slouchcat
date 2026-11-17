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
//   POST /api/v1/request {mediaType, mediaId, tvdbId?, seasons?}   a request of the X-API-User
//                                    (concept 25 stage 2): pending, or approved with AUTO_APPROVE
//                                    or ADMIN; 409 «Request for this media already exists.» for an
//                                    open request of the film or of every chosen season; 403
//                                    «Movie Quota exceeded.» / «Series Quota exceeded.» past the
//                                    user's limit, which counts the requests (a season each)
//   GET  /api/v1/request?take=&skip=&requestedBy=&filter=   {pageInfo, results}, newest first
//   GET|DELETE /api/v1/request/{id}  one request; DELETE 204 (only pending, or the key's owner)
//   POST /api/v1/request/{id}/retry  a failed request (status 4) sent again: approved (2), 200
//                                    with the request; any other 500 «Request is not failed.»
//   GET  /api/v1/movie/{id}, /tv/{id}  the titles of tmdb-server.mjs with mediaInfo and, for a
//                                    series, the seasons Seerr can order (season 0 included)
//   GET  /api/v1/user?take=&skip=    {pageInfo, results} (I-11)
//   GET  /api/v1/user/{id}           one user, 404 {"message": "User not found."}
//   POST /api/v1/user                a local user (201, without e-mail in the answer, I-12); the
//                                    same e-mail in any case 409 USER_EXISTS (I-14)
//   GET  /api/v1/user/{id}/settings/main    username, e-mail, the four limit fields and the
//                                    instance's `global*` ones (I-15)
//   POST /api/v1/user/{id}/settings/main    writes them (200 without the limit fields, I-16);
//                                    the e-mail of another user 400 INVALID_EMAIL (I-18)
//   GET|POST /api/v1/user/{id}/settings/permissions   {permissions} (I-19)
//   GET|POST /api/v1/user/{id}/settings/password      {hasPassword}; POST 204, under 8
//                                    characters 400 (I-20)
//   GET  /api/v1/user/{id}/quota     {movie {days, limit, used, remaining?, restricted}, tv}
//                                    (I-30); limit 0 without `remaining` (I-31), null takes the
//                                    instance's default (I-32), an administrator 0 (I-34)
//   POST /__control                  {"instance": "other"} answers as another instance (user 1
//                                    with another e-mail and creation time), {"instance": "same"}
//                                    as the first one again; {"failCreate": true|false} answers
//                                    POST /user with 500 (an account then waits for Seerr);
//                                    {"used": {"<id>": {"movie": n, "tv": n}}} sets what a
//                                    user's limit has used
//                                    {"request": {"id": n, "status": 1..5}} sets a request's
//                                    status (approve 2, decline 3, fail 4, complete 5)
//   GET  /__requests                 every request, for the specs' checks
//   POST /__reset                    the first instance, the five users of USERS and no requests
//                                    (request and user ids go on counting)
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';
import { MOVIES, SHOWS } from './tmdb-server.mjs';

/** The api key the specs enter (a fake; the log scan of global-teardown.ts looks for it). */
export const SEERR_API_KEY = 'e2e-seerr-key-4f2a9c71d3b8';

/** Seerr's version: Seerr from 3.0 counts as tested (architecture 15.1). */
export const SEERR_VERSION = '3.5.1';

// Permission bits of Seerr (server/internal/seerr/types.go).
const ADMIN = 2;
const REQUEST = 32;
const AUTO_APPROVE = 128;
const REQUEST_MOVIE = 262144;
const MANAGE_USERS = 8;
const MANAGE_ISSUES = 1048576;
/** The instance's default rights of a new user (I-13). */
const DEFAULT_PERMISSIONS = REQUEST | AUTO_APPROVE | MANAGE_ISSUES;
/** The instance's default limit (I-7): 0 is unlimited (I-31). */
const DEFAULT_QUOTA = { limit: 0, days: 7 };

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

/** A part of GET /user/{id}/quota (I-30, I-31, I-32, I-34). */
function quotaPart(user, kind, used, requested = 0) {
  const admin = (user.permissions & (ADMIN | MANAGE_USERS)) !== 0;
  const limit = admin ? 0 : (user[`${kind}QuotaLimit`] ?? DEFAULT_QUOTA.limit);
  const days = user[`${kind}QuotaDays`] ?? DEFAULT_QUOTA.days;
  const count = (used?.[kind] ?? 0) + requested;
  if (limit === 0) return { days, limit: 0, used: count, restricted: false };
  const remaining = Math.max(0, limit - count);
  return { days, limit, used: count, remaining, restricted: remaining === 0 };
}

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
  let failCreate = false;
  /** What each user's limit has used, by id. */
  let used = {};
  /** The requests made through the mock (concept 25 stage 2). */
  let requests = [];
  let nextRequest = 100;

  /** What a user's requests count against the limit: films, and seasons of series. */
  const requested = (user, kind) =>
    requests
      .filter((r) => r.requestedBy === user.id && r.status !== 3 && r.type === kind)
      .reduce((sum, r) => sum + (kind === 'movie' ? 1 : r.seasons.length), 0);

  /** Seerr's media row of a title: its status follows the newest open request. */
  const mediaOf = (type, tmdbId) => {
    const own = requests.filter((r) => r.type === type && r.mediaId === tmdbId && r.status !== 3);
    if (own.length === 0) return undefined;
    const approved = own.some((r) => r.status === 2 || r.status === 5);
    const status = own.some((r) => r.status === 5) ? 5 : approved ? 3 : 2;
    return {
      id: tmdbId + 50000,
      tmdbId,
      tvdbId: own[0].tvdbId ?? null,
      mediaType: type,
      status,
      status4k: 1,
      seasons: own.flatMap((r) =>
        r.seasons.map((season) => ({
          id: season.id,
          seasonNumber: season.seasonNumber,
          status,
          status4k: 1,
        })),
      ),
      requests: [],
      createdAt: CREATED,
      updatedAt: CREATED,
    };
  };

  const requestOf = (r) => ({
    id: r.id,
    status: r.status,
    type: r.type,
    is4k: false,
    media: mediaOf(r.type, r.mediaId) ?? {
      id: r.mediaId + 50000,
      tmdbId: r.mediaId,
      mediaType: r.type,
      status: 1,
      status4k: 1,
      seasons: [],
    },
    seasons: r.seasons.map((season) => ({ ...season, status: r.status })),
    requestedBy: userOf(users.find((user) => user.id === r.requestedBy) ?? users[0], other),
    modifiedBy: null,
    createdAt: r.createdAt,
    updatedAt: r.updatedAt,
  });

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
        defaultPermissions: DEFAULT_PERMISSIONS,
        defaultQuotas: {
          movie: { quotaLimit: DEFAULT_QUOTA.limit, quotaDays: DEFAULT_QUOTA.days },
          tv: { quotaLimit: DEFAULT_QUOTA.limit, quotaDays: DEFAULT_QUOTA.days },
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
      const body = JSON.parse((await readBody(request)) || '{}');
      if (body.mediaType !== 'movie' && body.mediaType !== 'tv') {
        // The request validator refuses the empty body: CSRF protection is off.
        sendJson(response, 400, { message: 'request/body must have required property mediaType' });
        return;
      }
      const type = body.mediaType;
      const mediaId = Number(body.mediaId);
      const open = requests.filter(
        (r) => r.type === type && r.mediaId === mediaId && r.status !== 3,
      );
      const taken = new Set(open.flatMap((r) => r.seasons.map((season) => season.seasonNumber)));
      const seasons = type === 'tv' ? (body.seasons ?? []).filter((n) => !taken.has(n)) : [];
      if ((type === 'movie' && open.length > 0) || (type === 'tv' && seasons.length === 0)) {
        sendJson(response, 409, { message: 'Request for this media already exists.' });
        return;
      }
      const kind = type === 'movie' ? 'movie' : 'tv';
      const part = quotaPart(actor, kind, used[String(actor.id)], requested(actor, kind));
      const needed = type === 'movie' ? 1 : seasons.length;
      if (part.limit > 0 && part.remaining < needed) {
        sendJson(response, 403, {
          message: type === 'movie' ? 'Movie Quota exceeded.' : 'Series Quota exceeded.',
        });
        return;
      }
      const auto = (actor.permissions & (ADMIN | AUTO_APPROVE)) !== 0;
      const now = new Date().toISOString();
      const made = {
        id: nextRequest,
        status: auto ? 2 : 1,
        type,
        mediaId,
        tvdbId: body.tvdbId,
        seasons: seasons.map((seasonNumber, index) => ({
          id: nextRequest * 100 + index,
          seasonNumber,
        })),
        requestedBy: actor.id,
        createdAt: now,
        updatedAt: now,
      };
      nextRequest += 1;
      requests.push(made);
      sendJson(response, 201, requestOf(made));
      return;
    }
    if (method === 'GET' && path === '/request') {
      const take = Number(url.searchParams.get('take') ?? 10);
      const skip = Number(url.searchParams.get('skip') ?? 0);
      const by = url.searchParams.get('requestedBy');
      const list = requests
        .filter((r) => by === null || String(r.requestedBy) === by)
        .toSorted((a, b) => b.updatedAt.localeCompare(a.updatedAt) || b.id - a.id);
      sendJson(response, 200, {
        pageInfo: {
          pages: Math.max(1, Math.ceil(list.length / take)),
          pageSize: take,
          results: list.length,
          page: Math.floor(skip / take) + 1,
        },
        results: list.slice(skip, skip + take).map(requestOf),
      });
      return;
    }
    const one = /^\/request\/(\d+)$/.exec(path);
    if (one) {
      const found = requests.find((r) => String(r.id) === one[1]);
      if (!found) {
        sendJson(response, 404, { message: 'Request not found.' });
        return;
      }
      if (method === 'GET') {
        sendJson(response, 200, requestOf(found));
        return;
      }
      if (method === 'DELETE') {
        if (found.status !== 1 && actor.id !== users[0].id) {
          sendJson(response, 401, {
            message: 'You do not have permission to delete this request.',
          });
          return;
        }
        requests = requests.filter((r) => r !== found);
        response.writeHead(204).end();
        return;
      }
    }
    const retry = /^\/request\/(\d+)\/retry$/.exec(path);
    if (method === 'POST' && retry) {
      const found = requests.find((r) => String(r.id) === retry[1]);
      if (!found) {
        sendJson(response, 404, { message: 'Request not found.' });
        return;
      }
      if (found.status !== 4) {
        sendJson(response, 500, { message: 'Request is not failed.' });
        return;
      }
      found.status = 2;
      sendJson(response, 200, requestOf(found));
      return;
    }
    const title = /^\/(movie|tv)\/(\d+)$/.exec(path);
    if (method === 'GET' && title) {
      const [, type, id] = title;
      const fixture = (type === 'movie' ? MOVIES : SHOWS).find((entry) => String(entry.id) === id);
      if (!fixture) {
        sendJson(response, 404, { message: 'Unable to retrieve media.' });
        return;
      }
      const mediaInfo = mediaOf(type, fixture.id);
      if (type === 'movie') {
        sendJson(response, 200, {
          id: fixture.id,
          title: fixture.en,
          releaseDate: fixture.date,
          externalIds: { imdbId: `tt0${fixture.id}`, tvdbId: null },
          ...(mediaInfo ? { mediaInfo } : {}),
        });
        return;
      }
      sendJson(response, 200, {
        id: fixture.id,
        name: fixture.en,
        firstAirDate: fixture.date,
        seasons: [
          {
            id: fixture.id * 10,
            seasonNumber: 0,
            episodeCount: 2,
            name: 'Specials',
            airDate: fixture.date,
          },
          ...fixture.seasons.map((season) => ({
            id: fixture.id * 10 + season.n,
            seasonNumber: season.n,
            episodeCount: season.episodes,
            name: `Season ${season.n}`,
            airDate: season.date,
          })),
        ],
        externalIds: { imdbId: `tt0${fixture.id}`, tvdbId: fixture.tvdb },
        ...(mediaInfo ? { mediaInfo } : {}),
      });
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
      if (failCreate) {
        sendJson(response, 500, { message: 'Something went wrong.' });
        return;
      }
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
        permissions: DEFAULT_PERMISSIONS,
        hasPassword: String(body.password ?? '') !== '',
      };
      nextId += 1;
      users.push(user);
      // The answer carries no e-mail (I-12).
      sendJson(response, 201, { ...userOf(user, other), email: undefined });
      return;
    }
    const single = /^\/user\/(\d+)(\/.*)?$/.exec(path);
    const user = single ? users.find((entry) => String(entry.id) === single[1]) : undefined;
    if (single && !user) {
      sendJson(response, 404, { message: 'User not found.' });
      return;
    }
    const rest = single?.[2] ?? '';
    if (method === 'GET' && user && rest === '') {
      sendJson(response, 200, userOf(user, other));
      return;
    }
    if (user && rest === '/settings/main') {
      if (method === 'GET') {
        sendJson(response, 200, {
          username: user.displayName,
          email: user.email,
          movieQuotaLimit: user.movieQuotaLimit ?? null,
          movieQuotaDays: user.movieQuotaDays ?? null,
          tvQuotaLimit: user.tvQuotaLimit ?? null,
          tvQuotaDays: user.tvQuotaDays ?? null,
          globalMovieQuotaLimit: DEFAULT_QUOTA.limit,
          globalMovieQuotaDays: DEFAULT_QUOTA.days,
          globalTvQuotaLimit: DEFAULT_QUOTA.limit,
          globalTvQuotaDays: DEFAULT_QUOTA.days,
          locale: '',
          discoverRegion: '',
          streamingRegion: '',
          originalLanguage: '',
          watchlistSyncMovies: false,
          watchlistSyncTv: false,
        });
        return;
      }
      if (method === 'POST') {
        const body = JSON.parse((await readBody(request)) || '{}');
        const email = typeof body.email === 'string' ? body.email.toLowerCase() : undefined;
        if (email && users.some((entry) => entry.id !== user.id && entry.email === email)) {
          sendJson(response, 400, { message: 'INVALID_EMAIL' });
          return;
        }
        if (email) user.email = email;
        if (typeof body.username === 'string' && body.username !== '') {
          user.displayName = body.username;
        }
        for (const key of ['movieQuotaLimit', 'movieQuotaDays', 'tvQuotaLimit', 'tvQuotaDays']) {
          if (key in body) user[key] = body[key] ?? undefined;
        }
        // The answer has no limit fields (I-16).
        sendJson(response, 200, {
          username: user.displayName,
          email: user.email,
          locale: '',
          discoverRegion: '',
          streamingRegion: '',
          originalLanguage: '',
          watchlistSyncMovies: false,
          watchlistSyncTv: false,
        });
        return;
      }
    }
    if (user && rest === '/settings/permissions') {
      if (method === 'POST') {
        const body = JSON.parse((await readBody(request)) || '{}');
        user.permissions = Number(body.permissions ?? 0);
      }
      sendJson(response, 200, { permissions: user.permissions });
      return;
    }
    if (user && rest === '/settings/password') {
      if (method === 'GET') {
        sendJson(response, 200, { hasPassword: user.hasPassword === true });
        return;
      }
      if (method === 'POST') {
        const body = JSON.parse((await readBody(request)) || '{}');
        if (String(body.newPassword ?? '').length < 8) {
          sendJson(response, 400, { message: 'Password must be at least 8 characters.' });
          return;
        }
        user.hasPassword = true;
        response.writeHead(204).end();
        return;
      }
    }
    if (method === 'GET' && user && rest === '/quota') {
      const mine = used[String(user.id)];
      sendJson(response, 200, {
        movie: quotaPart(user, 'movie', mine, requested(user, 'movie')),
        tv: quotaPart(user, 'tv', mine, requested(user, 'tv')),
      });
      return;
    }
    sendJson(response, 404, { message: 'Not found.' });
  };

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    void (async () => {
      if (request.method === 'POST' && url.pathname === '/__control') {
        const body = JSON.parse((await readBody(request)) || '{}');
        if (body.instance !== undefined && body.instance !== 'other' && body.instance !== 'same') {
          response.writeHead(400).end();
          return;
        }
        if (body.instance !== undefined) other = body.instance === 'other';
        if (typeof body.failCreate === 'boolean') failCreate = body.failCreate;
        if (body.used && typeof body.used === 'object') used = { ...used, ...body.used };
        if (body.request && typeof body.request === 'object') {
          const found = requests.find((r) => r.id === Number(body.request.id));
          if (!found) {
            response.writeHead(404).end();
            return;
          }
          found.status = Number(body.request.status);
          found.updatedAt = new Date().toISOString();
        }
        response.writeHead(204).end();
        return;
      }
      if (request.method === 'GET' && url.pathname === '/__requests') {
        sendJson(response, 200, requests);
        return;
      }
      if (request.method === 'POST' && url.pathname === '/__reset') {
        users = USERS.map((user) => ({ ...user }));
        other = false;
        // User ids go on counting too: the server keeps the users it opened (the shared user
        // among them), and a new user with an old id would collide with them.
        failCreate = false;
        used = {};
        // Request ids go on counting, as Seerr's do: the server keeps the ids of earlier wishes.
        requests = [];
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
