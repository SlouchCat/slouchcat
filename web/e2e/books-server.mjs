// Local book sources for the Playwright suite (concept 19 D3): the e2e server asks this mock instead
// of Audible, Audnexus, the Deutsche Nationalbibliothek, Apple Books, Open Library and Google Books
// (SLOUCHCAT_TEST_BOOKS_BASE_URL, honoured with --dev only), so no test needs the internet.
// start-server.mjs starts it next to the Slouchcat binary and stops it with it; `node
// e2e/books-server.mjs` runs it alone. The paths are those of BookSourceURLsFromBase
// (server/internal/metadata/book_provider.go), the answers have the shapes of the fixtures in
// server/internal/metadata/testdata/books/.
//
// Every source answers "nothing found" by default, so the automatic match at start fills no field
// of the test audiobooks the other specs read (author, title, cover). Two exceptions:
//
//   - A search whose title holds SEARCH_TERM ("Quellentest", typed in the editor's tab «Zuordnen»
//     by audiobook-sources.spec.ts; the automatic match never sends it) finds two invented Audible
//     entries, one of them as long as the M4B test book (1 min), and one Apple Books entry.
//   - Apple Books knows the MP3 test book by title and author, without length or cover: the
//     automatic match keeps it as a candidate («Prüfen», state review) and writes nothing.
//
//   GET /audible/<region>/1.0/catalog/products?title=&author=   {products: [...]}
//   GET /audible/<region>/1.0/catalog/products/<ASIN>           {product: {...}} (no fields: unknown)
//   GET /audnexus/books/<ASIN>?region=                          the book, or 404
//   GET /dnb?…                                                  an SRU answer without records
//   GET /apple/search?term=…                                    {resultCount, results}
//   GET /apple/lookup?id=…                                      {resultCount, results}
//   GET /openlibrary/search.json?…                              {numFound: 0, docs: []}
//   GET /google/volumes?…                                       {totalItems: 0}
//   GET /covers/<name>.png                                      a tiny PNG for the hit list
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

/** The title the e2e spec searches for in the editor; only it finds Audible entries. */
export const SEARCH_TERM = 'Quellentest';

/** The MP3 test book (testdata/generate-media.sh), which Apple Books knows without length. */
const MP3_TITLE = 'Slouchcat Test Audiobook MP3';
const AUTHOR = 'Slouchcat Test Author';

/** A transparent 1×1 PNG. */
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNgYGD4DwABBAEAwS2OUAAAAABJRU5ErkJggg==',
  'base64',
);

/** The invented Audible entries of SEARCH_TERM, in the order the catalog answers them. */
function audibleProducts(base) {
  return [
    {
      asin: 'B0E2E00001',
      title: SEARCH_TERM,
      subtitle: 'Ungekürzte Lesung',
      authors: [{ name: AUTHOR }],
      narrators: [{ name: 'Erika Sprecherin' }],
      publisher_name: 'Testverlag Hören',
      release_date: '2020-05-04',
      language: 'german',
      runtime_length_min: 1,
      series: [{ title: 'Die Testreihe', sequence: '2' }],
      product_images: { 500: `${base}/covers/B0E2E00001.png` },
    },
    {
      asin: 'B0E2E00002',
      title: SEARCH_TERM,
      subtitle: 'Hörspiel',
      authors: [{ name: AUTHOR }],
      narrators: [{ name: 'Viele Stimmen' }],
      publisher_name: 'Testverlag Hören',
      release_date: '2018-01-05',
      language: 'german',
      runtime_length_min: 240,
      product_images: { 500: `${base}/covers/B0E2E00002.png` },
    },
  ];
}

/** Audnexus' details of an invented entry: what the comparison of the editor shows. */
function audnexusBook(product) {
  return {
    asin: product.asin,
    title: product.title,
    subtitle: product.subtitle,
    authors: product.authors,
    narrators: product.narrators,
    publisherName: product.publisher_name,
    releaseDate: `${product.release_date}T00:00:00.000Z`,
    language: product.language,
    runtimeLengthMin: product.runtime_length_min,
    ...(product.series
      ? { seriesPrimary: { name: product.series[0].title, position: product.series[0].sequence } }
      : {}),
    image: product.product_images[500],
    summary: '<p>Ein erfundener Satz für den Test.</p>',
    genres: [{ asin: '1', name: 'Krimi', type: 'genre' }],
    isbn: '978-3-00-000000-1',
    region: 'de',
    formatType: 'unabridged',
  };
}

