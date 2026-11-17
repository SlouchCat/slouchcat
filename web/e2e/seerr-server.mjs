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
//   GET  /api/v1/settings/radarr     one Radarr server (I-9): the Radarr mock of arr-server.mjs
//   GET  /api/v1/settings/sonarr     one Sonarr server: the Sonarr mock (stage 4, profile tags
//                                    and progress), with their key ARR_API_KEY
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
//   POST /api/v1/request/{id}/approve|decline   a pending request (status 1) approved (2) or
//                                    declined (3), 200 with the request; any other 409 «Only
//                                    pending requests can be approved or declined.» (stage 4)
//   GET  /api/v1/movie/{id}, /tv/{id}  the titles of tmdb-server.mjs with mediaInfo and, for a
//                                    series, the seasons Seerr can order (season 0 included)
//   GET  /api/v1/discover/trending, /discover/movies, /discover/tv, /discover/movies/upcoming,
//        /discover/tv/upcoming, /search    {page, totalPages, totalResults, results} (I-43) over the
//                                    titles of tmdb-server.mjs and a few TMDB does not know
//                                    (DISCOVER_UNKNOWN); only the parameters Seerr's validator
//                                    knows on the route, any other 400 «Unknown query parameter»
//                                    (I-44); `language` on /discover/movies|tv filters the original
//                                    language (I-60); page above 500 answers 500 (I-67); 6 a page
//   GET  /api/v1/discover/genreslider/movie|tv   [{id, name, backdrops}] (I-45)
//   GET  /api/v1/genres/movie|tv     [{id, name}] (I-45)
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
//   GET  /api/v1/user/{id}/requests?take=&skip=   {pageInfo, results} of that user, newest first
//                                    (the count before «Anfragen übernehmen», stage 4)
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
import { ARR_API_KEY } from './arr-server.mjs';
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
  apiKey: ARR_API_KEY,
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

// The wishes area (concept 25 stage 5): what the discover and search routes list. Titles of the
// TMDB mock, in Seerr's order of popularity, with the votes the sort orders filter by (I-64,
// I-65); `seerr` marks a title another Seerr user requested (mediaInfo, «Bereits angefordert»).
const DISCOVER_FACTS = {
  9001: { votes: 7.1, count: 900 },
  9005: { votes: 6.4, count: 700, seerr: 2 },
  9002: { votes: 6.9, count: 640 },
  9006: { votes: 7.8, count: 820 },
  9003: { votes: 8.2, count: 40 },
  9004: { votes: 0, count: 0 },
  9100: { votes: 7.4, count: 400 },
  9101: { votes: 6.8, count: 260 },
  9102: { votes: 0, count: 0 },
};

/** Titles TMDB does not know: Slouchcat keeps Seerr's text and has no image (the placeholder). */
export const DISCOVER_UNKNOWN = [
  { id: 9901, type: 'movie', name: 'Paper Lantern', date: '2023-04-14', votes: 6.1, count: 120 },
  { id: 9902, type: 'movie', name: 'Iron Orchard', date: '2022-08-30', votes: 5.9, count: 80 },
  { id: 9903, type: 'movie', name: 'Quiet Harbor', date: '2021-02-11', votes: 6.6, count: 300 },
  { id: 9904, type: 'movie', name: 'Amber Road', date: '2027-05-21', votes: 0, count: 0 },
  { id: 9951, type: 'tv', name: 'Copper Valley', date: '2020-10-02', votes: 7.0, count: 90 },
];

/** A person among the search hits, as Seerr lists them; Slouchcat never shows one (N-53). */
const PERSON = { id: 7001, mediaType: 'person', name: 'Nora Nebelhaus', profilePath: null };

const DISCOVER_PAGE = 6;

/** The parameters Seerr's validator knows per route (server/internal/seerr/calls.go). */
const DISCOVER_PARAMS = {
  '/discover/trending': ['page', 'language', 'mediaType', 'timeWindow'],
  '/discover/movies': [
    'page',
    'sortBy',
    'genre',
    'studio',
    'keywords',
    'language',
    'primaryReleaseDateGte',
    'primaryReleaseDateLte',
    'withRuntimeGte',
    'withRuntimeLte',
    'voteAverageGte',
    'voteAverageLte',
    'voteCountGte',
    'voteCountLte',
    'watchProviders',
    'watchRegion',
    'certification',
    'certificationGte',
    'certificationLte',
    'certificationCountry',
  ],
  '/discover/tv': [
    'page',
    'sortBy',
    'genre',
    'network',
    'keywords',
    'language',
    'firstAirDateGte',
    'firstAirDateLte',
    'withRuntimeGte',
    'withRuntimeLte',
    'voteAverageGte',
    'voteAverageLte',
    'voteCountGte',
    'voteCountLte',
    'watchProviders',
    'watchRegion',
    'status',
    'certification',
    'certificationGte',
    'certificationLte',
    'certificationCountry',
  ],
  '/discover/movies/upcoming': ['page', 'language'],
  '/discover/tv/upcoming': ['page', 'language'],
  '/discover/genreslider/movie': ['language'],
  '/discover/genreslider/tv': ['language'],
  '/genres/movie': ['language'],
  '/genres/tv': ['language'],
  '/search': ['query', 'page', 'language'],
};

