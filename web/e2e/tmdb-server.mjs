// Local TMDB for the Playwright suite (concept 25 stage 2): the titles outside the library that
// the preview page, the request dialog and the search section «Noch nicht in der Bibliothek» show.
// start-server.mjs starts it next to the Slouchcat binary and hands its address to the server as
// SLOUCHCAT_TEST_TMDB_BASE_URL (honoured with --dev only). The server asks TMDB only with a key,
// and the suite runs without one on purpose (library titles come from the file names): a spec
// that needs these titles stores the key TMDB_API_KEY through the administration
// (`enableTmdb` in e2e/wishes.ts) and clears it again afterwards. `node e2e/tmdb-server.mjs`
// runs the mock alone.
//
// The API answers with and without the `/3` of api.themoviedb.org; the images with and without
// `/t/p` of image.tmdb.org. Every call needs `Authorization: Bearer TMDB_API_KEY` or
// `api_key=TMDB_API_KEY`, else 401 as TMDB answers a wrong key.
//
//   GET /configuration                     the key check
//   GET /search/movie|tv?query=            the fixtures whose German or English title contains
//                                          the query (case-insensitive); nothing else
//   GET /movie/{id}?language=&append_to_response=release_dates,credits,images
//   GET /tv/{id}?language=&append_to_response=content_ratings,external_ids,credits,images
//   GET /movie|tv/{id}/videos              one YouTube trailer per fixture
//   GET /{size}/{file}.png                 a small generated picture (poster 2:3, backdrop 16:9)
//   GET /{size}/genre-{id}.jpg             a small JPEG: the backdrop of a genre tile (the server
//                                          keeps only JPEGs there, as TMDB serves them)
//   GET /__stats                           {calls}: how many API calls the mock answered
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';
import { deflateSync } from 'node:zlib';

/** The key the specs store (a fake; the log scan of global-teardown.ts looks for it). */
export const TMDB_API_KEY = 'e2e-tmdb-key-8c41d0a7f2b9';

/**
 * The titles outside the library. Ids from 9000 up, far from any real use in the suite; neutral
 * made-up names. `seerr-server.mjs` knows the same ids for its /movie and /tv routes.
 */
export const MOVIES = [
  {
    id: 9001,
    de: 'Glasvogel',
    en: 'Glass Bird',
    date: '2026-03-12',
    runtime: 101,
    cert: 'FSK 12',
    genres: [{ id: 18, de: 'Drama', en: 'Drama' }],
    color: [70, 110, 160],
  },
  {
    id: 9002,
    de: 'Kupfermond',
    en: 'Copper Moon',
    date: '2025-10-01',
    runtime: 94,
    cert: '6',
    genres: [{ id: 12, de: 'Abenteuer', en: 'Adventure' }],
    color: [170, 110, 60],
  },
  {
    id: 9003,
    de: 'Glasfabrik',
    en: 'Glass Works',
    date: '2024-05-20',
    runtime: 88,
    cert: '0',
    genres: [{ id: 99, de: 'Dokumentarfilm', en: 'Documentary' }],
    color: [90, 150, 120],
  },
  // The wishes area (concept 25 stage 5): further titles of the discover lists of seerr-server.mjs.
  {
    id: 9004,
    de: 'Sternenfeld',
    en: 'Star Field',
    date: '2027-03-05',
    runtime: 112,
    cert: '12',
    genres: [{ id: 12, de: 'Abenteuer', en: 'Adventure' }],
    color: [40, 60, 120],
  },
  {
    id: 9005,
    de: 'Rotes Ufer',
    en: 'Red Shore',
    date: '2025-06-01',
    runtime: 97,
    cert: '12',
    genres: [{ id: 18, de: 'Drama', en: 'Drama' }],
    color: [150, 60, 60],
  },
  {
    id: 9006,
    de: 'Winterhaus',
    en: 'Winter House',
    date: '2024-11-20',
    runtime: 105,
    cert: '6',
    genres: [{ id: 18, de: 'Drama', en: 'Drama' }],
    color: [140, 160, 180],
  },
];

