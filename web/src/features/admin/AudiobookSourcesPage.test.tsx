import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { adminShell, renderAdminPage } from '@/test/admin';
import { json, mockApi, problem, signedIn, type Routes } from '@/test/api';
import { audioIds, library } from '@/test/fixtures';
import type { AudiobookSources, BookReviewEntry } from './queries';

// The page «Hörbücher: Quellen» (concept 19 D6.6, mockup Quellen): the source list of each
// audiobook folder with the way into its dialog, the settings with one save bar, the Google Books
// key, the state of the sources and of the library, «Jetzt alle ohne Zuordnung zuordnen» and the
// list «Prüfen» with the pen of the editor. The keyboard paths are in e2e/audiobook-sources.spec.ts.

const sources = (overrides: Partial<AudiobookSources> = {}): AudiobookSources => ({
  audibleRegion: 'de',
  audnexusUrl: 'https://api.audnex.us',
  googleBooksKey: { source: 'none' },
  autoMatch: 'all',
  tolerancePercent: 3,
  normalizeAuthors: true,
  normalizeSeries: true,
  folders: [
    {
      libraryId: 3,
      libraryName: 'Hörbücher',
      rootId: 7,
      path: '/media/Hoerbuecher',
      providers: ['local', 'audible', 'dnb', 'applebooks'],
    },
  ],
  sources: [
    { id: 'audible', configured: true, pausedUntil: '2026-11-10T22:30:00Z' },
    { id: 'dnb', configured: true },
    { id: 'applebooks', configured: true },
    { id: 'openlibrary', configured: true },
    { id: 'googlebooks', configured: false },
  ],
  stats: { total: 107, matched: 91, review: 9, none: 7, error: 0, pending: 0 },
  running: false,
  ...overrides,
});

const review: BookReviewEntry[] = [
  {
    itemId: audioIds.book,
    title: 'Slouchcat Test Audiobook M4B',
    author: 'Slouchcat Test Author',
    libraryId: 3,
    durationMs: 82_440_000,
    state: 'review',
    candidates: [
      {
        provider: 'audible',
        providerId: 'B0TEST0001',
        title: 'Slouchcat Test Audiobook M4B',
        subtitle: 'Band 1',
        durationMs: 82_440_000,
      },
      { provider: 'dnb', providerId: '1234567890', title: 'Slouchcat Test Audiobook M4B' },
    ],
  },
  {
    itemId: audioIds.podcast,
    title: 'Ein gestörtes Buch',
    author: 'Slouchcat Test Author',
    libraryId: 3,
    state: 'error',
    // `<source>:<reason>`, as the server stores it.
    error: 'audible:paused',
    candidates: [],
  },
];

const audioLibrary = library({
  id: 3,
  name: 'Hörbücher',
  kind: 'audiobooks',
  roots: [
    {
      id: 7,
      path: '/media/Hoerbuecher',
      content: 'audiobook',
      providers: ['local', 'audible', 'dnb', 'applebooks'],
      fsType: 'ext4',
      watch: 'inotify',
      itemCount: 2,
      fileCount: 3,
    },
  ],
});

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  ...adminShell,
  'GET /admin/metadata/audiobooks': () => json(sources()),
  'GET /admin/metadata/audiobooks/review': () => json(review),
  'GET /admin/libraries': () => json([audioLibrary]),
  'GET /admin/metadata/providers': () => json([]),
  ...extra,
});

const adminBook = {
  id: audioIds.book,
  kind: 'audiobook',
  title: 'Slouchcat Test Audiobook M4B',
  libraryId: 3,
  metadataState: 'skipped',
  providers: ['local', 'audible'],
  fieldSources: [],
  lockedFields: [],
  trailerState: 'none',
  trailers: [],
  mediaFiles: [],
  children: { seasons: 0, episodes: 0, extras: 0 },
};

async function openPage() {
  await renderAdminPage('/admin/metadata/audiobooks');
  return screen.findByRole('heading', { level: 1, name: 'Hörbücher: Quellen' });
}