const GENRE_SLIDES = {
  movie: [
    { id: 18, de: 'Drama', en: 'Drama', backdrops: ['/genre-18.jpg'] },
    { id: 12, de: 'Abenteuer', en: 'Adventure', backdrops: ['/genre-12.jpg'] },
    { id: 99, de: 'Dokumentarfilm', en: 'Documentary', backdrops: [] },
  ],
  tv: [
    { id: 80, de: 'Krimi', en: 'Crime', backdrops: ['/genre-80.jpg'] },
    {
      id: 10765,
      de: 'Sci-Fi & Fantasy',
      en: 'Sci-Fi & Fantasy',
      backdrops: ['/genre-10765.jpg'],
    },
  ],
};

const GENRES = {
  movie: [
    { id: 12, de: 'Abenteuer', en: 'Adventure' },
    { id: 35, de: 'Komödie', en: 'Comedy' },
    { id: 99, de: 'Dokumentarfilm', en: 'Documentary' },
    { id: 18, de: 'Drama', en: 'Drama' },
  ],
  tv: [
    { id: 18, de: 'Drama', en: 'Drama' },
    { id: 80, de: 'Krimi', en: 'Crime' },
    { id: 10765, de: 'Sci-Fi & Fantasy', en: 'Sci-Fi & Fantasy' },
  ],
};

/** Every title the discover routes know, as `{type, id, name: {de, en}, date, genres, …}`. */
const DISCOVER_TITLES = [
  ...MOVIES.map((movie) => ({ type: 'movie', fixture: movie })),
  ...SHOWS.map((show) => ({ type: 'tv', fixture: show })),
]
  .map(({ type, fixture }) => ({
    type,
    id: fixture.id,
    names: { de: fixture.de, en: fixture.en },
    date: fixture.date,
    genres: fixture.genres.map((genre) => genre.id),
    tvdb: fixture.tvdb,
    ...DISCOVER_FACTS[fixture.id],
  }))
  .concat(
    DISCOVER_UNKNOWN.map((title) => ({
      ...title,
      names: { de: title.name, en: title.name },
      genres: [18],
    })),
  )
  .toSorted((a, b) => b.count - a.count || a.id - b.id);

/**
 * Starts the Seerr mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_SEERR_PORT, default 8086); its Radarr
 * and Sonarr point at `radarrPort` and `sonarrPort` (the mocks of arr-server.mjs). Resolves with
 * its port and a `close` function.
 */