export const SHOWS = [
  {
    id: 9100,
    de: 'Nebelhafen',
    en: 'Fog Harbour',
    date: '2023-09-02',
    runtime: 48,
    cert: '12',
    tvdb: 79100,
    seasons: [
      { n: 1, episodes: 8, date: '2023-09-02' },
      { n: 2, episodes: 10, date: '2024-09-07' },
      { n: 3, episodes: 10, date: '2025-09-06' },
    ],
    next: { season: 4, episode: 1, date: '2026-12-04' },
    genres: [{ id: 80, de: 'Krimi', en: 'Crime' }],
    color: [60, 80, 100],
  },
  {
    id: 9101,
    de: 'Glasküste',
    en: 'Glass Coast',
    date: '2022-01-15',
    runtime: 42,
    cert: '6',
    tvdb: 79101,
    seasons: [
      { n: 1, episodes: 6, date: '2022-01-15' },
      { n: 2, episodes: 6, date: '2023-01-14' },
    ],
    genres: [{ id: 10765, de: 'Sci-Fi & Fantasy', en: 'Sci-Fi & Fantasy' }],
    color: [120, 90, 150],
  },
  {
    id: 9102,
    de: 'Lichtturm',
    en: 'Light Tower',
    date: '2027-02-01',
    runtime: 50,
    cert: '12',
    tvdb: 79102,
    seasons: [{ n: 1, episodes: 8, date: '2027-02-01' }],
    genres: [{ id: 18, de: 'Drama', en: 'Drama' }],
    color: [170, 150, 80],
  },
];

/** A JPEG of 64 × 36 in one colour, for the genre backdrops of seerr-server.mjs. */
const GENRE_JPEG = Buffer.from(
  '/9j/4AAQSkZJRgABAgAAAQABAAD//gAQTGF2YzYwLjMxLjEwMgD/2wBDAAgQEBMQExYWFhYWFhoYGhsbGxoaGhobGxsdHR0iIiIdHR0bGx0dICAiIiUmJSMjIiMmJigoKDAwLi44ODpFRVP/xABNAAEBAAAAAAAAAAAAAAAAAAAABgEBAQEAAAAAAAAAAAAAAAAAAAQFEAEAAAAAAAAAAAAAAAAAAAAAEQEAAAAAAAAAAAAAAAAAAAAA/8AAEQgAJABAAwEiAAIRAAMRAP/aAAwDAQACEQMRAD8AnQGglAAAAAAAAAAAAAAAAAAAAAAf/9k=',
  'base64',
);

