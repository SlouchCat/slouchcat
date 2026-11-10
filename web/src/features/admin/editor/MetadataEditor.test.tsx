import type { AdminItem, ImageCandidate } from '@slouchcat/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';
import { alienVersions, audioIds, audiobookDetail, ids } from '@/test/fixtures';

// The metadata editor over the detail page (concept 15 7.4): the pen, the candidates with their
// filter and the choice, «Speichern» with every change at once and the field errors; the editor
// of an audiobook (concept 19 D5.2). The upload and the keyboard paths are in
// e2e/metadata-editor.spec.ts and e2e/audiobook-editor.spec.ts.

const item = {
  id: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  libraryId: 1,
  metadataState: 'matched',
  providers: ['tmdb'],
  fieldSources: [],
  lockedFields: ['overview:de'],
  trailerState: 'none',
  trailers: [],
  mediaFiles: [],
  children: { seasons: 0, episodes: 0, extras: 0 },
  images: [{ kind: 'poster', provider: 'tmdb', lang: 'de', chosen: false, locked: false }],
  imageKinds: ['poster', 'backdrop', 'logo'],
  metadata: {
    lang: 'de',
    title: 'Alien',
    sortTitle: 'Alien',
    year: 1979,
    overview: 'Im Weltall hört dich niemand schreien.',
    genres: ['Horror'],
    locked: ['overview'],
    sources: [{ field: 'title', provider: 'tmdb' }],
  },
} as unknown as AdminItem;

const candidate = (provider: string, lang: string, source: string): ImageCandidate => ({
  provider,
  lang,
  source,
  width: 2000,
  height: 3000,
  score: 0,
  voteCount: 0,
  previewUrl: `https://images.example.test/${source}`,
  chosen: false,
});

function routes(overrides: Record<string, Parameters<typeof mockApi>[0][string]> = {}) {
  return mockApi({
    ...signedIn,
    [`GET /items/${ids.alien}`]: () => json(alienVersions),
    [`GET /admin/items/${ids.alien}`]: () => json(item),
    'GET /libraries/visible': () =>
      json([{ id: 1, name: 'Filme', kind: 'video', contents: ['movie'], itemCount: 1 }]),
    'GET /admin/genres': () => json({ lang: 'de', genres: ['Action', 'Drama', 'Horror'] }),
    'GET /admin/settings/metadata': () => json({ knownLanguages: ['de', 'en'] }),
    [`GET /admin/items/${ids.alien}/image-candidates`]: () =>
      json({
        candidates: [
          candidate('tmdb', 'de', 'tmdb-de'),
          candidate('tmdb', 'en', 'tmdb-en'),
          candidate('fanart', 'en', 'fanart-en'),
        ],
        unavailable: [],
      }),
    ...overrides,
  });
}

async function openEditor(user: ReturnType<typeof userEvent.setup>) {
  renderApp(`/items/${ids.alien}`);
  await user.click(await screen.findByRole('button', { name: 'Metadaten bearbeiten' }));
  return screen.findByRole('dialog', { name: 'Alien' });
}

