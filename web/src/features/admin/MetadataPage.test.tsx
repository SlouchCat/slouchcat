import type {
  LibraryMetadata,
  LibraryMetadataUpdate,
  MetadataProviderInfo,
  MetadataSettings,
  MetadataSettingsUpdate,
} from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { library } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn, type Call, type Routes } from '@/test/api';

// The page "Metadaten" (concept 10, 7.1; layout concept 12 Met-B): the scope in the header, the
// settings areas in a list, the selected area beside it; the server default and a library's
// deviation, the language places, the image kinds and chains, quality, sources, rating country
// and the apply run; one save bar for every changed scope.

const settings = (overrides: Partial<MetadataSettings> = {}): MetadataSettings => ({
  textLangs: ['de', 'en'],
  imageLangs: { poster: ['de', 'en', ''], backdrop: ['', 'de', 'en'], logo: ['de', 'en'] },
  imageKinds: ['logo', 'season_poster', 'episode_still'],
  imageMinWidth: { poster: 500, backdrop: 1280 },
  imageByVote: true,
  localImages: true,
  ratingCountry: 'DE',
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
  library({ id: 2, name: 'Serien' }),
  library({ id: 3, name: 'Hörbücher', kind: 'audiobooks' }),
];

const libraryMeta = (overrides: Partial<LibraryMetadata> = {}): LibraryMetadata => ({
  libraryId: 2,
  textLangs: null,
  images: null,
  effective: settings(),
  affectedTitles: 40,
  ...overrides,
});

/** The library id in `/admin/libraries/{id}/metadata`. */
const idOf = (call: Call) => Number(call.path.split('/')[3]);

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
  'GET /admin/libraries/*/metadata': (call: Call) =>
    json(libraryMeta({ libraryId: idOf(call), affectedTitles: idOf(call) * 20 })),
  ...extra,
});

/**
 * A server that keeps what it is sent: the default and each library's deviation. `failLibrary`
 * answers the PUT of that library with an error.
 */
function statefulRoutes({ failLibrary }: { failLibrary?: number } = {}) {
  let server = settings();
  const stored = new Map<number, LibraryMetadata>();
  const metaOf = (id: number) =>
    stored.get(id) ?? libraryMeta({ libraryId: id, effective: server, affectedTitles: id * 20 });
  return routes({
    'GET /admin/settings/metadata': () => json(server),
    'PUT /admin/settings/metadata': (call) => {
      server = settings({ ...(call.body as Partial<MetadataSettings>) });
      return json(server);
    },
    'GET /admin/libraries/*/metadata': (call) => json(metaOf(idOf(call))),
    'PUT /admin/libraries/*/metadata': (call) => {
      const id = idOf(call);
      if (id === failLibrary) return problem(500, 'internal');
      const body = call.body as LibraryMetadataUpdate;
      stored.set(
        id,
        libraryMeta({
          libraryId: id,
          textLangs: body.textLangs ?? null,
          images: body.images ?? null,
          effective: server,
          affectedTitles: id * 20,
        }),
      );
      return json(metaOf(id));
    },
  });
}

const region = (name: string) => screen.getByRole('region', { name });
const saveBar = () => region('Speichern');
const areas = () => screen.getByRole('listbox', { name: 'Bereiche' });
const scopeSelect = () => screen.getByLabelText('Geltung');

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

/** Selects a settings area in the list on the left. */
async function openArea(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(within(areas()).getByRole('option', { name }));
  return screen.findByRole('region', { name: areaTitle[name] ?? name });
}
const areaTitle: Record<string, string> = { Anwenden: 'Auf bestehende Titel anwenden' };