// A PNG of one colour: the image pipeline decodes, scales and stores it like a real one.
const CRC_TABLE = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});
const crc32 = (buffer) => {
  let c = 0xffffffff;
  for (const byte of buffer) c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
};
const chunk = (type, data) => {
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type, 'ascii'), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(body));
  return Buffer.concat([length, body, crc]);
};
function png(width, height, [r, g, b]) {
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header.set([8, 2, 0, 0, 0], 8); // 8 bit, RGB
  const row = Buffer.alloc(1 + width * 3);
  for (let x = 0; x < width; x += 1) row.set([r, g, b], 1 + x * 3);
  const raw = Buffer.concat(Array.from({ length: height }, () => row));
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

const lang = (url) => ((url.searchParams.get('language') ?? 'en').startsWith('de') ? 'de' : 'en');
const yearOf = (date) => Number(date.slice(0, 4));
const posterPath = (kind, title) => `/${kind}-${title.id}-poster.png`;
const backdropPath = (kind, title) => `/${kind}-${title.id}-backdrop.png`;

function images(kind, title) {
  const image = (path, width, height) => ({
    file_path: path,
    iso_639_1: null,
    width,
    height,
    vote_average: 5,
    vote_count: 10,
  });
  return {
    posters: [image(posterPath(kind, title), 800, 1200)],
    backdrops: [image(backdropPath(kind, title), 1920, 1080)],
    logos: [],
  };
}

function movieOf(movie, language, append) {
  return {
    id: movie.id,
    imdb_id: `tt0${movie.id}`,
    title: movie[language],
    original_title: movie.en,
    original_language: 'en',
    overview: `${movie[language]}: ${language === 'de' ? 'eine erfundene Geschichte für die Tests.' : 'a made-up story for the tests.'}`,
    tagline: '',
    release_date: movie.date,
    status: 'Released',
    runtime: movie.runtime,
    vote_average: 7.1,
    genres: movie.genres.map((genre) => ({ id: genre.id, name: genre[language] })),
    poster_path: posterPath('movie', movie),
    backdrop_path: backdropPath('movie', movie),
    release_dates: {
      results: [
        {
          iso_3166_1: 'DE',
          release_dates: [
            { certification: movie.cert, type: 3, release_date: `${movie.date}T00:00:00.000Z` },
          ],
        },
        {
          iso_3166_1: 'CH',
          release_dates: [
            {
              certification: movie.cert.replace('FSK ', ''),
              type: 3,
              release_date: `${movie.date}T00:00:00.000Z`,
            },
          ],
        },
      ],
    },
    credits: { cast: [], crew: [] },
    ...(append.includes('images') ? { images: images('movie', movie) } : {}),
    belongs_to_collection: null,
  };
}

function showOf(show, language, append) {
  const episode = (season, number, date) => ({
    season_number: season,
    episode_number: number,
    name: `${season}x${number}`,
    overview: '',
    air_date: date,
    runtime: show.runtime,
    still_path: '',
  });
  const last = show.seasons.at(-1);
  return {
    id: show.id,
    name: show[language],
    original_name: show.en,
    original_language: 'en',
    overview: `${show[language]}: ${language === 'de' ? 'eine erfundene Serie für die Tests.' : 'a made-up series for the tests.'}`,
    tagline: '',
    first_air_date: show.date,
    status: show.next ? 'Returning Series' : 'Ended',
    episode_run_time: [show.runtime],
    vote_average: 7.4,
    genres: show.genres.map((genre) => ({ id: genre.id, name: genre[language] })),
    poster_path: posterPath('tv', show),
    backdrop_path: backdropPath('tv', show),
    number_of_seasons: show.seasons.length,
    seasons: [
      // Specials, which the server leaves out (season 0).
      {
        season_number: 0,
        episode_count: 2,
        air_date: show.date,
        name: 'Specials',
        poster_path: '',
      },
      ...show.seasons.map((season) => ({
        season_number: season.n,
        episode_count: season.episodes,
        air_date: season.date,
        name: `${language === 'de' ? 'Staffel' : 'Season'} ${season.n}`,
        poster_path: '',
      })),
    ],
    last_episode_to_air: episode(last.n, last.episodes, last.date),
    next_episode_to_air: show.next
      ? episode(show.next.season, show.next.episode, show.next.date)
      : null,
    created_by: [],
    content_ratings: {
      results: [
        { iso_3166_1: 'DE', rating: show.cert },
        { iso_3166_1: 'CH', rating: show.cert },
      ],
    },
    external_ids: { imdb_id: `tt0${show.id}`, tvdb_id: show.tvdb },
    credits: { cast: [], crew: [] },
    ...(append.includes('images') ? { images: images('tv', show) } : {}),
  };
}

const searchHit = (kind, title) =>
  kind === 'movie'
    ? {
        id: title.id,
        title: title.en,
        original_title: title.en,
        release_date: title.date,
        overview: '',
        poster_path: posterPath('movie', title),
      }
    : {
        id: title.id,
        name: title.en,
        original_name: title.en,
        first_air_date: title.date,
        overview: '',
        poster_path: posterPath('tv', title),
      };

const sendJson = (response, status, body) => {
  response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
  response.end(JSON.stringify(body));
};

/**
 * Starts the TMDB mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_TMDB_PORT, default 8085); resolves with
 * its port and a `close` function.
 */
export function startTmdbServer({
  port = Number(process.env.SLOUCHCAT_E2E_TMDB_PORT ?? 8085),
} = {}) {
  let calls = 0;
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    if (url.pathname === '/__stats') {
      sendJson(response, 200, { calls });
      return;
    }
    if (request.method === 'GET' && /\/genre-\d+\.jpg$/.test(url.pathname)) {
      response.writeHead(200, {
        'Content-Type': 'image/jpeg',
        'Content-Length': GENRE_JPEG.length,
      });
      response.end(GENRE_JPEG);
      return;
    }
    // Images: /t/p/{size}/{file}.png or /{size}/{file}.png, no key (TMDB's CDN asks for none).
    const picture = /\/(movie|tv)-(\d+)-(poster|backdrop)\.png$/.exec(url.pathname);
    if (request.method === 'GET' && picture) {
      const [, kind, id, role] = picture;
      const title = (kind === 'movie' ? MOVIES : SHOWS).find((entry) => String(entry.id) === id);
      if (!title) {
        response.writeHead(404).end();
        return;
      }
      // Big enough for every size the server keeps (posters to w800, backdrops to w1920): a
      // picture is never scaled up.
      const body = role === 'poster' ? png(800, 1200, title.color) : png(1920, 1080, title.color);
      response.writeHead(200, { 'Content-Type': 'image/png', 'Content-Length': body.length });
      response.end(body);
      return;
    }
    const path = url.pathname.replace(/^\/3(?=\/)/, '');
    const auth = request.headers.authorization ?? '';
    if (auth !== `Bearer ${TMDB_API_KEY}` && url.searchParams.get('api_key') !== TMDB_API_KEY) {
      sendJson(response, 401, {
        status_code: 7,
        status_message: 'Invalid API key: You must be granted a valid key.',
        success: false,
      });
      return;
    }
    calls += 1;
    const language = lang(url);
    const append = (url.searchParams.get('append_to_response') ?? '').split(',');
    if (path === '/configuration') {
      sendJson(response, 200, {
        images: { secure_base_url: `http://127.0.0.1:${port}/t/p/`, poster_sizes: ['original'] },
      });
      return;
    }
    const search = /^\/search\/(movie|tv)$/.exec(path);
    if (search) {
      const query = (url.searchParams.get('query') ?? '').toLowerCase();
      const list = search[1] === 'movie' ? MOVIES : SHOWS;
      const results = list
        .filter(
          (title) =>
            query !== '' && [title.de, title.en].some((name) => name.toLowerCase().includes(query)),
        )
        .map((title) => searchHit(search[1], title));
      sendJson(response, 200, { page: 1, results, total_pages: 1, total_results: results.length });
      return;
    }
    const videos = /^\/(movie|tv)\/(\d+)\/videos$/.exec(path);
    if (videos) {
      const known = (videos[1] === 'movie' ? MOVIES : SHOWS).some(
        (title) => String(title.id) === videos[2],
      );
      sendJson(
        response,
        known ? 200 : 404,
        known
          ? {
              id: Number(videos[2]),
              results: [
                {
                  iso_639_1: 'de',
                  iso_3166_1: 'DE',
                  name: 'Trailer',
                  key: `e2e${videos[2]}trl`,
                  site: 'YouTube',
                  type: 'Trailer',
                  official: true,
                  published_at: '2026-01-01T00:00:00.000Z',
                  size: 1080,
                },
              ],
            }
          : { status_code: 34, status_message: 'The resource you requested could not be found.' },
      );
      return;
    }
    const detail = /^\/(movie|tv)\/(\d+)$/.exec(path);
    if (detail) {
      const movie = detail[1] === 'movie' && MOVIES.find((title) => String(title.id) === detail[2]);
      const show = detail[1] === 'tv' && SHOWS.find((title) => String(title.id) === detail[2]);
      if (movie) sendJson(response, 200, movieOf(movie, language, append));
      else if (show) sendJson(response, 200, showOf(show, language, append));
      else {
        sendJson(response, 404, {
          status_code: 34,
          status_message: 'The resource you requested could not be found.',
        });
      }
      return;
    }
    // /find, /collection, /person, seasons …: nothing is known (library titles stay as they are).
    sendJson(response, 404, {
      status_code: 34,
      status_message: 'The resource you requested could not be found.',
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
  const { port } = await startTmdbServer();
  console.log(`tmdb mock on http://127.0.0.1:${port}`);
}