describe('metadata editor', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('opens with the pen: head, link to the administration, tab Bilder first', async () => {
    routes();
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    expect(within(dialog).getByText('Film · Filme')).toBeVisible();
    expect(within(dialog).getByRole('link', { name: 'In der Verwaltung öffnen' })).toHaveAttribute(
      'href',
      `/admin/items/${ids.alien}`,
    );
    expect(within(dialog).getByRole('tab', { selected: true })).toHaveTextContent('Bilder');
    expect(
      within(dialog).getByRole('button', { name: 'Poster, automatisch · TMDB' }),
    ).toHaveAttribute('aria-pressed', 'true');
    await user.click(within(dialog).getByRole('button', { name: 'Schliessen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('filters the candidates by provider and language; a click chooses and locks', async () => {
    const chosen = {
      ...item,
      images: [{ kind: 'poster', provider: 'fanart', lang: 'en', chosen: true, locked: true }],
    } as unknown as AdminItem;
    const { calls } = routes({
      [`PUT /admin/items/${ids.alien}/images/poster`]: () => json(chosen),
    });
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    const grid = await within(dialog).findByRole('listbox', { name: 'Kandidaten für Poster' });
    expect(within(grid).getAllByRole('option')).toHaveLength(3);

    await user.selectOptions(within(dialog).getByRole('combobox', { name: 'Anbieter' }), 'tmdb');
    expect(within(grid).getAllByRole('option')).toHaveLength(2);
    await user.selectOptions(within(dialog).getByRole('combobox', { name: 'Sprache' }), 'en');
    expect(within(grid).getAllByRole('option')).toHaveLength(1);
    await user.selectOptions(within(dialog).getByRole('combobox', { name: 'Anbieter' }), '*');
    expect(within(grid).getAllByRole('option')).toHaveLength(2);
    expect(within(dialog).getByText('2 Bilder')).toBeVisible();

    await user.click(within(grid).getAllByRole('option')[1]!);
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
        provider: 'fanart',
        source: 'fanart-en',
      }),
    );
    expect(
      await within(dialog).findByText('Poster gewählt und gesperrt; das Bild wird geholt.'),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole('button', { name: 'Poster, gewählt · gesperrt' }),
    ).toBeVisible();
    expect(within(dialog).getByRole('button', { name: 'Zurück auf automatisch' })).toBeVisible();
  });

  it('«Speichern» sends every change and the lifted lock at once', async () => {
    const { calls } = routes({
      [`PATCH /admin/items/${ids.alien}/metadata`]: () => json(item),
      [`PUT /admin/items/${ids.alien}/rating`]: () => json({ rating: 16 }),
    });
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    await user.click(within(dialog).getByRole('tab', { name: 'Metadaten' }));
    const panel = within(dialog).getByRole('tabpanel');
    expect(within(panel).getByText(/Texte gelten für die Sprache Deutsch/)).toBeVisible();
    expect(within(panel).getByText('aus TMDB')).toBeVisible();

    const title = within(panel).getByRole('textbox', { name: 'Titel' });
    const titleLock = within(panel).getByRole('button', { name: 'Sperre Titel' });
    expect(titleLock).toHaveAttribute('aria-pressed', 'false');
    expect(titleLock).toBeDisabled();
    await user.clear(title);
    await user.type(title, 'Alien 2');
    expect(titleLock).toHaveAttribute('aria-pressed', 'true');

    const overviewLock = within(panel).getByRole('button', { name: 'Sperre Beschreibung' });
    expect(overviewLock).toHaveAttribute('aria-pressed', 'true');
    await user.click(overviewLock);
    expect(overviewLock).toHaveAttribute('aria-pressed', 'false');

    const genre = within(panel).getByRole('combobox', { name: 'Genre hinzufügen' });
    await user.type(genre, 'Action{Enter}');
    expect(within(panel).getByRole('button', { name: 'Action entfernen' })).toBeVisible();
    await user.click(within(panel).getByRole('button', { name: 'Horror entfernen' }));

    await user.selectOptions(within(panel).getByRole('combobox', { name: 'Altersfreigabe' }), '16');
    expect(within(dialog).getByRole('tab', { name: 'Metadaten ungespeichert' })).toBeVisible();

    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
        title: 'Alien 2',
        genres: ['Action'],
        unlock: ['overview'],
      }),
    );
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ rating: 16 }),
    );
    expect(await within(dialog).findByText('Metadaten gespeichert.')).toBeInTheDocument();
    expect(within(dialog).getByRole('tab', { name: 'Metadaten' })).toBeVisible();
  });

  it('shows a wrong year before sending and the server’s field errors at their field', async () => {
    const { calls } = routes({
      [`PATCH /admin/items/${ids.alien}/metadata`]: () =>
        problem(422, 'validation_failed', {
          errors: [
            { field: 'sortTitle', code: 'required' },
            { field: 'genres', code: 'too_long' },
          ],
        }),
    });
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    await user.click(within(dialog).getByRole('tab', { name: 'Metadaten' }));
    const panel = within(dialog).getByRole('tabpanel');

    const year = within(panel).getByRole('textbox', { name: 'Jahr' });
    await user.clear(year);
    await user.type(year, '79');
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    expect(await within(panel).findByText('Ungültiger Wert.')).toBeVisible();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);

    await user.clear(year);
    await user.type(year, '1979');
    // A free sort title is empty and shows the title; the title's own text sets and locks it.
    const sort = within(panel).getByRole('textbox', { name: 'Sortiertitel' });
    expect(sort).toHaveValue('');
    expect(sort).toHaveAttribute('placeholder', 'Alien');
    await user.type(sort, 'Alien');
    expect(within(panel).getByRole('button', { name: 'Sperre Sortiertitel' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await user.click(within(panel).getByRole('button', { name: 'Horror entfernen' }));
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
        sortTitle: 'Alien',
        genres: [],
      }),
    );
    expect(await within(panel).findByText('Pflichtfeld.')).toBeVisible();
    // The genres have no input of their own: the message stands under their group.
    const genres = within(panel).getByRole('group', { name: 'Genres' });
    expect(within(genres).getByText('Zu lang.')).toBeVisible();
    expect(genres).toHaveAccessibleDescription('Zu lang.');
  });

  it('a failed rating after saved texts reads the title again and keeps the draft', async () => {
    let reads = 0;
    const { calls } = routes({
      [`GET /admin/items/${ids.alien}`]: () => {
        reads += 1;
        return json(item);
      },
      [`PATCH /admin/items/${ids.alien}/metadata`]: () => json(item),
      [`PUT /admin/items/${ids.alien}/rating`]: () =>
        problem(422, 'validation_failed', { errors: [{ field: 'rating', code: 'invalid_value' }] }),
    });
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    await user.click(within(dialog).getByRole('tab', { name: 'Metadaten' }));
    const panel = within(dialog).getByRole('tabpanel');
    const title = within(panel).getByRole('textbox', { name: 'Titel' });
    await user.clear(title);
    await user.type(title, 'Alien 2');
    await user.selectOptions(within(panel).getByRole('combobox', { name: 'Altersfreigabe' }), '16');
    const before = reads;
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'PUT')).toBe(true));
    // The rating's message stands under its select.
    expect(await within(panel).findAllByText('Ungültiger Wert.')).not.toHaveLength(0);
    await waitFor(() => expect(reads).toBeGreaterThan(before));
    expect(title).toHaveValue('Alien 2');
    expect(within(dialog).getByRole('tab', { name: 'Metadaten ungespeichert' })).toBeVisible();
  });

  it('Enter on a picked suggestion adds the suggestion, not the typed part', async () => {
    routes();
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    await user.click(within(dialog).getByRole('tab', { name: 'Metadaten' }));
    const panel = within(dialog).getByRole('tabpanel');
    const genre = within(panel).getByRole('combobox', { name: 'Genre hinzufügen' });
    await user.type(genre, 'Dra');
    // Firefox writes the highlighted suggestion into the field after the page saw the Enter.
    fireEvent.keyDown(genre, { key: 'Enter' });
    fireEvent.change(genre, { target: { value: 'Drama' } });
    expect(await within(panel).findByRole('button', { name: 'Drama entfernen' })).toBeVisible();
    expect(within(panel).queryByRole('button', { name: 'Dra entfernen' })).not.toBeInTheDocument();
    expect(genre).toHaveValue('');
  });

  it('a half-typed date is refused before sending instead of clearing the date', async () => {
    const dated = {
      ...item,
      metadata: { ...item.metadata, releaseDate: '1979-05-25' },
    } as unknown as AdminItem;
    const { calls } = routes({
      [`GET /admin/items/${ids.alien}`]: () => json(dated),
      [`PATCH /admin/items/${ids.alien}/metadata`]: () => json(dated),
    });
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    await user.click(within(dialog).getByRole('tab', { name: 'Metadaten' }));
    const panel = within(dialog).getByRole('tabpanel');
    const date = within(panel).getByLabelText('Erscheinungsdatum');
    // What a browser reports for a date with a segment missing: no value, bad input.
    Object.defineProperty(date, 'validity', {
      configurable: true,
      get: () => ({ badInput: true, valid: false }),
    });
    await user.clear(date);
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    expect(await within(panel).findByText('Ungültiger Wert.')).toBeVisible();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
  });

  it('a later choice clears the message of a failed upload', async () => {
    const chosen = {
      ...item,
      images: [{ kind: 'poster', provider: 'fanart', lang: 'en', chosen: true, locked: true }],
    } as unknown as AdminItem;
    // Too large is answered at once, without a request (the multipart body is not the mock's).
    routes({ [`PUT /admin/items/${ids.alien}/images/poster`]: () => json(chosen) });
    const user = userEvent.setup();
    const dialog = await openEditor(user);
    const grid = await within(dialog).findByRole('listbox', { name: 'Kandidaten für Poster' });
    await user.upload(
      within(dialog).getByLabelText('Eigenes Bild hochladen', { selector: 'input' }),
      new File([new Uint8Array(20 * 1024 * 1024 + 1)], 'gross.png', { type: 'image/png' }),
    );
    const failed = 'Die Datei ist grösser als 20 MB.';
    expect(await within(dialog).findByText(failed)).toBeVisible();
    await user.click(within(grid).getAllByRole('option')[2]!);
    expect(
      await within(dialog).findByText('Poster gewählt und gesperrt; das Bild wird geholt.'),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(failed)).not.toBeInTheDocument();
  });
});