describe('metadata settings page (concept 10, concept 12 Met-B)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the server default: areas, languages in order, image kinds, chains, quality, sources, country, apply', async () => {
    mockApi(routes());
    renderApp('/admin/metadata');
    expect(await screen.findByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();
    // The intro sits behind the help button (S-2).
    expect(screen.getByRole('button', { name: 'Hilfe zu dieser Seite' })).toBeVisible();

    const languages = await screen.findByRole('region', { name: 'Sprachen' });
    expect(
      within(areas())
        .getAllByRole('option')
        .map((option) => option.textContent),
    ).toEqual([
      expect.stringContaining('Sprachen'),
      expect.stringContaining('Bilder'),
      expect.stringContaining('Güte'),
      expect.stringContaining('Quellen'),
      expect.stringContaining('Altersfreigabe'),
      expect.stringContaining('Anwenden'),
    ]);
    expect(within(areas()).getByRole('option', { name: 'Sprachen' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(scopeSelect()).toHaveValue('server');
    expect(within(languages).getByText(/Gilt für alle Bibliotheken ohne eigene Abweichung/));
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
    // The default needs Deutsch (MET-2, as the server checks it): it cannot go, Englisch can.
    expect(
      within(list).getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }),
    ).toBeDisabled();
    expect(
      within(list).getByRole('button', { name: '2. Sprache (Englisch) entfernen' }),
    ).toBeEnabled();
    // Place 1 may only swap Deutsch to place 2; every other language would drop it.
    expect(disabledOf(within(list).getByRole('combobox', { name: '1. Sprache' }))).toEqual([
      'Französisch',
      'Japanisch',
      'Spanisch',
    ]);
    expect(disabledOf(within(list).getByRole('combobox', { name: '2. Sprache' }))).toEqual([]);
    // No up and down buttons any more (S-16).
    expect(within(list).queryByRole('button', { name: /nach vorne|nach hinten/ })).toBeNull();
    // The quota line of ADR 0006: two languages, 50 titles a day.
    expect(within(languages).getByText(/2 Sprachen: bis zu 50 Titel am Tag/)).toBeVisible();
    // The links of MET-1.
    expect(
      screen.getByRole('link', { name: 'Die Zeitpläne stehen unter Hintergrundjobs.' }),
    ).toHaveAttribute('href', '/admin/jobs');

    const user = userEvent.setup();
    const images = await openArea(user, 'Bilder');
    expect(within(images).getAllByText('immer')).toHaveLength(2);
    expect(within(images).getByRole('checkbox', { name: /Logo/ })).toBeChecked();
    // Each switchable kind says what it is for (MET-3).
    expect(
      within(images).getByRole('checkbox', { name: 'Staffelposter' }),
    ).toHaveAccessibleDescription('Sonst das Serienposter');
    expect(within(images).getByRole('checkbox', { name: /Folgenbild/ })).toBeChecked();
    const poster = within(images).getByRole('group', { name: 'Bildsprache Poster' });
    expect(placesOf(poster)).toEqual(['de', 'en', '']);
    // «Ohne Text» is a choice of the image places.
    expect(
      within(within(poster).getByRole('combobox', { name: '1. Sprache' })).getByRole('option', {
        name: 'Ohne Text',
      }),
    ).toBeInTheDocument();

    const quality = await openArea(user, 'Güte');
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
    expect(
      within(sources).getByRole('link', { name: 'Schlüssel unter Integrationen' }),
    ).toBeVisible();

    const rating = await openArea(user, 'Altersfreigabe');
    expect(within(rating).getByRole('radio', { name: 'Deutschland (DE)' })).toBeChecked();
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
    expect(within(saveBar()).getByRole('button', { name: 'Speichern' })).toBeDisabled();

    const apply = await openArea(user, 'Anwenden');
    expect(
      within(apply).getByRole('radio', { name: 'Alle Bibliotheken (1234 Titel)' }),
    ).toBeChecked();
    // Only video libraries can be applied to, each with its titles.
    expect(within(apply).getByRole('radio', { name: 'Serien (40 Titel)' })).toBeVisible();
    expect(within(apply).queryByRole('radio', { name: /Hörbücher/ })).toBeNull();
    expect(
      await within(apply).findByText(/Noch wartend: 3 Zuordnungen, 7 Bilder\. Noch kein Lauf\./),
    ).toBeVisible();
  });

  it('the selected area lives in ?item= and survives a reload', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/metadata?item=rating');
    expect(await screen.findByRole('region', { name: 'Altersfreigabe' })).toBeVisible();
    const user = userEvent.setup();
    await openArea(user, 'Güte');
    expect(router.state.location.search).toMatchObject({ item: 'quality' });
    // The default area drops the parameter.
    await openArea(user, 'Sprachen');
    expect(router.state.location.search).not.toHaveProperty('item');
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
    renderApp('/admin/metadata');
    const languages = await screen.findByRole('region', { name: 'Sprachen' });
    const user = userEvent.setup();
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    // «Weitere Sprache» appends a place with the first free language (by name: Französisch).
    await user.click(within(list).getByRole('button', { name: 'Weitere Sprache' }));
    expect(placesOf(list)).toEqual(['de', 'en', 'fr']);
    expect(within(list).getByRole('combobox', { name: '3. Sprache' })).toHaveFocus();
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    expect(within(areas()).getByRole('option', { name: 'Sprachen' })).toHaveAccessibleDescription(
      /^Geändert · Deutsch › Englisch › Französisch$/,
    );
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

  it('swaps an image language with «Ohne Text» by its place', async () => {
    const { calls } = mockApi(statefulRoutes());
    renderApp('/admin/metadata?item=images');
    const images = await screen.findByRole('region', { name: 'Bilder' });
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
            errors: [{ field: 'textLangs', code: 'ui_lang_required' }],
          }),
      }),
    );
    renderApp('/admin/metadata');
    const languages = await screen.findByRole('region', { name: 'Sprachen' });
    const user = userEvent.setup();
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    // The page keeps Deutsch itself; here the server refuses the list all the same.
    await user.click(within(list).getByRole('button', { name: '2. Sprache (Englisch) entfernen' }));
    expect(placesOf(list)).toEqual(['de']);
    expect(
      within(list).getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }),
    ).toBeDisabled();
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(languages).findByText(
        'Der Standard braucht Deutsch; eine Bibliothek Deutsch oder Englisch.',
      ),
    ).toBeVisible();
    expect(screen.getByText(/^«Standard» wurde nicht gespeichert: /)).toBeVisible();
    expect(within(list).getAllByRole('listitem')).toHaveLength(1);
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('edits a library: the two deviations, dimmed inherited values, a PUT with null for "like the server"', async () => {
    const { calls } = mockApi(statefulRoutes());
    renderApp('/admin/metadata?library=2');
    expect(await screen.findByText(/Serien übernimmt den Standard/)).toBeVisible();
    const user = userEvent.setup();
    expect(scopeSelect()).toHaveValue('2');
    // Quality, sources, rating and apply belong to the server: not on a library.
    expect(within(areas()).getAllByRole('option')).toHaveLength(2);
    expect(within(areas()).queryByRole('option', { name: 'Güte' })).toBeNull();
    expect(within(areas()).queryByRole('option', { name: 'Altersfreigabe' })).toBeNull();
    const languages = region('Sprachen');
    expect(within(languages).getByText('wie Server')).toBeVisible();
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    // Read-only: the places are disabled, no «×», no «Weitere Sprache».
    expect(within(list).getByRole('combobox', { name: '2. Sprache' })).toBeDisabled();
    expect(
      within(list).queryByRole('button', { name: '2. Sprache (Englisch) entfernen' }),
    ).toBeNull();
    expect(within(list).queryByRole('button', { name: 'Weitere Sprache' })).toBeNull();

    await user.click(screen.getByRole('switch', { name: 'Sprachen abweichend' }));
    expect(within(languages).queryByText('wie Server')).toBeNull();
    // A library needs Deutsch or Englisch: with both there, each may go, every choice is open.
    expect(
      within(list).getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }),
    ).toBeEnabled();
    expect(disabledOf(within(list).getByRole('combobox', { name: '1. Sprache' }))).toEqual([]);
    await user.click(within(list).getByRole('button', { name: '2. Sprache (Englisch) entfernen' }));
    expect(placesOf(list)).toEqual(['de']);
    // The last entry cannot go; place 1 may hold Deutsch or Englisch only.
    expect(
      within(list).getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }),
    ).toBeDisabled();
    expect(disabledOf(within(list).getByRole('combobox', { name: '1. Sprache' }))).toEqual([
      'Französisch',
      'Japanisch',
      'Spanisch',
    ]);
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/libraries/2/metadata')).toHaveLength(1));
    expect(puts(calls, '/admin/libraries/2/metadata')[0]).toEqual({
      textLangs: ['de'],
      images: null,
    });
    expect(
      await screen.findByText('Gespeichert für Serien. 40 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    expect(puts(calls, '/admin/settings/metadata')).toHaveLength(0);

    // Switching the deviation off and saving sends null: the library follows the server again.
    await user.click(screen.getByRole('switch', { name: 'Sprachen abweichend' }));
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/libraries/2/metadata')).toHaveLength(2));
    expect(puts(calls, '/admin/libraries/2/metadata')[1]).toEqual({
      textLangs: null,
      images: null,
    });
    // Switching it on again brings back the last deviating list, not the default (S-13).
    await user.click(screen.getByRole('switch', { name: 'Sprachen abweichend' }));
    expect(placesOf(list)).toEqual(['de']);
  });

  it('shows on a library that follows the default when the default has unsaved changes', async () => {
    mockApi(statefulRoutes());
    renderApp('/admin/metadata');
    const user = userEvent.setup();
    const languages = await screen.findByRole('region', { name: 'Sprachen' });
    await user.click(
      within(within(languages).getByRole('group', { name: 'Metadaten-Sprachen' })).getByRole(
        'button',
        { name: '2. Sprache (Englisch) entfernen' },
      ),
    );
    await user.selectOptions(scopeSelect(), 'Serien');
    const library = await screen.findByRole('region', { name: 'Sprachen' });
    await within(library).findByText(/Serien übernimmt den Standard/);
    expect(
      within(library).getByText(
        'Standard hat ungespeicherte Änderungen; hier steht der gespeicherte Stand.',
      ),
    ).toBeVisible();
    // Its images follow an unchanged default: no note there.
    await openArea(user, 'Bilder');
    expect(screen.queryByText(/Standard hat ungespeicherte Änderungen/)).toBeNull();
  });

  it('drops an unknown ?library= from the address and shows the default', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/metadata?library=3');
    expect(
      await screen.findByText(/Gilt für alle Bibliotheken ohne eigene Abweichung/),
    ).toBeVisible();
    await waitFor(() => expect(router.state.location.search).not.toHaveProperty('library'));
    expect(scopeSelect()).toHaveValue('server');
  });

  it('switches the scope from the server to a library and back without losing the server form', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/metadata');
    await screen.findByRole('region', { name: 'Sprachen' });
    const user = userEvent.setup();
    await user.selectOptions(scopeSelect(), 'Serien');
    expect(await screen.findByText(/Serien übernimmt den Standard/)).toBeVisible();
    expect(router.state.location.search).toMatchObject({ library: 2 });
    // Audiobook libraries have no metadata settings.
    expect(within(scopeSelect()).queryByRole('option', { name: 'Hörbücher' })).toBeNull();
    await user.selectOptions(scopeSelect(), 'Standard');
    expect(
      await screen.findByText(/Gilt für alle Bibliotheken ohne eigene Abweichung/),
    ).toBeVisible();
    expect(within(areas()).getByRole('option', { name: 'Güte' })).toBeVisible();
  });

  it('keeps the drafts of every scope while the scope and the area change (S-13, S-17)', async () => {
    mockApi(statefulRoutes());
    const { router } = renderApp('/admin/metadata');
    const user = userEvent.setup();
    const languages = await screen.findByRole('region', { name: 'Sprachen' });
    await user.click(
      within(within(languages).getByRole('group', { name: 'Metadaten-Sprachen' })).getByRole(
        'button',
        { name: '2. Sprache (Englisch) entfernen' },
      ),
    );
    await openArea(user, 'Altersfreigabe');
    await user.click(screen.getByRole('radio', { name: 'Schweiz (CH)' }));

    // To the library Serien: its own change.
    await user.selectOptions(scopeSelect(), 'Serien');
    await screen.findByText(/Serien übernimmt den Standard/);
    // The library has no "Altersfreigabe": the list falls back to its first area.
    expect(region('Sprachen')).toBeVisible();
    await user.click(screen.getByRole('switch', { name: 'Sprachen abweichend' }));
    expect(screen.getByRole('option', { name: 'Standard (geändert)' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Serien (geändert)' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Filme' })).toBeInTheDocument();
    // Changing the scope or the area is not leaving the page: no question.
    expect(screen.queryByRole('alertdialog')).toBeNull();

    // Back to the default: its area is selected again, and both its changes are still there.
    await user.selectOptions(scopeSelect(), 'Standard (geändert)');
    const rating = await screen.findByRole('region', { name: 'Altersfreigabe' });
    expect(within(rating).getByRole('radio', { name: 'Schweiz (CH)' })).toBeChecked();
    expect(
      within(areas()).getByRole('option', { name: 'Altersfreigabe' }),
    ).toHaveAccessibleDescription('Geändert · Schweiz (CH)');
    const back = await openArea(user, 'Sprachen');
    expect(
      within(within(back).getByRole('group', { name: 'Metadaten-Sprachen' })).getAllByRole(
        'listitem',
      ),
    ).toHaveLength(1);
    // And the library's switch too.
    await user.selectOptions(scopeSelect(), 'Serien (geändert)');
    expect(await screen.findByRole('switch', { name: 'Sprachen abweichend' })).toBeChecked();
    expect(router.state.location.search).toMatchObject({ library: 2 });

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

  it('saves the changed scopes one after the other and stops at a failure, naming the scope', async () => {
    const { calls } = mockApi(statefulRoutes({ failLibrary: 1 }));
    renderApp('/admin/metadata?item=rating');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('radio', { name: 'Schweiz (CH)' }));
    // Serien first, then Filme: the save goes by the list order, not by the order of the edits.
    await user.selectOptions(scopeSelect(), 'Serien');
    await user.click(await screen.findByRole('switch', { name: 'Sprachen abweichend' }));
    await user.selectOptions(scopeSelect(), 'Filme');
    await screen.findByText(/Filme übernimmt den Standard/);
    await user.click(await screen.findByRole('switch', { name: 'Sprachen abweichend' }));

    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText(/^«Filme» wurde nicht gespeichert: /)).toBeVisible();
    const order = calls.filter((call) => call.method === 'PUT').map((call) => call.path);
    expect(order).toEqual(['/admin/settings/metadata', '/admin/libraries/1/metadata']);
    // The default is saved; Filme failed and Serien after it was never sent: both stay changed.
    expect(
      screen.getByText('Gespeichert für Standard. 1234 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    expect(screen.getByRole('option', { name: 'Standard' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Filme (geändert)' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Serien (geändert)' })).toBeInTheDocument();
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
    await user.click(await screen.findByRole('radio', { name: 'Schweiz (CH)' }));
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/settings/metadata')).toHaveLength(1));
    // Typed while the server has not answered yet.
    await user.click(screen.getByRole('radio', { name: 'Österreich (AT)' }));
    release();
    expect(
      await screen.findByText('Gespeichert für Standard. 1234 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    expect(puts(calls, '/admin/settings/metadata')[0]).toMatchObject({ ratingCountry: 'CH' });
    // The newer value stays, still unsaved.
    expect(screen.getByRole('radio', { name: 'Österreich (AT)' })).toBeChecked();
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    expect(screen.getByRole('option', { name: 'Standard (geändert)' })).toBeInTheDocument();
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
    await user.click(await screen.findByRole('radio', { name: 'Schweiz (CH)' }));
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/settings/metadata')).toHaveLength(1));
    // Back to the value the server held before: no draft any more until the save lands.
    await user.click(screen.getByRole('radio', { name: 'Deutschland (DE)' }));
    release();
    expect(
      await screen.findByText('Gespeichert für Standard. 1234 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    // The server holds CH now; DE is what was chosen last and stays, unsaved.
    await waitFor(() =>
      expect(screen.getByRole('radio', { name: 'Deutschland (DE)' })).toBeChecked(),
    );
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('"Verwerfen" names every changed scope and resets them all after the question', async () => {
    const { calls } = mockApi(statefulRoutes());
    renderApp('/admin/metadata?item=rating');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('radio', { name: 'Schweiz (CH)' }));
    await user.selectOptions(scopeSelect(), 'Serien');
    await user.click(await screen.findByRole('switch', { name: 'Sprachen abweichend' }));

    await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    expect(dialog).toHaveTextContent(
      'Die ungespeicherten Änderungen an Standard und Serien gehen verloren.',
    );
    // Cancel keeps them.
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();

    await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Verwerfen' }),
    );
    await waitFor(() => expect(within(saveBar()).getByText('Gespeichert')).toBeVisible());
    expect(screen.getByRole('switch', { name: 'Sprachen abweichend' })).not.toBeChecked();
    expect(screen.getByRole('option', { name: 'Standard' })).toBeInTheDocument();
    await user.selectOptions(scopeSelect(), 'Standard');
    const rating = await openArea(user, 'Altersfreigabe');
    expect(within(rating).getByRole('radio', { name: 'Deutschland (DE)' })).toBeChecked();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('"Jetzt anwenden" posts what and where and names the queued run', async () => {
    const { calls } = mockApi(
      routes({
        'POST /admin/metadata/apply': () => json({ jobId: 99, titles: 40 }, 202),
      }),
    );
    renderApp('/admin/metadata?item=apply');
    const apply = await screen.findByRole('region', { name: 'Auf bestehende Titel anwenden' });
    const user = userEvent.setup();
    await user.click(within(apply).getByRole('radio', { name: 'Nur Texte' }));
    await user.click(within(apply).getByRole('radio', { name: 'Serien (40 Titel)' }));
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
    expect(posts[0]?.body).toEqual({ what: ['texts'], libraryIds: [2] });
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
});