export function startSeerrServer({
  port = Number(process.env.SLOUCHCAT_E2E_SEERR_PORT ?? 8086),
  radarrPort = 7878,
  sonarrPort = 8989,
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

  /** One result of a list (I-43): mediaInfo from the requests of the mock, else the fixture's. */
  const resultOf = (title, language) => {
    const request = mediaOf(title.type, title.id);
    const mediaInfo =
      request ??
      (title.seerr ? { tmdbId: title.id, mediaType: title.type, status: title.seerr } : undefined);
    const name = title.names[language] ?? title.names.en;
    return {
      id: title.id,
      mediaType: title.type,
      ...(title.type === 'movie'
        ? { title: name, originalTitle: title.names.en, releaseDate: title.date }
        : { name, originalName: title.names.en, firstAirDate: title.date }),
      overview: '',
      posterPath: `/${title.type}-${title.id}-poster.png`,
      backdropPath: `/${title.type}-${title.id}-backdrop.png`,
      genreIds: title.genres,
      originalLanguage: 'en',
      voteAverage: title.votes,
      voteCount: title.count,
      popularity: title.count / 10,
      ...(mediaInfo ? { mediaInfo } : {}),
    };
  };

  /** The discover and search routes (concept 25 stage 5, appendix I sections 4 and 4a). */
  const discover = (path, url, response) => {
    const query = url.searchParams;
    for (const key of query.keys()) {
      if (!DISCOVER_PARAMS[path].includes(key)) {
        sendJson(response, 400, { message: `Unknown query parameter '${key}'` });
        return;
      }
    }
    const page = Number(query.get('page') ?? 1);
    if (!Number.isInteger(page) || page < 1) {
      sendJson(response, 400, { message: 'request/query/page must be >= 1' });
      return;
    }
    if (page > 500) {
      sendJson(response, 500, { status: 500, message: 'Unable to retrieve popular movies.' });
      return;
    }
    const language = (query.get('language') ?? 'en').startsWith('de') ? 'de' : 'en';
    const slider = /^\/discover\/genreslider\/(movie|tv)$/.exec(path);
    if (slider) {
      sendJson(
        response,
        200,
        GENRE_SLIDES[slider[1]].map((genre) => ({
          id: genre.id,
          name: genre[language],
          backdrops: genre.backdrops,
        })),
      );
      return;
    }
    const genres = /^\/genres\/(movie|tv)$/.exec(path);
    if (genres) {
      sendJson(
        response,
        200,
        GENRES[genres[1]].map((genre) => ({ id: genre.id, name: genre[language] })),
      );
      return;
    }
    const today = new Date().toISOString().slice(0, 10);
    let list = DISCOVER_TITLES;
    if (path === '/discover/trending') {
      list = list.filter((title) => title.date <= today);
    } else if (path === '/discover/movies/upcoming' || path === '/discover/tv/upcoming') {
      const type = path.startsWith('/discover/movies') ? 'movie' : 'tv';
      list = list.filter((title) => title.type === type && title.date > today);
    } else if (path === '/discover/movies' || path === '/discover/tv') {
      const type = path === '/discover/movies' ? 'movie' : 'tv';
      list = list.filter((title) => title.type === type);
      // `language` is a filter on the original language here, not the display language (I-60).
      if (query.has('language')) list = list.filter(() => query.get('language') === 'en');
      const genre = (query.get('genre') ?? '').split(',').filter(Boolean).map(Number);
      list = list.filter((title) => genre.every((id) => title.genres.includes(id)));
      const before = query.get(type === 'movie' ? 'primaryReleaseDateLte' : 'firstAirDateLte');
      if (before) list = list.filter((title) => title.date <= before);
      const votes = Number(query.get('voteCountGte') ?? 0);
      list = list.filter((title) => title.count >= votes);
      // An unknown order gives the list by popularity (I-68).
      const sortBy = query.get('sortBy') ?? 'popularity.desc';
      if (sortBy === 'primary_release_date.desc' || sortBy === 'first_air_date.desc') {
        list = list.toSorted((a, b) => b.date.localeCompare(a.date));
      } else if (sortBy === 'vote_average.desc') {
        list = list.toSorted((a, b) => b.votes - a.votes);
      }
    } else if (path === '/search') {
      const text = (query.get('query') ?? '').trim().toLowerCase();
      const id = /^(tmdb|tvdb|imdb):(\S+)$/.exec(text);
      const found = id
        ? list.filter((title) =>
            id[1] === 'tmdb'
              ? String(title.id) === id[2]
              : id[1] === 'tvdb'
                ? String(title.tvdb ?? '') === id[2]
                : `tt0${title.id}` === id[2],
          )
        : list.filter(
            (title) =>
              text !== '' &&
              Object.values(title.names).some((name) => name.toLowerCase().includes(text)),
          );
      const person =
        (id?.[1] === 'tmdb' && found.length > 0) ||
        (text !== '' && PERSON.name.toLowerCase().includes(text));
      const results = [
        ...found.map((title) => resultOf(title, language)),
        ...(person ? [PERSON] : []),
      ];
      sendJson(response, 200, {
        page: 1,
        totalPages: 1,
        totalResults: results.length,
        results: page === 1 ? results : [],
      });
      return;
    }
    const totalPages = Math.max(1, Math.ceil(list.length / DISCOVER_PAGE));
    sendJson(response, 200, {
      page,
      totalPages,
      totalResults: list.length,
      results: list
        .slice((page - 1) * DISCOVER_PAGE, page * DISCOVER_PAGE)
        .map((title) => resultOf(title, language)),
    });
  };

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
      sendJson(response, 200, [dvr(0, 'Radarr', radarrPort)]);
      return;
    }
    if (method === 'GET' && path === '/settings/sonarr') {
      sendJson(response, 200, [dvr(0, 'Sonarr', sonarrPort)]);
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
    const decide = /^\/request\/(\d+)\/(approve|decline)$/.exec(path);
    if (method === 'POST' && decide) {
      const found = requests.find((r) => String(r.id) === decide[1]);
      if (!found) {
        sendJson(response, 404, { message: 'Request not found.' });
        return;
      }
      if (found.status !== 1) {
        sendJson(response, 409, { message: 'Only pending requests can be approved or declined.' });
        return;
      }
      found.status = decide[2] === 'approve' ? 2 : 3;
      found.updatedAt = new Date().toISOString();
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
    if (method === 'GET' && path in DISCOVER_PARAMS) {
      discover(path, url, response);
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
    if (method === 'GET' && user && rest === '/requests') {
      const take = Number(url.searchParams.get('take') ?? 10);
      const skip = Number(url.searchParams.get('skip') ?? 0);
      const list = requests
        .filter((r) => r.requestedBy === user.id)
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