const book = {
  id: audioIds.book,
  kind: 'audiobook',
  title: 'Slouchcat Test Audiobook M4B',
  libraryId: 2,
  metadataState: 'matched',
  providers: ['local'],
  fieldSources: [],
  lockedFields: ['narrator'],
  trailerState: 'none',
  trailers: [],
  mediaFiles: [],
  children: { seasons: 0, episodes: 0, extras: 0 },
  images: [{ kind: 'poster', provider: 'embedded', chosen: false, locked: false }],
  imageKinds: ['poster'],
  durationMs: 60_000,
  metadata: {
    lang: 'de',
    title: 'Slouchcat Test Audiobook M4B',
    year: 2024,
    genres: ['Test'],
    author: 'Slouchcat Test Author',
    narrator: 'Slouchcat Test Narrator',
    seriesName: 'Slouchcat Test Audio Series',
    seriesIndex: 1,
    locked: ['narrator'],
    sources: [
      { field: 'title', provider: 'local' },
      { field: 'author', provider: 'local' },
    ],
  },
} as unknown as AdminItem;

function bookRoutes(overrides: Record<string, Parameters<typeof mockApi>[0][string]> = {}) {
  return mockApi({
    ...signedIn,
    [`GET /items/${audioIds.book}`]: () => json(audiobookDetail),
    [`GET /admin/items/${audioIds.book}`]: () => json(book),
    'GET /libraries/visible': () =>
      json([
        { id: 2, name: 'Hörbücher', kind: 'audiobooks', contents: ['audiobook'], itemCount: 2 },
      ]),
    'GET /admin/genres': () => json({ lang: 'de', genres: ['Test'] }),
    'GET /admin/settings/metadata': () => json({ knownLanguages: ['de', 'en'] }),
    [`GET /admin/items/${audioIds.book}/image-candidates`]: () =>
      json({
        candidates: [
          {
            ...candidate('local', '', 'file:Slouchcat Test Audiobook M4B/cover.jpg?1234-5678'),
            width: 600,
            height: 600,
          },
          {
            ...candidate('embedded', '', 'embedded:mf-a1?5678'),
            width: 500,
            height: 500,
            chosen: true,
          },
        ],
        unavailable: [],
      }),
    ...overrides,
  });
}

