// Local Radarr and Sonarr for the Playwright suite (concept 25 stage 4): the profile tags and the
// download progress of the wishes. start-server.mjs starts one of each next to the Slouchcat binary
// (thirteen and fourteen below the app's port) and the Seerr mock names them under
// /settings/radarr and /settings/sonarr, so the server takes address and key from Seerr as with
// real instances. `node e2e/arr-server.mjs` runs both alone. The answers have the shapes Radarr 6.4.4
// and Sonarr 4.0.20 gave in the check of the real instances (appendix I section 5) and that the
// fake of the server's tests models (server/internal/arr/arrtest).
//
// Every /api/v3 call needs `X-Api-Key: ARR_API_KEY`, else 401 with an empty body.
//
//   GET  /api/v3/tag                  every tag {id, label}
//   POST /api/v3/tag {label}          idempotent, lower-cases the label; Radarr refuses labels
//                                     outside ^[a-z0-9-]+$ with 400 and a validation array (I-52)
//   GET  /api/v3/movie?tmdbId=        Radarr: the film with that TMDB id, Radarr id = TMDB id; [] for
//                                     a film the control made absent
//   GET  /api/v3/series?tvdbId=       Sonarr: the series with that TVDB id, Sonarr id = TVDB id, with
//                                     seasons 1 to 3 and their statistics
//   PUT  /api/v3/movie/editor, /series/editor {movieIds|seriesIds, tags, applyTags}   adds the tags;
//                                     a title of `failEditor` answers 400 with a validation array
//   GET  /api/v3/queue?page=&pageSize=   the queue in pages
//   POST /__control                   {"absent": [tmdbId|tvdbId]} titles the service does not have,
//                                     {"queue": [{"title": id, "state": "downloading"|"stalled",
//                                     "season"?: n}]} the queue (one entry each), {"failEditor": [id]}
//                                     titles the editor refuses, {"files": {"<id>": [season, done,
//                                     total]}} the episode files of a series season
//   GET  /__calls                     the calls so far, by method and route
//   POST /__reset                     no tags, no queue, every title present, no failure
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

/** The api key of both services, as Seerr hands it out (a fake; the log scan looks for it). */
export const ARR_API_KEY = 'e2e-arr-key-9b41d7c2e6f0';

const RADARR_LABEL = /^[a-z0-9-]+$/;

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

/** A queue record of Radarr (I-56) or Sonarr (documentation, I-57) for the title `id`. */
function record(kind, entry, index) {
  const own =
    kind === 'radarr'
      ? { movieId: entry.title }
      : {
          seriesId: entry.title,
          episodeId: entry.title * 100 + index,
          seasonNumber: entry.season ?? 1,
        };
  if (entry.state === 'stalled') {
    return {
      id: 500 + index,
      ...own,
      title: `Title.${entry.title}.2160p`,
      size: 5000,
      sizeleft: 0,
      timeleft: '00:00:00',
      status: 'completed',
      trackedDownloadState: 'importBlocked',
      trackedDownloadStatus: 'warning',
      errorMessage: '',
      statusMessages: [
        {
          title: `Title.${entry.title}.2160p`,
          messages: ['No files found are eligible for import'],
        },
      ],
      downloadClient: 'SABnzbd',
      protocol: 'usenet',
      outputPath: `/downloads/complete/Title.${entry.title}`,
    };
  }
  // 45 % done with 12 minutes left, as the first picture of I-56.
  return {
    id: 500 + index,
    ...own,
    title: `Title.${entry.title}.1080p`,
    size: 1000,
    sizeleft: 550,
    timeleft: '00:12:00',
    status: 'downloading',
    trackedDownloadState: 'downloading',
    trackedDownloadStatus: 'ok',
    statusMessages: [],
    downloadClient: 'SABnzbd',
    protocol: 'usenet',
    outputPath: `/downloads/incomplete/Title.${entry.title}`,
  };
}

/**
 * Starts a mock of `kind` ('radarr' or 'sonarr') on 127.0.0.1:`port`; resolves with its port and a
 * `close` function.
 */
