import type {
  MetadataProviderInfo,
  MetadataSettings,
  MetadataSettingsUpdate,
} from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { library, ratingSystems } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn, type Call, type Routes } from '@/test/api';
import { afterDialogsClosed } from '@/ui/aria';

// The page "Metadaten" (concept 10, 7.1; concept 24 3.4): the default every folder without
// settings of its own follows; the settings areas as fold sections, all closed, each header with
// its summary and «geändert»; the language places, the image kinds and chains, quality, sources,
// rating country, the apply run per folder and the video folders; one save bar.

const settings = (overrides: Partial<MetadataSettings> = {}): MetadataSettings => ({
  textLangs: ['de', 'en'],
  imageLangs: { poster: ['de', 'en', ''], backdrop: ['', 'de', 'en'], logo: ['de', 'en'] },
  imageKinds: ['logo', 'season_poster', 'episode_still'],
  imageMinWidth: { poster: 500, backdrop: 1280 },
  imageByVote: true,
  localImages: true,
  ratingCountry: 'DE',
  ratingSystems,
  tmdbIncludeAdult: false,
  fanartEnabled: false,
  knownLanguages: ['de', 'en', 'es', 'fr', 'ja'],
  affectedTitles: 1234,
  localImageTitles: 211,
  ...overrides,
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
    configured: false,
    roles: ['identify', 'fill'],
    contents: ['series'],
    languages: ['de', 'en'],
  },
  { id: 'anilist', configured: true, roles: ['fill'], contents: ['series'], languages: ['en'] },
  { id: 'local', configured: true, roles: ['fill'], contents: ['movie', 'series'], languages: [] },
];

const libraries = [
  library(),
  library({
    id: 2,
    name: 'Serien',
    roots: [{ ...library().roots[0]!, id: 2, path: '/media/Serien', content: 'series' }],
  }),
  library({
    id: 3,
    name: 'Hörbücher',
    kind: 'audiobooks',
    roots: [{ ...library().roots[0]!, id: 3, path: '/media/Hoerbuecher', content: 'audiobook' }],
  }),
];

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/settings/metadata': () => json(settings()),
  'GET /admin/libraries': () => json(libraries),
  'GET /admin/metadata/providers': () => json(providers),
  'GET /admin/job-types': () =>
    json({
      types: [
        { type: 'metadata.match', counts: { queued: 3, planned: 0, running: 0, failed: 0 } },
        { type: 'image.fetch', counts: { queued: 7, planned: 0, running: 0, failed: 0 } },
      ],
    }),
  ...extra,
});

/** A server that keeps the default it is sent. */
function statefulRoutes() {
  let server = settings();
  return routes({
    'GET /admin/settings/metadata': () => json(server),
    'PUT /admin/settings/metadata': (call) => {
      server = settings({ ...(call.body as Partial<MetadataSettings>) });
      return json(server);
    },
  });
}

const region = (name: string) => screen.getByRole('region', { name });
const saveBar = () => region('Speichern');

/** The values of the places in a language list, place 1 first ("" = Ohne Text). */
const placesOf = (list: HTMLElement) =>
  within(list)
    .getAllByRole<HTMLSelectElement>('combobox')
    .map((select) => select.value);

/** The names of the choices a place select has switched off. */
const disabledOf = (select: HTMLElement) =>
  Array.from((select as HTMLSelectElement).options)
    .filter((option) => option.disabled)
    .map((option) => option.textContent);

const puts = (calls: Call[], path: string) =>
  calls.filter((call) => call.method === 'PUT' && call.path === path).map((call) => call.body);

/** The toggle of a fold section: the button in its h2. */
const toggleOf = (name: string) =>
  within(screen.getByRole('heading', { level: 2, name })).getByRole('button', { name });

/** The header line of a fold section: title, summary and «geändert». */
const headerOf = (name: string) => screen.getByRole('heading', { level: 2, name }).parentElement!;

/** Opens a fold section and returns its content (react-aria's panel, named by the toggle). */
async function openArea(user: ReturnType<typeof userEvent.setup>, name: string) {
  const toggle = await screen.findByRole('button', { name });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  return screen.getByRole('group', { name });
}

/** The content of an open section. */
const sectionOf = (name: string) => screen.findByRole('group', { name });

