import type { AdminItem, ImageCandidate } from '@slouchcat/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';
import { alienVersions, ids } from '@/test/fixtures';

// The metadata editor over the detail page (concept 15 7.4): the pen, the candidates with their
// filter and the choice, «Speichern» with every change at once and the field errors. The upload
// and the keyboard paths are in e2e/metadata-editor.spec.ts.

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
    'GET /libraries/visible': () => json([{ id: 1, name: 'Filme', kind: 'video' }]),
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