export function startArrServer({ kind, port }) {
  let tags = [];
  let nextTag = 1;
  /** Tag ids per title id. */
  let titleTags = new Map();
  let absent = new Set();
  let queue = [];
  let failEditor = new Set();
  /** Episode files of a season: `${title}:${season}` → [done, total]. */
  let files = new Map();
  let calls = [];

  const movie = (tmdbId) => ({
    id: tmdbId,
    tmdbId,
    title: `Film ${tmdbId}`,
    tags: titleTags.get(tmdbId) ?? [],
    hasFile: false,
    monitored: true,
    path: `/movies/Film ${tmdbId}`,
  });

  const series = (tvdbId) => {
    const seasons = [1, 2, 3].map((seasonNumber) => {
      const [done, total] = files.get(`${tvdbId}:${seasonNumber}`) ?? [0, 10];
      return {
        seasonNumber,
        monitored: true,
        statistics: { episodeFileCount: done, episodeCount: total, totalEpisodeCount: total },
      };
    });
    const sum = (field) => seasons.reduce((total, season) => total + season.statistics[field], 0);
    return {
      id: tvdbId,
      tvdbId,
      title: `Series ${tvdbId}`,
      tags: titleTags.get(tvdbId) ?? [],
      path: `/tv/Series ${tvdbId}`,
      seasons,
      statistics: {
        seasonCount: seasons.length,
        episodeFileCount: sum('episodeFileCount'),
        episodeCount: sum('episodeCount'),
        totalEpisodeCount: sum('totalEpisodeCount'),
      },
    };
  };

  const routes = async (request, url, response) => {
    const method = request.method ?? 'GET';
    const path = url.pathname;
    calls.push(`${method} ${path}`);
    if (request.headers['x-api-key'] !== ARR_API_KEY) {
      response.writeHead(401).end();
      return;
    }
    if (path === '/api/v3/tag' && method === 'GET') {
      sendJson(response, 200, tags);
      return;
    }
    if (path === '/api/v3/tag' && method === 'POST') {
      const body = JSON.parse((await readBody(request)) || '{}');
      const label = String(body.label ?? '')
        .trim()
        .toLowerCase();
      if (label === '' || (kind === 'radarr' && !RADARR_LABEL.test(label))) {
        sendJson(response, 400, [
          {
            propertyName: 'Label',
            errorMessage: 'Allowed characters a-z, 0-9 and -',
            attemptedValue: body.label,
            severity: 'error',
          },
        ]);
        return;
      }
      let tag = tags.find((entry) => entry.label === label);
      if (!tag) {
        tag = { id: nextTag, label };
        nextTag += 1;
        tags.push(tag);
      }
      sendJson(response, 201, tag);
      return;
    }
    if (kind === 'radarr' && path === '/api/v3/movie' && method === 'GET') {
      const id = Number(url.searchParams.get('tmdbId'));
      sendJson(response, 200, id > 0 && !absent.has(id) ? [movie(id)] : []);
      return;
    }
    if (kind === 'sonarr' && path === '/api/v3/series' && method === 'GET') {
      const id = Number(url.searchParams.get('tvdbId'));
      sendJson(response, 200, id > 0 && !absent.has(id) ? [series(id)] : []);
      return;
    }
    const editor = kind === 'radarr' ? '/api/v3/movie/editor' : '/api/v3/series/editor';
    if (path === editor && method === 'PUT') {
      const body = JSON.parse((await readBody(request)) || '{}');
      const ids = (kind === 'radarr' ? body.movieIds : body.seriesIds) ?? [];
      if (ids.some((id) => failEditor.has(id))) {
        sendJson(response, 400, [
          {
            propertyName: 'Tags',
            errorMessage: 'The tag could not be applied.',
            severity: 'error',
          },
        ]);
        return;
      }
      for (const id of ids) {
        const own = titleTags.get(id) ?? [];
        titleTags.set(id, [...new Set([...own, ...(body.tags ?? [])])]);
      }
      sendJson(
        response,
        202,
        ids.map((id) => (kind === 'radarr' ? movie(id) : series(id))),
      );
      return;
    }
    if (path === '/api/v3/queue' && method === 'GET') {
      const page = Math.max(1, Number(url.searchParams.get('page') ?? 1));
      const size = Math.max(1, Number(url.searchParams.get('pageSize') ?? 10));
      const records = queue.map((entry, index) => record(kind, entry, index));
      sendJson(response, 200, {
        page,
        pageSize: size,
        sortKey: 'timeleft',
        sortDirection: 'ascending',
        totalRecords: records.length,
        records: records.slice((page - 1) * size, page * size),
      });
      return;
    }
    sendJson(response, 404, { message: 'NotFound' });
  };

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    void (async () => {
      if (request.method === 'POST' && url.pathname === '/__control') {
        const body = JSON.parse((await readBody(request)) || '{}');
        if (Array.isArray(body.absent)) absent = new Set(body.absent.map(Number));
        if (Array.isArray(body.queue)) queue = body.queue;
        if (Array.isArray(body.failEditor)) failEditor = new Set(body.failEditor.map(Number));
        if (body.files && typeof body.files === 'object') {
          for (const [id, [season, done, total]] of Object.entries(body.files)) {
            files.set(`${id}:${season}`, [done, total]);
          }
        }
        response.writeHead(204).end();
        return;
      }
      if (request.method === 'GET' && url.pathname === '/__calls') {
        sendJson(response, 200, calls);
        return;
      }
      if (request.method === 'POST' && url.pathname === '/__reset') {
        tags = [];
        titleTags = new Map();
        absent = new Set();
        queue = [];
        failEditor = new Set();
        files = new Map();
        calls = [];
        response.writeHead(204).end();
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
  const radarr = await startArrServer({ kind: 'radarr', port: 8084 });
  const sonarr = await startArrServer({ kind: 'sonarr', port: 8083 });
  console.log(`radarr mock on :${radarr.port}, sonarr mock on :${sonarr.port}`);
}