describe('metadata settings page (concept 10, concept 24 3.4)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('«Speichern» without a change sends nothing and keeps the focus', async () => {
    const { calls } = mockApi(routes());
    renderApp('/admin/metadata');
    await screen.findByRole('heading', { level: 2, name: 'Sprachen' });
    const user = userEvent.setup();
    const save = within(saveBar()).getByRole('button', { name: 'Speichern' });
    await user.click(save);
    expect(save).toHaveFocus();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('puts the SaveBar into the page column, so it sticks while the sections scroll', async () => {
    mockApi(routes());
    renderApp('/admin/metadata');
    const languages = await screen.findByRole('heading', { level: 2, name: 'Sprachen' });
    // position: sticky holds an element inside its parent: the parent must be the long column
    // that holds the h1 and the areas, not a wrapper only as tall as the bar.
    const column = screen.getByRole('region', { name: 'Speichern' }).parentElement;
    expect(column).toContainElement(screen.getByRole('heading', { level: 1, name: 'Metadaten' }));
    expect(column).toContainElement(languages);
  });

  it('shows the server default: areas, languages in order, image kinds, chains, quality, sources, country, apply', async () => {
    mockApi(routes());
    renderApp('/admin/metadata');
    expect(await screen.findByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();
    // The intro stands open below the title (build plan 2.6), no help button.
    expect(screen.queryByRole('button', { name: 'Hilfe zu dieser Seite' })).toBeNull();

    expect(
      screen.getByText(
        'Gilt für jeden Ordner ohne eigene Einstellung. Eigene Sprachen und Bilder eines Ordners stellst du unter «Bearbeiten» seiner Bibliothek ein.',
      ),
    ).toBeVisible();
    // Seven fold sections, all closed, each header with its summary (concept 24 3.4).
    const sections = await screen.findAllByRole('heading', { level: 2 });
    expect(sections.map((heading) => heading.textContent)).toEqual([
      'Sprachen',
      'Bilder',
      'Bildqualität',
      'Quellen',
      'Altersfreigabe',
      'Anwenden',
      'Ordner',
    ]);
    for (const heading of sections)
      expect(within(heading).getByRole('button')).toHaveAttribute('aria-expanded', 'false');
    expect(headerOf('Sprachen')).toHaveTextContent(/^Sprachen.*Deutsch › Englisch$/);
    expect(headerOf('Bilder')).toHaveTextContent(
      'Poster, Hintergrund, Logo, Staffelposter und Folgenbild',
    );
    expect(headerOf('Bildqualität')).toHaveTextContent('Poster 500 px, Hintergrund 1280 px');
    expect(headerOf('Quellen')).toHaveTextContent('Lokale Dateien');
    expect(headerOf('Altersfreigabe')).toHaveTextContent('Deutschland');
    expect(headerOf('Anwenden')).toHaveTextContent('Texte und Bilder · Alle Ordner (1234 Titel)');
    expect(headerOf('Ordner')).toHaveTextContent(
      '2 Video-Ordner folgen dem Standard, 0 haben Eigenes',
    );
    // No list of areas and no scope any more (concept 24 3.4): the page edits the default only.
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(screen.queryByLabelText('Geltung')).toBeNull();
    const user = userEvent.setup();
    const languages = await openArea(user, 'Sprachen');
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    expect(within(list).getAllByRole('listitem')).toHaveLength(2);
    // Fixed places, each a select of the languages (10.2).
    expect(within(list).getByRole('combobox', { name: '1. Sprache' })).toHaveValue('de');
    expect(within(list).getByRole('combobox', { name: '2. Sprache' })).toHaveValue('en');
    expect(
      within(within(list).getByRole('combobox', { name: '1. Sprache' }))
        .getAllByRole('option')
        .map((option) => option.textContent),
    ).toEqual(['Deutsch', 'Englisch', 'Französisch', 'Japanisch', 'Spanisch']);
    // The default needs no Deutsch any more (concept 22 F-7): each place may go, every choice is
    // open.
    expect(
      within(list).getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }),
    ).toBeEnabled();
    expect(
      within(list).getByRole('button', { name: '2. Sprache (Englisch) entfernen' }),
    ).toBeEnabled();
    expect(disabledOf(within(list).getByRole('combobox', { name: '1. Sprache' }))).toEqual([]);
    expect(disabledOf(within(list).getByRole('combobox', { name: '2. Sprache' }))).toEqual([]);
    // No up and down buttons any more (S-16).
    expect(within(list).queryByRole('button', { name: /nach vorne|nach hinten/ })).toBeNull();
    // The quota line of ADR 0006: two languages, 50 titles a day.
    expect(within(languages).getByText(/2 Sprachen: bis zu 50 Titel am Tag/)).toBeVisible();
    // The order is the fallback (MET-2), and a provider that lacks a language says so: AniList
    // has no Deutsch; TMDB and TVDB have both, the local files have no language list.
    expect(within(languages).getByText(/^Reihenfolge = Rückfall: fehlt ein Text/)).toBeVisible();
    const warning = within(languages).getByText(/kennt nicht:/);
    expect(warning).toHaveTextContent(/^ANILIST kennt nicht: Deutsch 2 Sprachen:/);
    expect(warning).not.toHaveTextContent(/TMDB|TVDB|LOCAL/);
    // The links of MET-1.
    expect(
      screen.getByRole('link', {
        name: 'Die Reihenfolge der Quellen steht je Ordner unter Bibliotheken.',
      }),
    ).toHaveAttribute('href', '/admin/libraries');
    expect(
      screen.getByRole('link', { name: 'Die Zeitpläne stehen unter Hintergrundjobs.' }),
    ).toHaveAttribute('href', '/admin/jobs');

    const images = await openArea(user, 'Bilder');
    expect(within(images).getAllByText('immer')).toHaveLength(2);
    expect(within(images).getByRole('checkbox', { name: /Logo/ })).toBeChecked();
    // Each switchable kind says what it is for (MET-3).
    expect(
      within(images).getByRole('checkbox', { name: 'Staffelposter' }),
    ).toHaveAccessibleDescription('Sonst das Serienposter');
    expect(within(images).getByRole('checkbox', { name: /Folgenbild/ })).toBeChecked();
    // The kinds left out on purpose are named, not offered (MET-3).
    expect(
      within(images).getByText(
        'Art, Banner und Disc gibt es bewusst nicht: keine Seite zeigt sie.',
      ),
    ).toBeVisible();
    expect(within(images).queryByRole('checkbox', { name: /Banner|Disc|Art/ })).toBeNull();
    const poster = within(images).getByRole('group', { name: 'Bildsprache Poster' });
    expect(placesOf(poster)).toEqual(['de', 'en', '']);
    // «Ohne Text» is a choice of the image places.
    expect(
      within(within(poster).getByRole('combobox', { name: '1. Sprache' })).getByRole('option', {
        name: 'Ohne Text',
      }),
    ).toBeInTheDocument();

    const quality = await openArea(user, 'Bildqualität');
    expect(within(quality).getByRole('radio', { name: '500 px' })).toBeChecked();
    expect(within(quality).getByRole('radio', { name: '1280 px' })).toBeChecked();
    expect(within(quality).getByRole('switch', { name: 'Nach Bewertung wählen' })).toBeChecked();

    const sources = await openArea(user, 'Quellen');
    expect(
      within(sources).getByRole('switch', { name: 'Lokale Dateien als Quelle' }),
    ).toBeChecked();
    expect(within(sources).getByText(/211 Titel mit Bilddateien/)).toBeVisible();
    expect(
      within(sources).getByRole('switch', { name: 'Fanart.tv als Bildquelle' }),
    ).not.toBeChecked();
    const keysLink = within(sources).getByRole('link', { name: 'Schlüssel unter Integrationen' });
    expect(keysLink).toBeVisible();
    // The link opens the keys' view of the integrations, not its tiles.
    expect(keysLink).toHaveAttribute('href', '/admin/integrations?item=keys');

    const rating = await openArea(user, 'Altersfreigabe');
    expect(within(rating).getByRole('combobox', { name: 'Land' })).toHaveValue('DE');
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
    // Nothing changed: «Speichern» stays, «Verwerfen» is not there (decision 18, addendum 3).
    expect(within(saveBar()).getByRole('button', { name: 'Speichern' })).toBeEnabled();
    expect(within(saveBar()).queryByRole('button', { name: 'Verwerfen' })).toBeNull();

    const apply = await openArea(user, 'Anwenden');
    expect(within(apply).getByRole('radio', { name: 'Alle Ordner (1234 Titel)' })).toBeChecked();
    // Only the folders of video libraries can be applied to (concept 24 3.3).
    // Each folder names the titles whose home folder it is (concept 24 3.3, homeTitleCount).
    expect(
      within(apply).getByRole('radio', { name: 'Serien: /media/Serien (12 Titel)' }),
    ).toBeVisible();
    expect(within(apply).queryByRole('radio', { name: /Hörbücher/ })).toBeNull();
    expect(
      await within(apply).findByText(/Noch wartend: 3 Zuordnungen, 7 Bilder\. Noch kein Lauf\./),
    ).toBeVisible();
  });

  it('?item= opens the named section only; the others stay closed and fold as the reader leaves them', async () => {
    mockApi(routes());
    renderApp('/admin/metadata?item=rating');
    expect(await sectionOf('Altersfreigabe')).toBeVisible();
    expect(toggleOf('Altersfreigabe')).toHaveAttribute('aria-expanded', 'true');
    expect(toggleOf('Sprachen')).toHaveAttribute('aria-expanded', 'false');
    expect(toggleOf('Ordner')).toHaveAttribute('aria-expanded', 'false');
    const user = userEvent.setup();
    // Folding it away is the reader's choice; another section opens beside it.
    await user.click(toggleOf('Altersfreigabe'));
    expect(toggleOf('Altersfreigabe')).toHaveAttribute('aria-expanded', 'false');
    await openArea(user, 'Bildqualität');
    expect(toggleOf('Bildqualität')).toHaveAttribute('aria-expanded', 'true');
    expect(toggleOf('Altersfreigabe')).toHaveAttribute('aria-expanded', 'false');
  });

  it('the same ?item= link again reopens its section and brings its header into view once', async () => {
    mockApi(routes());
    const scrolled = vi.spyOn(Element.prototype, 'scrollIntoView');
    const { router } = renderApp('/admin/metadata?item=rating');
    expect(await sectionOf('Altersfreigabe')).toBeVisible();
    // The section's block, its header first (the side navigation scrolls its own entry too).
    const block =
      toggleOf('Altersfreigabe').closest('h2')!.parentElement!.parentElement!.parentElement!;
    const scrollsOfBlock = () => scrolled.mock.contexts.filter((node) => node === block).length;
    await waitFor(() => expect(scrollsOfBlock()).toBe(1));
    expect(block).toContainElement(toggleOf('Altersfreigabe'));
    expect(block.firstElementChild).toContainElement(toggleOf('Altersfreigabe'));
    const user = userEvent.setup();
    await user.click(toggleOf('Altersfreigabe'));
    expect(toggleOf('Altersfreigabe')).toHaveAttribute('aria-expanded', 'false');
    expect(scrollsOfBlock()).toBe(1);
    // The same address again: no search value changes, the section opens and scrolls anew.
    await act(() => router.navigate({ to: '/admin/metadata', search: { item: 'rating' } }));
    await waitFor(() =>
      expect(toggleOf('Altersfreigabe')).toHaveAttribute('aria-expanded', 'true'),
    );
    await waitFor(() => expect(scrollsOfBlock()).toBe(2));
    expect(toggleOf('Sprachen')).toHaveAttribute('aria-expanded', 'false');
  });

  it('?item=folders opens the folders: one row per video folder with languages, images and the way to its folders', async () => {
    const own = library({
      id: 4,
      name: 'Anime',
      roots: [
        {
          ...library().roots[0]!,
          id: 4,
          path: '/media/Anime',
          content: 'series',
          textLangs: ['ja', 'en'],
          images: { langs: {}, kinds: [] },
        },
      ],
    });
    mockApi(routes({ 'GET /admin/libraries': () => json([...libraries, own]) }));
    renderApp('/admin/metadata?item=folders');
    const folders = await sectionOf('Ordner');
    expect(headerOf('Ordner')).toHaveTextContent(
      '2 Video-Ordner folgen dem Standard, 1 hat Eigenes',
    );
    const rows = within(folders).getAllByRole('listitem');
    // Audiobook folders have no languages and images of their own (concept 24 3.1).
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent(/\/media\/Filme.*Sprachen: Standard.*Bilder: Standard/);
    expect(rows[2]).toHaveTextContent(
      /\/media\/Anime.*Anime.*Sprachen: Japanisch › Englisch.*Bilder: eigene/,
    );
    expect(
      within(folders).getByText(/^Hörbuch- und Podcast-Ordner haben keine eigenen/),
    ).toBeVisible();
    const user = userEvent.setup();
    // The row action opens «Bearbeiten» of the library with this folder's card open (4.5).
    await user.click(
      within(folders).getByRole('button', { name: 'Anime bearbeiten, Ordner /media/Anime' }),
    );
    const dialog = await screen.findByRole('dialog', { name: 'Bibliothek bearbeiten' });
    const card = within(dialog).getByRole('listitem', { name: 'Ordner 1' });
    expect(within(card).getByRole('button', { name: /\/media\/Anime/ })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    await waitFor(() => expect(within(card).getByLabelText('Ordner')).toHaveFocus());
  });

  it('the band after a save from «Bearbeiten» hands the focus to the page title when dismissed', async () => {
    const { calls } = mockApi(
      routes({
        'PUT /admin/libraries/1': (call: Call) =>
          json({
            library: { ...library(), name: (call.body as { name: string }).name },
            metadataChanged: [],
          }),
      }),
    );
    renderApp('/admin/metadata?item=folders');
    const folders = await sectionOf('Ordner');
    const user = userEvent.setup();
    await user.click(
      within(folders).getByRole('button', { name: 'Filme bearbeiten, Ordner /media/Filme' }),
    );
    const dialog = await screen.findByRole('dialog', { name: 'Bibliothek bearbeiten' });
    await user.type(within(dialog).getByLabelText('Name'), ' neu');
    const settingsLoads = () =>
      calls.filter((call) => call.method === 'GET' && call.path === '/admin/settings/metadata')
        .length;
    const before = settingsLoads();
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    // The dialog gives the focus back to the row's button; the save's own focus step (after the
    // settings are loaded again and the dialog is gone) leaves it there.
    const row = within(folders).getByRole('button', {
      name: 'Filme bearbeiten, Ordner /media/Filme',
    });
    await waitFor(() => expect(row).toHaveFocus());
    await waitFor(() => expect(settingsLoads()).toBeGreaterThan(before));
    await act(() => afterDialogsClosed());
    expect(row).toHaveFocus();
    const dismiss = await screen.findByRole('button', { name: 'Ausblenden' });
    await user.click(dismiss);
    expect(screen.queryByRole('button', { name: 'Ausblenden' })).toBeNull();
    await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveFocus());
  });

  it('adds a language, swaps its place, saves the whole list once and reports the affected titles', async () => {
    let current = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/settings/metadata': () => json(current),
        'PUT /admin/settings/metadata': (call) => {
          const body = call.body as MetadataSettingsUpdate;
          current = settings({ ...(body as Partial<MetadataSettings>), affectedTitles: 1234 });
          return json(current);
        },
      }),
    );
    renderApp('/admin/metadata?item=languages');
    const languages = await sectionOf('Sprachen');
    const user = userEvent.setup();
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    // «Weitere Sprache» appends a place with the first free language (by name: Französisch).
    await user.click(within(list).getByRole('button', { name: 'Weitere Sprache' }));
    expect(placesOf(list)).toEqual(['de', 'en', 'fr']);
    expect(within(list).getByRole('combobox', { name: '3. Sprache' })).toHaveFocus();
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    // The header says «geändert», with the new list as the summary.
    expect(headerOf('Sprachen')).toHaveTextContent('Deutsch › Englisch › Französischgeändert');
    expect(headerOf('Bilder')).not.toHaveTextContent('geändert');
    // A free language replaces the place.
    await user.selectOptions(within(list).getByRole('combobox', { name: '3. Sprache' }), 'ja');
    expect(placesOf(list)).toEqual(['de', 'en', 'ja']);
    // Japanisch on place 2: it swaps with Englisch, which held it.
    await user.selectOptions(
      within(list).getByRole('combobox', { name: '2. Sprache' }),
      'Japanisch',
    );
    expect(placesOf(list)).toEqual(['de', 'ja', 'en']);
    expect(within(list).getByText('Japanisch und Englisch haben die Plätze getauscht.'));
    expect(within(list).getByRole('combobox', { name: '2. Sprache' })).toHaveAccessibleDescription(
      'Wählst du einen Eintrag, der auf einem anderen Platz steht, tauschen die beiden die Plätze.',
    );

    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/settings/metadata')).toHaveLength(1));
    expect(puts(calls, '/admin/settings/metadata')[0]).toMatchObject({
      textLangs: ['de', 'ja', 'en'],
      ratingCountry: 'DE',
      imageKinds: ['logo', 'season_poster', 'episode_still'],
    });
    expect(
      await screen.findByText('Gespeichert für Standard. 1234 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
  });

  it('asks the sources about the list as edited and holds the hint while their answer loads', async () => {
    // TMDB knows Französisch, AniList only Englisch and Französisch; each answer names the
    // languages of the list it was asked about.
    const answers: MetadataProviderInfo[][] = [
      [providers[0]!, providers[2]!],
      [
        { ...providers[0]!, languages: ['de', 'en', 'fr'] },
        { ...providers[2]!, languages: ['en', 'fr'] },
      ],
    ];
    let release = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    let asked = 0;
    const { fetchMock } = mockApi(
      routes({
        'GET /admin/metadata/providers': async () => {
          asked += 1;
          if (asked > 1) await held;
          return json(answers[Math.min(asked, 2) - 1]);
        },
      }),
    );
    renderApp('/admin/metadata?item=languages');
    const languages = await sectionOf('Sprachen');
    expect(
      await within(languages).findByText(/^ANILIST kennt nicht: Deutsch 2 Sprachen/),
    ).toBeVisible();
    const user = userEvent.setup();
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    await user.click(within(list).getByRole('button', { name: 'Weitere Sprache' }));
    expect(placesOf(list)).toEqual(['de', 'en', 'fr']);
    // Asked about the edited list, not the saved default.
    await waitFor(() => expect(asked).toBe(2));
    const lastQuery = new URL((fetchMock.mock.calls.at(-1)?.[0] as Request).url).searchParams;
    expect(lastQuery.getAll('textLangs')).toEqual(['de', 'en', 'fr']);
    // The previous answer is about the old list: no hint from it (it would call Französisch
    // unknown to TMDB), only the quota line.
    expect(within(languages).queryByText(/kennt nicht/)).toBeNull();
    expect(within(languages).getByText(/^3 Sprachen:/)).toBeVisible();
    release();
    const hint = await within(languages).findByText(/kennt nicht/);
    expect(hint).toHaveTextContent(/^ANILIST kennt nicht: Deutsch 3 Sprachen:/);
    expect(hint).not.toHaveTextContent(/TMDB/);
  });

  it('swaps an image language with «Ohne Text» by its place', async () => {
    const { calls } = mockApi(statefulRoutes());
    renderApp('/admin/metadata?item=images');
    const images = await sectionOf('Bilder');
    const user = userEvent.setup();
    const poster = within(images).getByRole('group', { name: 'Bildsprache Poster' });
    expect(placesOf(poster)).toEqual(['de', 'en', '']);
    await user.selectOptions(
      within(poster).getByRole('combobox', { name: '1. Sprache' }),
      'Ohne Text',
    );
    expect(placesOf(poster)).toEqual(['', 'en', 'de']);
    expect(within(poster).getByText('Ohne Text und Deutsch haben die Plätze getauscht.'));
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/settings/metadata')).toHaveLength(1));
    expect(puts(calls, '/admin/settings/metadata')[0]).toMatchObject({
      imageLangs: { poster: ['', 'en', 'de'], backdrop: ['', 'de', 'en'], logo: ['de', 'en'] },
    });
  });

  it("shows the server's field error at the languages, names the scope and keeps the form", async () => {
    mockApi(
      routes({
        'PUT /admin/settings/metadata': () =>
          problem(422, 'validation_failed', {
            errors: [{ field: 'textLangs', code: 'lang_unknown' }],
          }),
      }),
    );
    renderApp('/admin/metadata?item=languages');
    const languages = await sectionOf('Sprachen');
    const user = userEvent.setup();
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    // A list without Deutsch is fine on the page (concept 22 F-7); here the server refuses it.
    await user.click(within(list).getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }));
    expect(placesOf(list)).toEqual(['en']);
    // The last place cannot go.
    expect(
      within(list).getByRole('button', { name: '1. Sprache (Englisch) entfernen' }),
    ).toBeDisabled();
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(await within(languages).findByText('Unbekannter Sprachcode.')).toBeVisible();
    expect(screen.getByText(/^«Standard» wurde nicht gespeichert: /)).toBeVisible();
    expect(within(list).getAllByRole('listitem')).toHaveLength(1);
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('opens a section folded during the save on its field error and marks the field', async () => {
    let release = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    mockApi(
      routes({
        'PUT /admin/settings/metadata': async () => {
          await held;
          return problem(422, 'validation_failed', {
            errors: [
              { field: 'imageMinWidth.poster', code: 'invalid' },
              { field: 'fanartEnabled', code: 'invalid' },
            ],
          });
        },
      }),
    );
    renderApp('/admin/metadata?item=quality');
    const quality = await sectionOf('Bildqualität');
    const user = userEvent.setup();
    await user.click(within(quality).getByRole('radio', { name: '1000 px' }));
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    // Folded while the server still decides; «Quellen» was never open.
    await user.click(toggleOf('Bildqualität'));
    expect(toggleOf('Bildqualität')).toHaveAttribute('aria-expanded', 'false');
    expect(toggleOf('Quellen')).toHaveAttribute('aria-expanded', 'false');
    release();
    await waitFor(() => expect(toggleOf('Bildqualität')).toHaveAttribute('aria-expanded', 'true'));
    expect(toggleOf('Quellen')).toHaveAttribute('aria-expanded', 'true');
    expect(toggleOf('Sprachen')).toHaveAttribute('aria-expanded', 'false');
    // Each section marks its field, not only the notice under the save bar.
    expect(within(await sectionOf('Bildqualität')).getByText('Ungültiger Wert.')).toBeVisible();
    expect(within(await sectionOf('Quellen')).getByText('Ungültiger Wert.')).toBeVisible();
  });

  it('drops ?library= of the earlier scope from the address and shows the default', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/metadata?library=2');
    expect(await screen.findByRole('heading', { level: 2, name: 'Sprachen' })).toBeVisible();
    await waitFor(() => expect(router.state.location.search).not.toHaveProperty('library'));
  });

  it('keeps the drafts while a section folds away, marks it «geändert» and asks before leaving (S-17)', async () => {
    mockApi(statefulRoutes());
    const { router } = renderApp('/admin/metadata');
    const user = userEvent.setup();
    const languages = await openArea(user, 'Sprachen');
    await user.click(
      within(within(languages).getByRole('group', { name: 'Metadaten-Sprachen' })).getByRole(
        'button',
        { name: '2. Sprache (Englisch) entfernen' },
      ),
    );
    // Folded away, the section keeps the draft and says «geändert» in its header.
    await user.click(toggleOf('Sprachen'));
    expect(toggleOf('Sprachen')).toHaveAttribute('aria-expanded', 'false');
    expect(within(headerOf('Sprachen')).getByText('geändert')).toBeVisible();
    await openArea(user, 'Altersfreigabe');
    await user.selectOptions(screen.getByRole('combobox', { name: 'Land' }), 'CH');
    // Opening another section is not leaving the page: no question.
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(headerOf('Altersfreigabe')).toHaveTextContent('Schweizgeändert');
    expect(headerOf('Quellen')).not.toHaveTextContent('geändert');
    const back = await openArea(user, 'Sprachen');
    expect(
      within(within(back).getByRole('group', { name: 'Metadaten-Sprachen' })).getAllByRole(
        'listitem',
      ),
    ).toHaveLength(1);

    // Leaving the page asks first; staying keeps everything.
    void router.navigate({ to: '/admin/jobs' });
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(router.state.location.pathname).toBe('/admin/metadata');
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('keeps what is typed while a save waits for the server', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    let server = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/settings/metadata': () => json(server),
        'PUT /admin/settings/metadata': async (call) => {
          await answered;
          server = settings({ ...(call.body as Partial<MetadataSettings>) });
          return json(server);
        },
      }),
    );
    renderApp('/admin/metadata?item=rating');
    const user = userEvent.setup();
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Land' }), 'CH');
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await user.click(
      within(await screen.findByRole('alertdialog', { name: 'Freigabeland wechseln?' })).getByRole(
        'button',
        { name: 'Wechseln und neu lesen' },
      ),
    );
    await waitFor(() => expect(puts(calls, '/admin/settings/metadata')).toHaveLength(1));
    // Typed while the server has not answered yet.
    await user.selectOptions(screen.getByRole('combobox', { name: 'Land' }), 'FR');
    release();
    expect(
      await screen.findByText('Gespeichert für Standard. 1234 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    expect(puts(calls, '/admin/settings/metadata')[0]).toMatchObject({ ratingCountry: 'CH' });
    // The newer value stays, still unsaved.
    expect(screen.getByRole('combobox', { name: 'Land' })).toHaveValue('FR');
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('keeps a field set back to its old value while the save waits (L-5)', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    let server = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/settings/metadata': () => json(server),
        'PUT /admin/settings/metadata': async (call) => {
          await answered;
          server = settings({ ...(call.body as Partial<MetadataSettings>) });
          return json(server);
        },
      }),
    );
    renderApp('/admin/metadata?item=rating');
    const user = userEvent.setup();
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Land' }), 'CH');
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await user.click(
      within(await screen.findByRole('alertdialog', { name: 'Freigabeland wechseln?' })).getByRole(
        'button',
        { name: 'Wechseln und neu lesen' },
      ),
    );
    await waitFor(() => expect(puts(calls, '/admin/settings/metadata')).toHaveLength(1));
    // Back to the value the server held before: no draft any more until the save lands.
    await user.selectOptions(screen.getByRole('combobox', { name: 'Land' }), 'DE');
    release();
    expect(
      await screen.findByText('Gespeichert für Standard. 1234 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    // The server holds CH now; DE is what was chosen last and stays, unsaved.
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Land' })).toHaveValue('DE'));
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('"Verwerfen" names the default and resets it after the question', async () => {
    const { calls } = mockApi(statefulRoutes());
    renderApp('/admin/metadata?item=rating');
    const user = userEvent.setup();
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Land' }), 'CH');

    await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    expect(dialog).toHaveTextContent('Die ungespeicherten Änderungen an Standard gehen verloren.');
    // Cancel keeps them.
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();

    await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Verwerfen' }),
    );
    await waitFor(() => expect(within(saveBar()).getByText('Gespeichert')).toBeVisible());
    expect(screen.getByRole('combobox', { name: 'Land' })).toHaveValue('DE');
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('"Jetzt anwenden" posts what and where and names the queued run', async () => {
    const { calls } = mockApi(
      routes({
        'POST /admin/metadata/apply': () => json({ jobId: 99, titles: 40 }, 202),
      }),
    );
    renderApp('/admin/metadata?item=apply');
    const apply = await sectionOf('Anwenden');
    const user = userEvent.setup();
    await user.click(within(apply).getByRole('radio', { name: 'Nur Texte' }));
    await user.click(
      within(apply).getByRole('radio', { name: 'Serien: /media/Serien (12 Titel)' }),
    );
    // The header names what and where.
    expect(headerOf('Anwenden')).toHaveTextContent('Nur Texte · Serien: /media/Serien (12 Titel)');
    await user.click(within(apply).getByRole('button', { name: 'Jetzt anwenden' }));
    expect(
      await within(apply).findByText(
        'Lauf 99 eingereiht: 40 Titel. Fortschritt unter Hintergrundjobs.',
      ),
    ).toBeVisible();
    const posts = calls.filter(
      (call) => call.method === 'POST' && call.path === '/admin/metadata/apply',
    );
    expect(posts).toHaveLength(1);
    expect(posts[0]?.body).toEqual({ what: ['texts'], rootIds: [2] });
  });

  it('a non-admin profile gets the notice instead of the settings', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({
          account: {
            id: 2,
            email: 'guest@example.test',
            role: 'member',
            isOwner: false,
            maxRating: 18,
            allLibraries: true,
            libraryIds: [],
          },
          profile: {
            id: 5,
            name: 'Gast',
            maxRating: 18,
            language: 'de',
            allLibraries: true,
            libraryIds: [],
            hasPin: false,
            hasOfflinePin: false,
            isPrimary: true,
            limits: {},
            effectiveLimits: {},
          },
        }),
    });
    renderApp('/admin/metadata');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/settings/metadata')).toBe(false);
  });

  it('names the rating countries by Intl, shows the steps of the choice and asks before a change (concept 22 F-16)', async () => {
    const { calls } = mockApi(statefulRoutes());
    renderApp('/admin/metadata?item=rating');
    const user = userEvent.setup();
    const country = await screen.findByRole('combobox', { name: 'Land' });
    expect(within(country).getByRole('option', { name: 'Frankreich (CNC)' })).toBeInTheDocument();
    expect(within(country).getByRole('option', { name: 'Schweiz' })).toBeInTheDocument();
    expect(
      screen.getByText('Stufen der Profile: FSK 0 · FSK 6 · FSK 12 · FSK 16 · FSK 18'),
    ).toBeVisible();
    await user.selectOptions(country, 'FR');
    expect(
      screen.getByText('Stufen der Profile: Tous publics · -10 · -12 · -16 · -18'),
    ).toBeVisible();
    // 1234 titles: saving asks first, cancelling sends nothing.
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Freigabeland wechseln?' });
    expect(dialog).toHaveTextContent('Slouchcat liest danach die Freigaben aller Titel neu.');
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });
});
