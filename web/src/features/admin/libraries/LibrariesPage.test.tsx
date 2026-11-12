import type { MetadataProviderInfo } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { library } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

// The page "Bibliotheken" on the pattern of concept 12 (variant C, Bib-C): filter tabs by kind,
// a row per library that folds out into actions, progress, error band and folders, and the
// dialog «Bibliothek hinzufügen». Every function of the old card page is checked here
// (Erhaltungsliste BIB-1 to BIB-6).

type User = ReturnType<typeof userEvent.setup>;

/** The row of a library: the toggle's table row. */
const rowOf = (name: string) => screen.getByRole('button', { name }).closest('tr')!;

/** Folds out the row of a library and returns its fold-out. */
async function openRow(user: User, name: string) {
  const toggle = await screen.findByRole('button', { name });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  return screen.findByRole('region', { name });
}

/** Opens «Bibliothek hinzufügen» from the page's head. */
async function openAdd(user: User) {
  await user.click(await screen.findByRole('button', { name: 'Bibliothek hinzufügen' }));
  return screen.findByRole('dialog', { name: 'Bibliothek hinzufügen' });
}

const books = library({
  id: 3,
  name: 'Hörbücher',
  kind: 'audiobooks',
  defaultRating: 0,
  roots: [
    {
      id: 3,
      path: '/media/Hoerbuecher',
      content: 'audiobook',
      providers: ['local'],
      fsType: 'ext4',
      watch: 'inotify',
      itemCount: 0,
      fileCount: 0,
    },
  ],
});
const providers: MetadataProviderInfo[] = [
  {
    id: 'tmdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['movie', 'series'],
    languages: ['de', 'en'],
  },
  {
    id: 'tvdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['series'],
    languages: ['de', 'en'],
  },
  { id: 'anilist', configured: true, roles: ['fill'], contents: ['series'], languages: ['en'] },
];

const podcasts = library({
  id: 4,
  name: 'Podcasts',
  kind: 'podcasts',
  defaultRating: 6,
  roots: [],
});