describe('AudiobookSourcesPage', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the folders, settings, sources, numbers and the list «Prüfen»', async () => {
    mockApi(routes());
    await openPage();
    const folder = await screen.findByRole('group', { name: '/media/Hoerbuecher' });
    expect(
      within(folder).getByText(
        'Lokale Dateien › Audible › Deutsche Nationalbibliothek › Apple Books',
      ),
    ).toBeVisible();
    expect(within(folder).getByText('Bibliothek Hörbücher')).toBeVisible();
    expect(screen.getByRole('combobox', { name: 'Region von Audible' })).toHaveValue('de');
    expect(screen.getByRole('textbox', { name: 'Adresse von Audnexus' })).toHaveValue(
      'https://api.audnex.us',
    );
    expect(screen.getByRole('radio', { name: 'Alle ohne Zuordnung' })).toBeChecked();
    expect(
      screen.getByText('Einmal für die ganze Bibliothek, danach wie «Neue Hörbücher».'),
    ).toBeVisible();
    expect(screen.getByRole('radio', { name: '3 %' })).toBeChecked();
    expect(screen.getByRole('switch', { name: 'Autorennamen vereinheitlichen' })).toBeChecked();
    expect(screen.getByText(/Audibles Nutzungsbedingungen verbieten/)).toBeVisible();
    // Audible rests, Google Books has no key.
    expect(screen.getByText(/^pausiert bis /)).toBeVisible();
    expect(screen.getByText('ohne Schlüssel aus')).toBeVisible();
    expect(screen.getByText('Zugeordnet').parentElement).toHaveTextContent('91');

    const table = await screen.findByRole('table', { name: 'Hörbücher zum Prüfen' });
    const rows = within(table).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('22 Std. 54 Min.');
    expect(rows[0]).toHaveTextContent('zu prüfen');
    expect(rows[0]).toHaveTextContent(
      'Audible: Slouchcat Test Audiobook M4B – Band 1 · 22 Std. 54 Min.',
    );
    expect(rows[0]).toHaveTextContent(
      'Deutsche Nationalbibliothek: Slouchcat Test Audiobook M4B · Länge unbekannt',
    );
    expect(rows[1]).toHaveTextContent('gestört');
    expect(rows[1]).toHaveTextContent('Audible pausiert nach wiederholten Absagen.');
    expect(rows[1]).toHaveTextContent('keine');
  });

  it('saves only the changed settings with the save bar', async () => {
    const { calls } = mockApi(
      routes({
        'PATCH /admin/metadata/audiobooks': () =>
          json(sources({ audibleRegion: 'com', autoMatch: 'new', tolerancePercent: 5 })),
      }),
    );
    const user = userEvent.setup();
    await openPage();
    await user.selectOptions(
      await screen.findByRole('combobox', { name: 'Region von Audible' }),
      'com',
    );
    await user.click(screen.getByRole('radio', { name: 'Neue Hörbücher' }));
    await user.click(screen.getByRole('radio', { name: '5 %' }));
    await user.click(screen.getByRole('switch', { name: 'Reihennamen vereinheitlichen' }));
    await user.click(screen.getByRole('switch', { name: 'Reihennamen vereinheitlichen' }));
    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
        audibleRegion: 'com',
        autoMatch: 'new',
        tolerancePercent: 5,
      }),
    );
    expect(await screen.findByText('Einstellungen für Hörbücher gespeichert.')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Region von Audible' })).toHaveValue('com');
  });

  it('shows the server’s field error of the Audnexus address', async () => {
    mockApi(
      routes({
        'PATCH /admin/metadata/audiobooks': () =>
          problem(422, 'validation_failed', {
            errors: [{ field: 'audnexusUrl', code: 'invalid_url' }],
          }),
      }),
    );
    const user = userEvent.setup();
    await openPage();
    const field = await screen.findByRole('textbox', { name: 'Adresse von Audnexus' });
    await user.clear(field);
    await user.type(field, 'http://audnexus.local:3000');
    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Gib eine Adresse mit http:// oder https:// an.')).toBeVisible();
  });

  it('Enter in the Audnexus address saves like the save bar', async () => {
    const { calls } = mockApi(
      routes({
        'PATCH /admin/metadata/audiobooks': () =>
          json(sources({ audnexusUrl: 'http://audnexus.local:3000' })),
      }),
    );
    const user = userEvent.setup();
    await openPage();
    const field = await screen.findByRole('textbox', { name: 'Adresse von Audnexus' });
    await user.clear(field);
    await user.type(field, 'http://audnexus.local:3000{Enter}');
    await waitFor(() =>
      expect(calls.filter((call) => call.method === 'PATCH').map((call) => call.body)).toEqual([
        { audnexusUrl: 'http://audnexus.local:3000' },
      ]),
    );
    expect(await screen.findByText('Einstellungen für Hörbücher gespeichert.')).toBeInTheDocument();
  });

  it('names the source and the reason of an unknown failure', async () => {
    mockApi(
      routes({
        'GET /admin/metadata/audiobooks/review': () =>
          json([{ ...review[1], error: 'googlebooks:quota' }]),
      }),
    );
    await openPage();
    const table = await screen.findByRole('table', { name: 'Hörbücher zum Prüfen' });
    expect(table).toHaveTextContent('Google Books hat einen Fehler gemeldet.');
  });

  it('stores the Google Books key on its own', async () => {
    const { calls } = mockApi(
      routes({
        'PATCH /admin/metadata/audiobooks': () =>
          json(sources({ googleBooksKey: { source: 'stored', hint: 'wxyz' } })),
      }),
    );
    const user = userEvent.setup();
    await openPage();
    await user.type(
      await screen.findByLabelText('Schlüssel für Google Books', { selector: 'input' }),
      ' test-google-key-wxyz ',
    );
    await user.click(screen.getByRole('button', { name: 'Schlüssel für Google Books speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
        googleBooksKey: 'test-google-key-wxyz',
      }),
    );
    expect(await screen.findByText('Gespeichert, endet auf wxyz')).toBeVisible();
  });

  it('queues every book without match and says how many', async () => {
    const { calls } = mockApi(
      routes({ 'POST /admin/metadata/audiobooks/match-all': () => json({ queued: 16 }, 202) }),
    );
    const user = userEvent.setup();
    await openPage();
    await user.click(
      await screen.findByRole('button', { name: 'Jetzt alle ohne Zuordnung zuordnen' }),
    );
    expect(await screen.findByText('16 Hörbücher eingereiht.')).toBeInTheDocument();
    expect(calls.some((call) => call.path === '/admin/metadata/audiobooks/match-all')).toBe(true);
  });

  it('the pen opens the editor of the book; «Quellen ändern» the dialog of its folder', async () => {
    mockApi(
      routes({
        [`GET /admin/items/${audioIds.book}`]: () => json(adminBook),
        'GET /libraries/visible': () => json([]),
      }),
    );
    const user = userEvent.setup();
    await openPage();
    await user.click(
      await screen.findByRole('button', { name: 'Slouchcat Test Audiobook M4B bearbeiten' }),
    );
    const editor = await screen.findByRole('dialog', { name: 'Slouchcat Test Audiobook M4B' });
    expect(within(editor).getByText('Nicht zugeordnet: Angaben nur aus den Dateien')).toBeVisible();
    await user.click(within(editor).getByRole('button', { name: 'Schliessen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Quellen von /media/Hoerbuecher ändern' }));
    expect(await screen.findByRole('dialog', { name: 'Ordner von Hörbücher' })).toBeVisible();
  });

  it('a book that left «Prüfen» hands the focus to the pen of the next row', async () => {
    let matched = false;
    mockApi(
      routes({
        'GET /admin/metadata/audiobooks/review': () => json(matched ? [review[1]] : review),
        [`GET /admin/items/${audioIds.book}`]: () => json(adminBook),
        'GET /libraries/visible': () => json([]),
      }),
    );
    const user = userEvent.setup();
    await openPage();
    await user.click(
      await screen.findByRole('button', { name: 'Slouchcat Test Audiobook M4B bearbeiten' }),
    );
    const editor = await screen.findByRole('dialog', { name: 'Slouchcat Test Audiobook M4B' });
    // Matched in the editor: the server takes it off the list.
    matched = true;
    await user.click(within(editor).getByRole('button', { name: 'Schliessen' }));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Ein gestörtes Buch bearbeiten' })).toHaveFocus(),
    );
    expect(
      screen.queryByRole('button', { name: 'Slouchcat Test Audiobook M4B bearbeiten' }),
    ).not.toBeInTheDocument();
  });

  it('is linked from «Verwaltung › Metadaten»', async () => {
    mockApi(
      routes({
        'GET /admin/settings/metadata': () =>
          json({
            textLangs: ['de'],
            imageLangs: { poster: ['de'], backdrop: [''], logo: ['de'] },
            imageKinds: [],
            imageMinWidth: { poster: 0, backdrop: 0 },
            imageByVote: true,
            localImages: true,
            ratingCountry: 'DE',
            tmdbIncludeAdult: false,
            fanartEnabled: false,
            knownLanguages: ['de'],
            affectedTitles: 0,
            localImageTitles: 0,
          }),
        'GET /admin/job-types': () => json({ types: [] }),
      }),
    );
    await renderAdminPage('/admin/metadata');
    expect(await screen.findByRole('link', { name: 'Quellen für Hörbücher' })).toHaveAttribute(
      'href',
      '/admin/metadata/audiobooks',
    );
  });
});