async function openBookEditor(user: ReturnType<typeof userEvent.setup>) {
  renderApp(`/items/${audioIds.book}`);
  await user.click(await screen.findByRole('button', { name: 'Metadaten bearbeiten' }));
  return screen.findByRole('dialog', { name: 'Slouchcat Test Audiobook M4B' });
}

describe('metadata editor: an audiobook (concept 19 D5.2)', () => {
  it('opens on «Metadaten» with the tabs of a book and its fields in order', async () => {
    bookRoutes();
    const user = userEvent.setup();
    const dialog = await openBookEditor(user);
    expect(within(dialog).getByText('Hörbuch · Hörbücher')).toBeVisible();
    expect(
      within(dialog)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual(['Metadaten', 'Cover', 'Zuordnen', 'Aktualisieren']);
    // Every field comes from the files: no source matched the book.
    expect(within(dialog).getByText('Nicht zugeordnet: Angaben nur aus den Dateien')).toBeVisible();
    expect(within(dialog).getByRole('tab', { selected: true })).toHaveTextContent('Metadaten');
    const panel = within(dialog).getByRole('tabpanel');
    expect(within(panel).getByText(/Texte gelten für jede Sprachzeile des Buches/)).toBeVisible();
    const labels = within(panel)
      .getAllByRole('textbox')
      .map((field) => field.getAttribute('name'));
    expect(labels).toEqual([
      'title',
      'tagline',
      'sortTitle',
      'author',
      'narrator',
      'seriesName',
      'seriesIndex',
      'year',
      'publisher',
      'overview',
      'isbn',
      'asin',
    ]);
    expect(within(panel).getByRole('textbox', { name: 'Untertitel' })).toBeVisible();
    expect(within(panel).getByRole('combobox', { name: 'Originalsprache' })).toBeVisible();
    expect(within(panel).getByRole('group', { name: 'Genres' })).toBeVisible();
    expect(within(panel).queryByRole('textbox', { name: 'Originaltitel' })).not.toBeInTheDocument();
    expect(within(panel).queryByLabelText('Erscheinungsdatum')).not.toBeInTheDocument();
    // The files' values read «aus den Dateien», a locked one «von Hand».
    expect(within(panel).getAllByText('aus den Dateien')).toHaveLength(2);
    expect(within(panel).getByText('von Hand')).toBeVisible();
    expect(within(panel).getByRole('button', { name: 'Sperre Sprecher' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('«Speichern» sends the book fields with the volume as a number', async () => {
    const { calls } = bookRoutes({
      [`PATCH /admin/items/${audioIds.book}/metadata`]: () => json(book),
    });
    const user = userEvent.setup();
    const dialog = await openBookEditor(user);
    const panel = within(dialog).getByRole('tabpanel');
    const field = (name: string) => within(panel).getByRole('textbox', { name });
    await user.clear(field('Autor'));
    await user.type(field('Autor'), 'Neuer Autor');
    expect(within(panel).getByRole('button', { name: 'Sperre Autor' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await user.clear(field('Sprecher'));
    await user.type(field('Sprecher'), 'Neue Sprecherin');
    await user.clear(field('Reihe'));
    await user.type(field('Reihe'), 'Neue Reihe');
    await user.clear(field('Band'));
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    // An emptied volume of a series that stays is refused before sending.
    expect(await within(panel).findByText('Pflichtfeld.')).toBeVisible();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);

    await user.type(field('Band'), '2,5');
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
        author: 'Neuer Autor',
        narrator: 'Neue Sprecherin',
        seriesName: 'Neue Reihe',
        seriesIndex: 2.5,
      }),
    );
    expect(await within(dialog).findByText('Metadaten gespeichert.')).toBeInTheDocument();
  });

  it('«Cover»: square candidates named by file, the embedded picture current', async () => {
    const chosen = {
      ...book,
      images: [{ kind: 'poster', provider: 'local', chosen: true, locked: true }],
    } as unknown as AdminItem;
    const { calls } = bookRoutes({
      [`PUT /admin/items/${audioIds.book}/images/poster`]: () => json(chosen),
    });
    const user = userEvent.setup();
    const dialog = await openBookEditor(user);
    await user.click(within(dialog).getByRole('tab', { name: 'Cover' }));
    expect(
      within(dialog).getByRole('button', { name: 'Cover, automatisch · Eingebettet' }),
    ).toHaveAttribute('aria-pressed', 'true');
    expect(within(dialog).getByText('Bilder im Ordner und in den Dateien')).toBeVisible();
    expect(within(dialog).queryByRole('combobox', { name: 'Sprache' })).not.toBeInTheDocument();
    const grid = await within(dialog).findByRole('listbox', { name: 'Kandidaten für Cover' });
    expect(grid).toHaveAttribute('data-shape', 'square');
    expect(
      within(grid)
        .getAllByRole('option')
        .map((option) => option.textContent),
    ).toEqual([
      'cover.jpg600 × 600 · Lokale Datei',
      'Eingebettetes Bildaktuell500 × 500 · Eingebettet',
    ]);
    await user.click(within(grid).getByRole('option', { name: /^cover\.jpg/ }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
        provider: 'local',
        source: 'file:Slouchcat Test Audiobook M4B/cover.jpg?1234-5678',
      }),
    );
    expect(
      await within(dialog).findByRole('button', { name: 'Cover, gewählt · gesperrt' }),
    ).toBeVisible();
  });

  it('«Aktualisieren» reads the files again, with «Dabei alle Schlösser lösen»', async () => {
    const { calls } = bookRoutes({
      [`POST /admin/items/${audioIds.book}/refresh`]: () => json(book),
    });
    const user = userEvent.setup();
    const dialog = await openBookEditor(user);
    await user.click(within(dialog).getByRole('tab', { name: 'Aktualisieren' }));
    const panel = within(dialog).getByRole('tabpanel');
    expect(within(panel).getByText(/metadata\.json, die Tags und die Ordnernamen/)).toBeVisible();
    // Without a match there is nothing to fetch from a provider.
    expect(within(panel).getByText('Erst unter «Zuordnen» einen Treffer wählen.')).toBeVisible();
    expect(within(panel).getByRole('button', { name: 'Neu holen' })).toBeDisabled();
    await user.click(within(panel).getByRole('checkbox', { name: /^Dabei alle Schlösser lösen/ }));
    await user.click(within(panel).getByRole('button', { name: 'Neu lesen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ unlockAll: true }),
    );
  });
});

// The tab «Zuordnen» of an audiobook (concept 19 D3, D5.2): the source choice, the search with
// the server's length mark, the comparison «Was übernommen wird» and «Vom Anbieter neu holen».
const bookInFolder = {
  ...book,
  providers: ['local', 'audible', 'dnb', 'applebooks', 'openlibrary', 'googlebooks'],
} as unknown as AdminItem;

const bookProvider = (id: string, configured = true) => ({
  id,
  configured,
  roles: id === 'local' ? ['identify'] : ['fill'],
  contents: ['audiobook'],
  languages: [],
});

const hits = [
  {
    provider: 'audible',
    providerId: 'B0TEST0001',
    title: 'Slouchcat Test Audiobook M4B',
    subtitle: 'Band 1',
    authors: ['Slouchcat Test Author'],
    narrators: ['Slouchcat Test Narrator'],
    publisher: 'Testverlag',
    year: 2024,
    durationMs: 60_000,
    asin: 'B0TEST0001',
    coverUrl: 'https://books.example.test/b0test0001.jpg',
    region: 'de',
    durationMatches: true,
    score: 0.9,
  },
  {
    provider: 'audible',
    providerId: 'B0TEST0002',
    title: 'Slouchcat Test Audiobook M4B',
    subtitle: 'Hörspiel',
    authors: ['Slouchcat Test Author'],
    durationMs: 7_260_000,
    asin: 'B0TEST0002',
    region: 'de',
    durationMatches: false,
    score: 0.3,
  },
];

const preview = {
  provider: 'audible',
  providerId: 'B0TEST0001',
  title: 'Slouchcat Test Audiobook M4B',
  durationMs: 60_000,
  durationMatches: true,
  fields: [
    {
      field: 'title',
      current: 'Slouchcat Test Audiobook M4B',
      proposed: 'Slouchcat Test Audiobook M4B',
      locked: false,
      differs: false,
    },
    { field: 'subtitle', current: '', proposed: 'Band 1', locked: false, differs: true },
    {
      field: 'narrator',
      current: 'Slouchcat Test Narrator',
      proposed: 'Neue Sprecherin',
      locked: true,
      differs: true,
    },
    { field: 'publisher', current: '', proposed: 'Testverlag', locked: false, differs: true },
    { field: 'asin', current: '', proposed: 'B0TEST0001', locked: false, differs: true },
    {
      field: 'cover',
      current: '',
      proposed: 'https://books.example.test/b0test0001.jpg',
      locked: false,
      differs: true,
    },
  ],
};

const matchedBook = {
  ...bookInFolder,
  bookMatch: {
    state: 'matched',
    provider: 'audible',
    providerId: 'B0TEST0001',
    region: 'de',
    matchedAt: '2026-11-10T21:00:00Z',
    manual: true,
  },
} as unknown as AdminItem;

function matchRoutes(overrides: Record<string, Parameters<typeof mockApi>[0][string]> = {}) {
  return bookRoutes({
    [`GET /admin/items/${audioIds.book}`]: () => json(bookInFolder),
    'GET /admin/metadata/providers': () =>
      json([
        bookProvider('local'),
        bookProvider('audible'),
        bookProvider('dnb'),
        bookProvider('applebooks'),
        bookProvider('openlibrary'),
        bookProvider('googlebooks', false),
      ]),
    'GET /admin/metadata/audiobooks': () => json({ audibleRegion: 'de' }),
    'GET /admin/metadata/search': () => json(hits),
    [`GET /admin/items/${audioIds.book}/match/preview`]: () => json(preview),
    ...overrides,
  });
}

/** The query of every request to `path`, in order. */
const queriesOf = (fetchMock: ReturnType<typeof mockApi>['fetchMock'], path: string) =>
  fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => url.pathname === `/api/v1${path}`)
    .map((url) => Object.fromEntries(url.searchParams));

async function openMatchTab(user: ReturnType<typeof userEvent.setup>) {
  const dialog = await openBookEditor(user);
  await user.click(within(dialog).getByRole('tab', { name: 'Zuordnen' }));
  return { dialog, panel: within(dialog).getByRole('tabpanel') };
}

describe('metadata editor: matching an audiobook (concept 19 D3)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('offers the sources of the folder and searches with title, author, region and book', async () => {
    const { fetchMock } = matchRoutes();
    const user = userEvent.setup();
    const { panel } = await openMatchTab(user);
    const select = within(panel).getByRole('combobox', { name: 'Quelle' });
    await waitFor(() =>
      expect(
        within(select)
          .getAllByRole('option')
          .map((option) => option.textContent),
      ).toEqual([
        'Audible Deutschland',
        'Audible USA',
        'Audible Grossbritannien',
        'Deutsche Nationalbibliothek',
        'Apple Books',
        // Google Books has no key: not offered.
        'Open Library',
      ]),
    );
    expect(within(panel).getByRole('textbox', { name: 'Titel' })).toHaveValue(
      'Slouchcat Test Audiobook M4B',
    );
    expect(within(panel).getByRole('textbox', { name: 'Autor' })).toHaveValue(
      'Slouchcat Test Author',
    );
    expect(await within(panel).findByText('Länge der Dateien: 0 Std. 1 Min.')).toBeVisible();

    await user.click(within(panel).getByRole('button', { name: 'Suchen' }));
    const first = await within(panel).findByRole('button', {
      name: 'Slouchcat Test Audiobook M4B – Band 1 wählen (0 Std. 1 Min., passt)',
    });
    expect(first).toHaveTextContent('Slouchcat Test Author · gelesen von Slouchcat Test Narrator');
    expect(first).toHaveTextContent('Testverlag · 2024 · ASIN B0TEST0001');
    expect(
      within(panel).getByRole('button', {
        name: 'Slouchcat Test Audiobook M4B – Hörspiel wählen (2 Std. 1 Min.)',
      }),
    ).toBeVisible();
    expect(queriesOf(fetchMock, '/admin/metadata/search')).toEqual([
      {
        provider: 'audible',
        kind: 'audiobook',
        query: 'Slouchcat Test Audiobook M4B',
        author: 'Slouchcat Test Author',
        region: 'de',
        itemId: audioIds.book,
      },
    ]);

    // Another source without region, an ISBN instead of the title.
    await user.selectOptions(select, 'dnb');
    await user.clear(within(panel).getByRole('textbox', { name: 'Titel' }));
    await user.clear(within(panel).getByRole('textbox', { name: 'Autor' }));
    await user.type(within(panel).getByRole('textbox', { name: 'ISBN' }), '978-3-00-000000-0');
    await user.click(within(panel).getByRole('button', { name: 'Suchen' }));
    await waitFor(() =>
      expect(queriesOf(fetchMock, '/admin/metadata/search')[1]).toEqual({
        provider: 'dnb',
        kind: 'audiobook',
        isbn: '978-3-00-000000-0',
        itemId: audioIds.book,
      }),
    );
  });

  it('asks for a title, an ASIN or an ISBN before searching', async () => {
    const { fetchMock } = matchRoutes();
    const user = userEvent.setup();
    const { panel } = await openMatchTab(user);
    await user.clear(within(panel).getByRole('textbox', { name: 'Titel' }));
    await user.click(within(panel).getByRole('button', { name: 'Suchen' }));
    expect(
      await within(panel).findByText('Gib einen Titel, eine ASIN oder eine ISBN an.'),
    ).toBeVisible();
    await user.type(within(panel).getByRole('textbox', { name: 'ASIN' }), 'B0TEST');
    await user.click(within(panel).getByRole('button', { name: 'Suchen' }));
    expect(
      await within(panel).findByText('Eine ASIN hat zehn Buchstaben oder Ziffern.'),
    ).toBeVisible();
    expect(queriesOf(fetchMock, '/admin/metadata/search')).toEqual([]);
  });

  it('says when the source rests', async () => {
    matchRoutes({
      'GET /admin/metadata/search': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'provider', code: 'provider_paused' }],
        }),
    });
    const user = userEvent.setup();
    const { panel } = await openMatchTab(user);
    await user.click(within(panel).getByRole('button', { name: 'Suchen' }));
    expect(
      await within(panel).findByText(
        'Die Quelle pausiert nach wiederholten Absagen. Versuche es später noch einmal.',
      ),
    ).toBeVisible();
  });

  it('compares a hit field by field; locked fields stay; the ticked fields are taken', async () => {
    const { calls, fetchMock } = matchRoutes({
      [`PUT /admin/items/${audioIds.book}/match`]: () => json(matchedBook, 202),
    });
    const user = userEvent.setup();
    const { dialog, panel } = await openMatchTab(user);
    await user.click(within(panel).getByRole('button', { name: 'Suchen' }));
    const first = await within(panel).findByRole('button', { name: /Band 1 wählen/ });
    await user.click(first);
    expect(first).toHaveAttribute('aria-pressed', 'true');

    const table = await within(panel).findByRole('table', {
      name: 'Felder von Audible, heute und neu',
    });
    expect(queriesOf(fetchMock, `/admin/items/${audioIds.book}/match/preview`)).toEqual([
      { provider: 'audible', providerId: 'B0TEST0001', region: 'de' },
    ]);
    // Only the fields that differ; the title is the same today.
    const rows = within(table).getAllByRole('row').slice(1);
    expect(rows.map((row) => within(row).getByRole('rowheader').textContent)).toEqual([
      'Untertitel',
      'Sprecher',
      'Verlag',
      'ASIN',
      'Cover',
    ]);
    // The locked narrator has no box, only its lock.
    expect(within(rows[1]!).queryByRole('checkbox')).not.toBeInTheDocument();
    expect(within(rows[1]!).getByText('gesperrt')).toBeVisible();
    expect(within(table).getByRole('checkbox', { name: 'Untertitel übernehmen' })).toBeChecked();
    expect(within(panel).getByRole('button', { name: '4 Felder übernehmen' })).toBeEnabled();

    await user.click(within(table).getByRole('checkbox', { name: 'Verlag übernehmen' }));
    await user.click(within(panel).getByRole('button', { name: '3 Felder übernehmen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
        provider: 'audible',
        providerId: 'B0TEST0001',
        region: 'de',
        fields: ['subtitle', 'asin', 'cover'],
      }),
    );
    expect(
      await within(dialog).findByText('3 Felder von Audible übernommen und gesperrt.'),
    ).toBeInTheDocument();
    expect(within(dialog).getByText('Zugeordnet: Audible')).toBeVisible();
    expect(within(panel).queryByRole('table')).not.toBeInTheDocument();
  });

  it('«Vom Anbieter neu holen» fetches the stored match again', async () => {
    const { calls } = matchRoutes({
      [`GET /admin/items/${audioIds.book}`]: () => json(matchedBook),
      [`POST /admin/items/${audioIds.book}/refresh`]: () => json(matchedBook, 202),
    });
    const user = userEvent.setup();
    const dialog = await openBookEditor(user);
    expect(within(dialog).getByText('Zugeordnet: Audible')).toBeVisible();
    await user.click(within(dialog).getByRole('tab', { name: 'Aktualisieren' }));
    const panel = within(dialog).getByRole('tabpanel');
    expect(within(panel).getByText('Holt die Angaben von Audible noch einmal.')).toBeVisible();
    await user.click(within(panel).getByRole('button', { name: 'Neu holen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
        unlockAll: false,
        fromProvider: true,
      }),
    );
    expect(
      await within(dialog).findByText('Die Angaben von Audible werden neu geholt.'),
    ).toBeInTheDocument();
  });
});