describe('admin libraries page (concept 12, Bib-C)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('renders scan state, counts, roots and the network share warning (BIB-1, BIB-2)', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([
          library(),
          library({
            id: 2,
            name: 'Serien',
            kind: 'video',
            roots: [
              {
                id: 2,
                path: '/media/Serien',
                content: 'series',
                providers: ['tmdb', 'tvdb'],
                fsType: 'nfs',
                watch: 'poll',
                warning: 'network_share',
                itemCount: 0,
                fileCount: 0,
              },
            ],
            scan: { state: 'failed', itemCount: 0, fileCount: 0, lastError: 'permission denied' },
          }),
        ]),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    await screen.findByRole('button', { name: 'Filme' });
    const movies = rowOf('Filme');
    expect(within(movies).getByText('Video')).toBeVisible();
    expect(within(movies).getByText('Aktuell')).toBeVisible();
    expect(within(movies).getByText(/^Letzter Scan:/)).toBeVisible();
    expect(within(movies).getByText('Titel: 12 · Dateien: 14')).toBeVisible();
    // The single path is cut to the column; the fold-out has it whole (S-5).
    expect(within(movies).getByText('/media/Filme')).toHaveAttribute('title', '/media/Filme');

    const series = rowOf('Serien');
    expect(within(series).getByText('Scan fehlgeschlagen')).toBeVisible();
    expect(within(series).getByText('Netzlaufwerk')).toBeVisible();

    const movieDetail = await openRow(user, 'Filme');
    expect(within(movieDetail).getByText('/media/Filme')).toBeVisible();
    expect(within(movieDetail).getByText('Änderungen werden sofort erkannt')).toBeVisible();
    expect(within(movieDetail).getByText('Dateisystem ext4')).toBeVisible();

    const seriesDetail = await openRow(user, 'Serien');
    expect(within(seriesDetail).getByText('Netzlaufwerk')).toBeVisible();
    expect(within(seriesDetail).getByText('Dateisystem nfs')).toBeVisible();
    expect(within(seriesDetail).getByText('TMDB › TVDB')).toBeVisible();
    expect(within(seriesDetail).getByText('Letzter Fehler: permission denied')).toBeVisible();
  });

  it('shows the keyframe index coverage of a video library and links the backfill (concept 14, 7.2)', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([
          library(),
          library({
            id: 2,
            name: 'Serien',
            keyframeIndex: { videoFiles: 2419, indexed: 2112, pending: 307, unavailable: 0 },
          }),
          library({
            id: 5,
            name: 'Konzerte',
            keyframeIndex: { videoFiles: 10, indexed: 7, pending: 0, unavailable: 3 },
          }),
          books,
        ]),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const movies = await openRow(user, 'Filme');
    expect(within(movies).getByText('Keyframe-Index vollständig')).toBeVisible();
    expect(within(movies).queryByRole('link', { name: /Nachholen/ })).toBeNull();

    // The numbers as the page formats them (Intl of the language), every part named.
    const series = await openRow(user, 'Serien');
    const formatted = new Intl.NumberFormat('de');
    expect(
      within(series).getByText(
        `Keyframe-Index: ${formatted.format(2112)} von ${formatted.format(2419)} Videodateien · 307 ausstehend · 0 nicht möglich`,
      ),
    ).toBeVisible();
    expect(
      within(series).getByRole('link', { name: 'Nachholen auf der Seite Hintergrundjobs' }),
    ).toHaveAttribute('href', '/admin/jobs?item=keyframes.backfill');

    // Nothing waits: no link, though some files can never be indexed.
    const concerts = await openRow(user, 'Konzerte');
    expect(
      within(concerts).getByText(
        'Keyframe-Index: 7 von 10 Videodateien · 0 ausstehend · 3 nicht möglich',
      ),
    ).toBeVisible();
    expect(within(concerts).queryByRole('link', { name: /Nachholen/ })).toBeNull();

    // An audio library has no coverage.
    const audio = await openRow(user, 'Hörbücher');
    expect(within(audio).queryByText(/Keyframe-Index/)).toBeNull();
  });

  it('filters by kind with counts in the tabs and keeps the tab in the address', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([library(), books, podcasts]) });
    const { router } = renderApp('/admin/libraries?tab=nonsense');
    const user = userEvent.setup();

    const tabs = await screen.findByRole('tablist', { name: 'Bibliotheken nach Art' });
    expect(within(tabs).getByRole('tab', { name: 'Alle 3 Bibliotheken' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(within(tabs).getByRole('tab', { name: 'Video 1 Bibliothek' })).toBeVisible();
    expect(within(tabs).getByRole('tab', { name: 'Hörbücher 1 Bibliothek' })).toBeVisible();
    expect(within(tabs).getByRole('tab', { name: 'Podcasts 1 Bibliothek' })).toBeVisible();
    // An unknown tab is replaced by the first one, in the address too.
    await waitFor(() => expect(router.state.location.search).toEqual({}));

    await user.click(within(tabs).getByRole('tab', { name: 'Hörbücher 1 Bibliothek' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'audiobooks' }));
    expect(screen.getByRole('button', { name: 'Hörbücher' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Filme' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Podcasts' })).toBeNull();
  });

  it('opens on the tab the address names', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([library(), books, podcasts]) });
    renderApp('/admin/libraries?tab=podcasts');

    expect(await screen.findByRole('tab', { name: 'Podcasts 1 Bibliothek' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByRole('button', { name: 'Podcasts' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Filme' })).toBeNull();
  });

  it('names the metadata deviation of a video library and links to it (BIB-3)', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library(), books]),
      'GET /admin/libraries/1/metadata': () =>
        json({
          textLangs: ['en'],
          effective: { textLangs: ['en'], imageLangs: {}, imageKinds: [] },
        }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    await screen.findByRole('button', { name: 'Filme' });
    expect(await within(rowOf('Filme')).findByText('Sprachen abweichend')).toBeVisible();
    // An audio library has no metadata settings of its own.
    // The dash is for the eye; a screen reader hears what it means.
    expect(within(rowOf('Hörbücher')).getByText('–')).toHaveAttribute('aria-hidden', 'true');
    expect(within(rowOf('Hörbücher')).getByText('Keine Metadaten-Geltung')).toBeInTheDocument();

    const detail = await openRow(user, 'Filme');
    expect(within(detail).getByRole('link', { name: 'Metadaten der Bibliothek' })).toHaveAttribute(
      'href',
      '/admin/metadata?library=1',
    );
    const audio = await openRow(user, 'Hörbücher');
    expect(within(audio).queryByRole('link', { name: 'Metadaten der Bibliothek' })).toBeNull();
  });

  it('names «wie Server», «Bilder abweichend» and both deviations (BIB-3)', async () => {
    const effective = { textLangs: ['de'], imageLangs: {}, imageKinds: [] };
    const images = { langs: {}, kinds: ['logo'] };
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([
          library({ id: 1, name: 'Filme' }),
          library({ id: 3, name: 'Serien' }),
          library({ id: 4, name: 'Dokus' }),
        ]),
      'GET /admin/libraries/1/metadata': () => json({ libraryId: 1, effective }),
      'GET /admin/libraries/3/metadata': () =>
        json({ libraryId: 3, textLangs: null, images, effective }),
      'GET /admin/libraries/4/metadata': () =>
        json({ libraryId: 4, textLangs: ['en'], images, effective }),
    });
    renderApp('/admin/libraries');

    await screen.findByRole('button', { name: 'Dokus' });
    expect(await within(rowOf('Filme')).findByText('wie Server')).toBeVisible();
    expect(await within(rowOf('Serien')).findByText('Bilder abweichend')).toBeVisible();
    expect(await within(rowOf('Dokus')).findByText('Sprachen und Bilder abweichend')).toBeVisible();
  });

  it('explains an aborted scan and accepts the loss after confirmation (BIB-4)', async () => {
    const aborted = library({
      scan: { state: 'aborted', itemCount: 12, fileCount: 2, lastError: 'loss_over_threshold' },
    });
    let current = aborted;
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([current]),
      'POST /admin/libraries/1/accept-loss': () => {
        current = { ...aborted, scan: { ...aborted.scan, state: 'queued' } };
        return json(current, 202);
      },
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const detail = await openRow(user, 'Filme');
    expect(within(detail).getByText(/Mehr als 20 % der Dateien fehlen/)).toBeVisible();
    // The danger sits in «Weitere Aktionen» at the bottom (S-6), with its confirmation.
    await user.click(within(detail).getByRole('button', { name: 'Weitere Aktionen für Filme' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Verlust akzeptieren' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Fehlende Dateien entfernen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Entfernen und scannen' }));

    await waitFor(() =>
      expect(calls.some((call) => call.path === '/admin/libraries/1/accept-loss')).toBe(true),
    );
    // The menu went with the loss: the h1 has the focus (ALL-2).
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toHaveFocus(),
    );
    expect(within(detail).queryByRole('button', { name: 'Weitere Aktionen für Filme' })).toBeNull();
  });

  it('offers no loss acceptance once the loss is accepted, like the overview (concept 11)', async () => {
    // The server's `attention.abortedScans` leaves out an accepted loss; the menu does as well.
    const accepted = library({
      scan: {
        state: 'aborted',
        itemCount: 12,
        fileCount: 2,
        lastError: 'loss_over_threshold',
        lossAcceptedAt: '2026-10-21T09:00:00Z',
      },
    });
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([accepted]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const detail = await openRow(user, 'Filme');
    expect(within(detail).getByText(/Mehr als 20 % der Dateien fehlen/)).toBeVisible();
    // A video library without a loss to accept has nothing more to offer.
    expect(within(detail).queryByRole('button', { name: 'Weitere Aktionen für Filme' })).toBeNull();
  });

  it('starts a rescan, polls every 5 s while the scan runs and shows «Live» meanwhile', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let state: 'done' | 'running' = 'done';
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([
          library({ scan: { state, itemCount: 3, fileCount: 3, filesSeen: 4, filesTotal: 10 } }),
        ]),
      'POST /admin/libraries/1/scan': () => {
        state = 'running';
        return json(library({ scan: { state, itemCount: 3, fileCount: 3 } }), 202);
      },
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    const detail = await openRow(user, 'Filme');
    // No scan, no polling: no live indicator either (S-2).
    expect(screen.queryByText(/^Live/)).toBeNull();
    const button = within(detail).getByRole('button', { name: 'Erneut scannen' });
    await user.click(button);

    expect(await within(rowOf('Filme')).findByText('Scan läuft')).toBeVisible();
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    expect(within(detail).getByRole('progressbar', { name: '4 von 10 Dateien' })).toBeVisible();
    // The same button now cancels, and it kept the focus.
    expect(within(detail).getByRole('button', { name: 'Scan abbrechen' })).toHaveFocus();
    const before = calls.filter((call) => call.path === '/admin/libraries').length;
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() =>
      expect(calls.filter((call) => call.path === '/admin/libraries').length).toBeGreaterThan(
        before,
      ),
    );
  });

  it('holds the polling while a dialog is open, not while a closed draft waits', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([library({ scan: { state: 'running', itemCount: 3, fileCount: 3 } })]),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    expect(await screen.findByText('Live, alle 5 Sekunden')).toBeVisible();
    const lists = () => calls.filter((call) => call.path === '/admin/libraries').length;

    const dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Dokus');
    const before = lists();
    await act(() => vi.advanceTimersByTimeAsync(12_000));
    expect(lists()).toBe(before);

    // Closed with the draft kept (Escape does not ask): the scan progress stays live.
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(await screen.findByText('Live, alle 5 Sekunden')).toBeVisible();
    const closed = lists();
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(lists()).toBeGreaterThan(closed));
  });

  it('adds a library and shows server errors at the path field (BIB-6)', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([]),
      'POST /admin/libraries': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'roots[0].path', code: 'library_path_invalid' }],
        }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    expect(await screen.findByText('Noch keine Bibliothek angelegt.')).toBeVisible();

    const dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Dokus');
    await user.click(within(dialog).getByRole('radio', { name: 'Video' }));
    await user.type(within(dialog).getByLabelText('Ordner'), '/media/Dokus');
    // The submit counts the cards of the folder list (F-13).
    await user.click(
      within(dialog).getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
    );

    await waitFor(() =>
      expect(within(dialog).getByLabelText('Ordner')).toHaveAccessibleDescription(
        /Diesen Ordner gibt es auf dem Server nicht/,
      ),
    );
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      name: 'Dokus',
      kind: 'video',
      roots: [{ path: '/media/Dokus', content: 'movie', providers: ['tmdb'] }],
      trickplay: {
        movies: { mode: 'off', keep: true },
        series: { mode: 'off', keep: true, scope: 'series' },
      },
    });
  });

  it('sets «Vorschau beim Spulen» of a new video library, films and series apart (concept 18, 9)', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([]),
      'POST /admin/libraries': () => json(library({ name: 'Dokus' }), 201),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Dokus');
    await user.type(within(dialog).getByLabelText('Ordner'), '/media/Dokus');
    const movies = within(dialog).getByRole('radiogroup', { name: 'Filme' });
    const series = within(dialog).getByRole('radiogroup', { name: 'Serien' });
    // «Aus» first and chosen (1.23.1), in both groups.
    expect(
      within(movies)
        .getAllByRole('radio')
        .map((radio) => radio.getAttribute('value')),
    ).toEqual(['off', 'ahead', 'on_start']);
    expect(
      within(series)
        .getAllByRole('radio')
        .map((radio) => radio.getAttribute('value')),
    ).toEqual(['off', 'ahead', 'on_start', 'series_start']);
    expect(within(movies).getByRole('radio', { name: 'Aus' })).toBeChecked();
    expect(within(series).getByRole('radio', { name: 'Aus' })).toBeChecked();
    // Films have no «Beim Start alle Folgen».
    expect(within(movies).queryByRole('radio', { name: 'Beim Start alle Folgen' })).toBeNull();
    await user.click(within(movies).getByText('Beim Start'));
    await user.click(within(dialog).getByRole('switch', { name: 'Speichern' }));
    await user.click(within(series).getByText('Beim Start alle Folgen'));
    const seriesGroup = dialog.querySelector<HTMLElement>('[data-trickplay-group="series"]')!;
    expect(within(seriesGroup).queryByRole('switch')).toBeNull();
    await user.click(within(dialog).getByText('Rest der Staffel'));
    expect(within(dialog).getByText(/die der übrigen Folgen der Staffel/)).toBeVisible();
    await user.click(
      within(dialog).getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
    );
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toMatchObject({
        trickplay: {
          movies: { mode: 'on_start', keep: false },
          series: { mode: 'series_start', keep: true, scope: 'season_rest' },
        },
      }),
    );
  });

  it('changes «Vorschau beim Spulen» with the estimate, warns before dropping and generates now', async () => {
    const estimateKind = (titles: number, perTitleBytes: number, storedBytes: number) => ({
      titles,
      files: titles,
      durationMs: titles * 6_840_000,
      perTitleBytes,
      libraryBytes: titles * perTitleBytes,
      storedBytes,
    });
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library()]),
      'GET /admin/libraries/1': () =>
        json({
          ...library(),
          trickplayEstimate: {
            movies: estimateKind(412, 2_900_000, 1_200_000_000),
            series: estimateKind(0, 0, 0),
            cacheFreeBytes: 33_000_000_000,
          },
        }),
      'POST /admin/job-types/trickplay.backfill/run': () => json([], 202),
      'PATCH /admin/libraries/1': ({ body }) => json({ ...library(), ...(body as object) }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const detail = await openRow(user, 'Filme');
    expect(
      within(detail).getByText('Vorschau beim Spulen: Filme Im Voraus · Serien Im Voraus'),
    ).toBeVisible();
    await user.click(
      within(detail).getByRole('button', { name: 'Vorschau beim Spulen von Filme' }),
    );
    const dialog = await screen.findByRole('dialog', { name: 'Vorschau beim Spulen: Filme' });
    const movies = dialog.querySelector<HTMLElement>('[data-trickplay-group="movies"]')!;
    await waitFor(() => expect(within(movies).getByText('412 Filme, 783 h')).toBeVisible());
    // One decimal below 10 MB, so 2.9 MB does not read «3 MB».
    expect(within(movies).getByText('Pro Film').nextSibling).toHaveTextContent(/^≈ 2[.,]9 MB$/);
    // A kind with nothing stored reads «0 MB», not «0 Byte».
    expect(within(dialog).getByText(/^Heute belegt: Filme .* · Serien 0 MB$/)).toBeVisible();
    expect(within(movies).getByText('Ganze Bibliothek').nextSibling).toHaveTextContent(
      /^≈ 1[.,]2 GB$/,
    );
    expect(within(dialog).getByText('Freier Platz im Cache: 33 GB')).toBeVisible();

    await user.click(within(dialog).getByRole('button', { name: 'Jetzt erzeugen' }));
    expect(
      await within(dialog).findByText('Das Nachholen der Vorschauen ist eingereiht.'),
    ).toBeVisible();

    // «Beim Start» with «Speichern»: «höchstens»; without it only during playback, 0 for the
    // library, and the stored previews go.
    await user.click(within(movies).getByText('Beim Start'));
    expect(within(movies).getByText('Ganze Bibliothek, höchstens')).toBeVisible();
    await user.click(within(dialog).getByRole('switch', { name: 'Speichern' }));
    expect(within(movies).getByText('Pro Film, nur während der Wiedergabe')).toBeVisible();
    expect(within(movies).getByText('Ganze Bibliothek').nextSibling).toHaveTextContent('0 MB');
    expect(
      within(dialog).getByText(/gespeicherten Vorschauen der Filme gelöscht \(1[.,]2 GB\)/),
    ).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
      trickplay: {
        movies: { mode: 'on_start', keep: false },
        series: { mode: 'ahead', keep: true, scope: 'series' },
      },
    });
    expect(screen.getByText('Die Vorschau beim Spulen von Filme ist gespeichert.')).toBeVisible();
  });

  it('keeps what was typed when the dialog is closed without adding (S-13)', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([library()]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    let dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Dokus');
    await user.type(within(dialog).getByLabelText('Ordner'), '/media/Dokus');
    // Escape keeps the draft without asking.
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    // The focus is back on the button that opened it.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Bibliothek hinzufügen' })).toHaveFocus(),
    );

    dialog = await openAdd(user);
    expect(within(dialog).getByLabelText('Name')).toHaveValue('Dokus');
    expect(within(dialog).getByLabelText('Ordner')).toHaveValue('/media/Dokus');
  });

  it('asks on «Abbrechen» whether to keep the draft; the band offers it again (concept 20, 13.2)', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([library()]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const band = () =>
      screen.queryByText('Der Entwurf einer neuen Bibliothek ist noch nicht gespeichert.');

    let dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Dokus');
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    let question = await screen.findByRole('alertdialog', {
      name: 'Entwurf behalten oder verwerfen?',
    });
    // «Entwurf behalten» is where Enter by habit lands.
    expect(within(question).getByRole('button', { name: 'Entwurf behalten' })).toHaveFocus();
    await user.click(within(question).getByRole('button', { name: 'Entwurf behalten' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(band()).toBeVisible();

    // «Weiter bearbeiten» opens the dialog with the draft; the band is gone meanwhile.
    await user.click(screen.getByRole('button', { name: 'Weiter bearbeiten' }));
    dialog = await screen.findByRole('dialog', { name: 'Bibliothek hinzufügen' });
    expect(within(dialog).getByLabelText('Name')).toHaveValue('Dokus');
    expect(band()).toBeNull();

    // «Entwurf verwerfen» in the question empties it: no band, the next opening is empty.
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    question = await screen.findByRole('alertdialog', { name: 'Entwurf behalten oder verwerfen?' });
    await user.click(within(question).getByRole('button', { name: 'Entwurf verwerfen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(band()).toBeNull();
    dialog = await openAdd(user);
    expect(within(dialog).getByLabelText('Name')).toHaveValue('');

    // Untouched, «Abbrechen» closes without asking.
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(screen.queryByRole('alertdialog')).toBeNull();
  });

  it('discards the draft from the band; leaving then does not ask (concept 20, 13.2)', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([library()]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const nav = () => screen.getByRole('navigation', { name: 'Verwaltung' });

    const dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Dokus');
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    // react-aria hands the focus back to the opener first.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Bibliothek hinzufügen' })).toHaveFocus(),
    );
    await user.click(screen.getByRole('button', { name: 'Entwurf verwerfen' }));
    expect(
      screen.queryByText('Der Entwurf einer neuen Bibliothek ist noch nicht gespeichert.'),
    ).toBeNull();
    expect(screen.getByText('Der Entwurf der neuen Bibliothek ist verworfen.')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toHaveFocus(),
    );
    await user.click(within(nav()).getByRole('link', { name: /^Integrationen/ }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  });

  it('a single folder card without a path is no draft, whatever its content (concept 20, 13.2)', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([library()]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openAdd(user);
    // A second card (for series), then the first one out: one empty card for series is left.
    await user.click(within(dialog).getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
    await user.click(within(dialog).getByRole('button', { name: 'Ordner 1 entfernen' }));
    expect(within(dialog).getByRole('radio', { name: 'Serien' })).toBeChecked();
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(
      screen.queryByText('Der Entwurf einer neuen Bibliothek ist noch nicht gespeichert.'),
    ).toBeNull();
  });

  it('orders the sources of a new folder by position (S-16)', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([]),
      'POST /admin/libraries': (call) => json(library(call.body as never), 201),
      'GET /admin/metadata/providers': () => json(providers),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    await screen.findByText('Noch keine Bibliothek angelegt.');

    const dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Serien');
    await user.type(within(dialog).getByLabelText('Ordner'), '/media/Serien');
    await user.click(within(dialog).getByRole('radio', { name: 'Serien' }));
    // No arrows any more: the position is a choice.
    expect(within(dialog).queryByRole('button', { name: /nach oben/ })).toBeNull();
    await user.selectOptions(within(dialog).getByLabelText('Platz für TVDB'), 'Platz 1');
    await user.click(
      within(dialog).getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
    );
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
        name: 'Serien',
        kind: 'video',
        roots: [{ path: '/media/Serien', content: 'series', providers: ['tvdb', 'tmdb'] }],
        trickplay: {
          movies: { mode: 'off', keep: true },
          series: { mode: 'off', keep: true, scope: 'series' },
        },
      }),
    );
  });

  it('cancels a running scan; a cancelled scan has its own text and no loss action', async () => {
    const running = library({
      scan: { state: 'running', itemCount: 3, fileCount: 4, filesSeen: 40, filesTotal: 100 },
    });
    let current = running;
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([current]),
      'DELETE /admin/libraries/1/scan': () => {
        current = library({
          scan: { state: 'aborted', itemCount: 3, fileCount: 4, lastError: 'cancelled' },
        });
        return json(current, 202);
      },
    });
    renderApp('/admin/libraries');

    const detail = await openRow(user, 'Filme');
    expect(within(detail).getByText('40 von 100 Dateien')).toBeVisible();
    expect(within(rowOf('Filme')).getByText('Titel: 3 · Dateien: 4')).toBeVisible();
    await user.click(within(detail).getByRole('button', { name: 'Scan abbrechen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/libraries/1/scan'),
      ).toBe(true),
    );
    expect(
      await within(detail).findByText(
        'Der Scan wurde abgebrochen. Der nächste Scan läuft wieder normal.',
      ),
    ).toBeVisible();
    expect(within(detail).queryByRole('button', { name: 'Scan abbrechen' })).toBeNull();
    expect(within(detail).getByRole('button', { name: 'Erneut scannen' })).toBeEnabled();
    expect(within(detail).queryByRole('button', { name: 'Weitere Aktionen für Filme' })).toBeNull();
  });

  it('audio libraries: default rating in the row, changed through PATCH (AN-2, M6)', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library(), books, podcasts]),
      'PATCH /admin/libraries/3': () => json({ ...books, defaultRating: 12 }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    await screen.findByRole('button', { name: 'Hörbücher' });
    expect(within(rowOf('Hörbücher')).getByText('Standard: FSK 0')).toBeVisible();
    expect(within(rowOf('Podcasts')).getByText('Standard: FSK 6')).toBeVisible();
    // A podcast library without folders names its feeds, not «0 Ordner».
    expect(within(rowOf('Podcasts')).getByText('Titel aus Feeds')).toBeVisible();
    expect(within(rowOf('Podcasts')).queryByText('0 Ordner')).toBeNull();
    // Video libraries have no default rating.
    expect(within(rowOf('Filme')).queryByText(/^Standard:/)).toBeNull();
    const podcastDetail = await openRow(user, 'Podcasts');
    expect(
      within(podcastDetail).getByText('Kein Ordner: Die Titel kommen aus abonnierten Feeds.'),
    ).toBeVisible();
    const movieDetail = await openRow(user, 'Filme');
    expect(
      within(movieDetail).queryByRole('button', { name: 'Weitere Aktionen für Filme' }),
    ).toBeNull();

    const detail = await openRow(user, 'Hörbücher');
    const more = within(detail).getByRole('button', { name: 'Weitere Aktionen für Hörbücher' });
    await user.click(more);
    await user.click(await screen.findByRole('menuitem', { name: 'Standardstufe ändern' }));
    const dialog = await screen.findByRole('dialog', { name: 'Standardstufe von Hörbücher' });
    expect(within(dialog).getByRole('radio', { name: 'FSK 0' })).toBeChecked();
    await user.click(within(dialog).getByText('FSK 12'));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({ defaultRating: 12 });
    expect(screen.getByText('Die Standardstufe von Hörbücher ist jetzt FSK 12.')).toBeVisible();
    await waitFor(() => expect(more).toHaveFocus());
  });

  it('adds a podcast library without folder and an audiobook library with its rating', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([]),
      'POST /admin/libraries': (call) => json(library(call.body as never), 201),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    await screen.findByText('Noch keine Bibliothek angelegt.');

    let dialog = await openAdd(user);
    // Video: no default rating to choose.
    expect(within(dialog).queryByRole('radiogroup', { name: 'Standard-Altersstufe' })).toBeNull();
    await user.type(within(dialog).getByLabelText('Name'), 'Podcasts');
    await user.click(within(dialog).getByRole('radio', { name: 'Podcasts' }));
    expect(within(dialog).queryByLabelText('Ordner')).not.toBeInTheDocument();
    expect(within(dialog).getByText(/Ordner optional/)).toBeVisible();
    const rating = within(dialog).getByRole('radiogroup', { name: 'Standard-Altersstufe' });
    expect(within(rating).getByRole('radio', { name: 'FSK 0' })).toBeChecked();
    await user.click(within(rating).getByText('FSK 6'));
    await user.click(
      within(dialog).getByRole('button', { name: 'Bibliothek ohne Ordner hinzufügen' }),
    );
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
        name: 'Podcasts',
        kind: 'podcasts',
        roots: [],
        defaultRating: 6,
      }),
    );
    // The dialog closes and the page says what happened.
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(screen.getByText('Die Bibliothek Podcasts ist angelegt.')).toBeVisible();

    // A new dialog starts empty.
    dialog = await openAdd(user);
    expect(within(dialog).getByLabelText('Name')).toHaveValue('');
    await user.type(within(dialog).getByLabelText('Name'), 'Hörbücher');
    await user.click(within(dialog).getByRole('radio', { name: 'Hörbücher' }));
    await user.type(within(dialog).getByLabelText('Ordner'), '/media/Hoerbuecher');
    await user.click(
      within(dialog).getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
    );
    await waitFor(() =>
      expect(calls.filter((call) => call.method === 'POST').at(-1)?.body).toEqual({
        name: 'Hörbücher',
        kind: 'audiobooks',
        roots: [{ path: '/media/Hoerbuecher', content: 'audiobook', providers: ['local'] }],
        defaultRating: 0,
      }),
    );
  });
  it('ignores a cancel right after the same button started the scan; both are announced', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let current = library();
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([current]),
      'POST /admin/libraries/1/scan': () => {
        current = library({ scan: { state: 'queued', itemCount: 12, fileCount: 14 } });
        return json(current, 202);
      },
      'DELETE /admin/libraries/1/scan': () => {
        current = library({
          scan: { state: 'aborted', itemCount: 12, fileCount: 14, lastError: 'cancelled' },
        });
        return json(current, 202);
      },
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    const detail = await openRow(user, 'Filme');

    // A double press: the second press lands on «Scan abbrechen» and is ignored.
    await user.dblClick(within(detail).getByRole('button', { name: 'Erneut scannen' }));
    expect(await screen.findByText('Der Scan von Filme ist gestartet.')).toBeVisible();
    const cancel = within(detail).getByRole('button', { name: 'Scan abbrechen' });
    await user.click(cancel);
    expect(
      calls.filter((call) => call.method === 'POST' && call.path.endsWith('/scan')),
    ).toHaveLength(1);
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);

    // A second later the button cancels again.
    await act(() => vi.advanceTimersByTimeAsync(1100));
    await user.click(cancel);
    expect(await screen.findByText('Der Scan von Filme ist abgebrochen.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'DELETE')).toHaveLength(1);
    // The cancelled scan is not announced a second time as stopped.
    expect(screen.queryByText(/wurde gestoppt/)).toBeNull();
  });

  it('announces a scan that fails while the page watches', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let state: 'running' | 'failed' = 'running';
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([
          library({
            scan: {
              state,
              itemCount: 3,
              fileCount: 3,
              ...(state === 'failed' ? { lastError: 'permission denied' } : {}),
            },
          }),
        ]),
    });
    renderApp('/admin/libraries');
    expect(
      await within(await waitFor(() => rowOf('Filme'))).findByText('Scan läuft'),
    ).toBeVisible();

    state = 'failed';
    await act(() => vi.advanceTimersByTimeAsync(5000));
    const status = await screen.findByText('Der Scan von Filme ist fehlgeschlagen.');
    expect(status.closest('[role="status"]')).not.toBeNull();
  });

  it('asks before leaving the page while the add dialog holds a draft (S-13)', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([library()]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const nav = () => screen.getByRole('navigation', { name: 'Verwaltung' });

    // Opened and closed untouched: no draft, no question.
    let dialog = await openAdd(user);
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await user.click(within(nav()).getByRole('link', { name: /^Integrationen/ }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
    await user.click(within(nav()).getByRole('link', { name: /^Bibliotheken/ }));

    // A typed name is a draft: leaving asks, staying keeps it.
    dialog = await openAdd(user);
    await user.type(within(dialog).getByLabelText('Name'), 'Dokus');
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await user.click(within(nav()).getByRole('link', { name: /^Integrationen/ }));
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
    dialog = await openAdd(user);
    expect(within(dialog).getByLabelText('Name')).toHaveValue('Dokus');

    // Typed and deleted again is no draft any more.
    await user.clear(within(dialog).getByLabelText('Name'));
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await user.click(within(nav()).getByRole('link', { name: /^Integrationen/ }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  });

  it('walks a menu of two entries with the arrow keys; Escape returns to its button', async () => {
    const aborted = library({
      ...books,
      scan: { state: 'aborted', itemCount: 1, fileCount: 1, lastError: 'loss_over_threshold' },
    });
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([aborted]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const detail = await openRow(user, 'Hörbücher');
    const more = within(detail).getByRole('button', { name: 'Weitere Aktionen für Hörbücher' });
    more.focus();
    await user.keyboard('{Enter}');
    const rating = await screen.findByRole('menuitem', { name: 'Standardstufe ändern' });
    const loss = screen.getByRole('menuitem', { name: 'Verlust akzeptieren' });
    // «Gefahr» at the bottom.
    expect(screen.getByRole('group', { name: 'Gefahr' })).toContainElement(loss);
    await waitFor(() => expect(rating).toHaveFocus());
    await user.keyboard('{ArrowDown}');
    expect(loss).toHaveFocus();
    await user.keyboard('{ArrowUp}');
    expect(rating).toHaveFocus();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull());
    await waitFor(() => expect(more).toHaveFocus());
  });
});