/** The Apple Books entries: the MP3 test book (no length, no cover) and one of SEARCH_TERM. */
const APPLE = [
  {
    wrapperType: 'audiobook',
    collectionId: 1000000901,
    artistName: AUTHOR,
    collectionName: MP3_TITLE,
    releaseDate: '2024-01-01T08:00:00Z',
    primaryGenreName: 'Test',
    copyright: '℗ 2024 Testverlag Hören',
  },
  {
    wrapperType: 'audiobook',
    collectionId: 1000000902,
    artistName: AUTHOR,
    collectionName: SEARCH_TERM,
    releaseDate: '2020-05-04T08:00:00Z',
    primaryGenreName: 'Krimis & Thriller',
    copyright: '℗ 2020 Testverlag Hören',
    description: 'Ein erfundener Satz.',
  },
];

const DNB_EMPTY = `<?xml version="1.0" encoding="UTF-8"?>
<searchRetrieveResponse xmlns="http://www.loc.gov/zing/srw/">
  <version>1.1</version>
  <numberOfRecords>0</numberOfRecords>
</searchRetrieveResponse>
`;

const holds = (text, part) => (text ?? '').toLowerCase().includes(part.toLowerCase());

/**
 * Starts the mock on 127.0.0.1:`port` and resolves with its port and a `close` function.
 */
export function startBooksServer({ port = 8087 } = {}) {
  const base = `http://127.0.0.1:${port}`;
  const products = audibleProducts(base);

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', base);
    const path = url.pathname;
    const query = url.searchParams;
    const send = (status, body, type = 'application/json') => {
      response.writeHead(status, { 'Content-Type': type, 'Cache-Control': 'no-store' });
      response.end(typeof body === 'string' || Buffer.isBuffer(body) ? body : JSON.stringify(body));
    };
    if (request.method !== 'GET') return send(405, { error: 'method' });

    const catalog = /^\/audible\/[a-z.]+\/1\.0\/catalog\/products(?:\/([A-Za-z0-9]{10}))?$/.exec(
      path,
    );
    if (catalog) {
      const asin = catalog[1];
      if (asin) {
        const product = products.find((entry) => entry.asin === asin.toUpperCase());
        // A product without fields: not in this marketplace.
        return send(200, { product: product ?? { asin }, response_groups: ['always-returned'] });
      }
      const found = holds(query.get('title'), SEARCH_TERM) ? products : [];
      return send(200, { products: found, total_results: found.length });
    }
    const audnexus = /^\/audnexus\/books\/([A-Za-z0-9]{10})$/.exec(path);
    if (audnexus) {
      const product = products.find((entry) => entry.asin === audnexus[1].toUpperCase());
      return product
        ? send(200, audnexusBook(product))
        : send(404, { statusCode: 404, error: 'Not Found', message: 'Book not found' });
    }
    if (path === '/dnb') return send(200, DNB_EMPTY, 'application/xml; charset=utf-8');
    if (path === '/apple/search') {
      const term = query.get('term') ?? '';
      const results = APPLE.filter((entry) => holds(term, entry.collectionName));
      return send(200, { resultCount: results.length, results });
    }
    if (path === '/apple/lookup') {
      const results = APPLE.filter((entry) => String(entry.collectionId) === query.get('id'));
      return send(200, { resultCount: results.length, results });
    }
    if (path === '/openlibrary/search.json') return send(200, { numFound: 0, start: 0, docs: [] });
    if (path === '/google/volumes') return send(200, { kind: 'books#volumes', totalItems: 0 });
    if (path.startsWith('/covers/') && path.endsWith('.png')) return send(200, PNG, 'image/png');
    return send(404, { error: 'not found' });
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
  const { port } = await startBooksServer();
  console.log(`book sources mock on http://127.0.0.1:${port}`);
}
