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
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';

// The page "Metadaten" (concept 10, 7.1): the server default and a library's deviation, the
// language chips, the image kinds and chains, quality, sources, rating country and the apply run.

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
  'GET /admin/libraries/*/metadata': () => json(libraryMeta()),
  ...extra,
});

const region = (name: string) => screen.getByRole('region', { name });
const saveBar = () => region('Speichern');

const puts = (calls: { method: string; path: string; body: unknown }[], path: string) =>
  calls.filter((call) => call.method === 'PUT' && call.path === path).map((call) => call.body);

describe('metadata settings page (concept 10)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the server default: languages in order, image kinds, chains, quality, sources, country', async () => {
    mockApi(routes());
    renderApp('/admin/metadata');
    expect(await screen.findByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();

    const languages = await screen.findByRole('region', { name: 'Sprachen' });
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    const items = within(list).getAllByRole('listitem');
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent('1.');
    expect(items[0]).toHaveTextContent('Deutsch');
    expect(items[1]).toHaveTextContent('Englisch');
    // Both interface languages are there, so either may go; the first cannot move forward.
    expect(within(list).getByRole('button', { name: 'Deutsch nach vorne' })).toBeDisabled();
    expect(within(list).getByRole('button', { name: 'Englisch nach hinten' })).toBeDisabled();
    expect(within(list).getByRole('button', { name: 'Deutsch entfernen' })).toBeEnabled();
    // The quota line of ADR 0006: two languages, 50 titles a day.
    expect(within(languages).getByText(/2 Sprachen: bis zu 50 Titel am Tag/)).toBeVisible();

    const images = region('Bilder');
    expect(within(images).getAllByText('immer')).toHaveLength(2);
    expect(within(images).getByRole('checkbox', { name: /Logo/ })).toBeChecked();
    expect(within(images).getByRole('checkbox', { name: /Folgenbild/ })).toBeChecked();
    const poster = within(images).getByRole('group', { name: 'Bildsprache Poster' });
    expect(
      within(poster)
        .getAllByRole('listitem')
        .map((li) => li.textContent),
    ).toEqual([
      expect.stringContaining('Deutsch'),
      expect.stringContaining('Englisch'),
      expect.stringContaining('Ohne Text'),
    ]);

    const quality = region('Güte');
    expect(within(quality).getByRole('radio', { name: '500 px' })).toBeChecked();
    expect(within(quality).getByRole('radio', { name: '1280 px' })).toBeChecked();
    expect(within(quality).getByRole('switch', { name: 'Nach Bewertung wählen' })).toBeChecked();

    const sources = region('Quellen');
    expect(
      within(sources).getByRole('switch', { name: 'Lokale Dateien als Quelle' }),
    ).toBeChecked();
    expect(within(sources).getByText(/211 Titel mit Bilddateien/)).toBeVisible();
    expect(
      within(sources).getByRole('switch', { name: 'Fanart.tv als Bildquelle' }),
    ).not.toBeChecked();

    expect(
      within(region('Altersfreigabe')).getByRole('radio', { name: 'Deutschland (DE)' }),
    ).toBeChecked();
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
    expect(within(saveBar()).getByRole('button', { name: 'Speichern' })).toBeDisabled();
    const apply = region('Auf bestehende Titel anwenden');
    expect(
      within(apply).getByRole('radio', { name: 'Alle Bibliotheken (1234 Titel)' }),
    ).toBeChecked();
    // Only video libraries can be applied to.
    expect(within(apply).getByRole('radio', { name: 'Serien' })).toBeVisible();
    expect(within(apply).queryByRole('radio', { name: 'Hörbücher' })).toBeNull();
    expect(
      await within(apply).findByText(/Noch wartend: 3 Zuordnungen, 7 Bilder\. Noch kein Lauf\./),
    ).toBeVisible();
  });

  it('adds a language, moves it, saves the whole list once and reports the affected titles', async () => {
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
    await user.selectOptions(within(languages).getByLabelText('Sprache wählen'), 'ja');
    await user.click(within(languages).getByRole('button', { name: 'Hinzufügen' }));
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    expect(within(list).getAllByRole('listitem')).toHaveLength(3);
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    await user.click(within(list).getByRole('button', { name: 'Japanisch nach vorne' }));
    expect(within(list).getAllByRole('listitem')[1]).toHaveTextContent('Japanisch');
    // The picker no longer offers the language that is in the list.
    expect(
      within(within(languages).getByLabelText('Sprache wählen')).queryByRole('option', {
        name: 'Japanisch',
      }),
    ).toBeNull();

    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/settings/metadata')).toHaveLength(1));
    expect(puts(calls, '/admin/settings/metadata')[0]).toMatchObject({
      textLangs: ['de', 'ja', 'en'],
      ratingCountry: 'DE',
      imageKinds: ['logo', 'season_poster', 'episode_still'],
    });
    expect(
      await screen.findByText('Gespeichert. 1234 Titel betroffen – jetzt anwenden?'),
    ).toBeVisible();
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
  });

  it("shows the server's field error at the languages and keeps the form", async () => {
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
    await user.click(within(list).getByRole('button', { name: 'Deutsch entfernen' }));
    // Englisch is the last interface language now and cannot go either.
    expect(within(list).getByRole('button', { name: 'Englisch entfernen' })).toBeDisabled();
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(languages).findByText(
        'Der Standard braucht Deutsch; eine Bibliothek Deutsch oder Englisch.',
      ),
    ).toBeVisible();
    expect(within(list).getAllByRole('listitem')).toHaveLength(1);
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('edits a library: the two deviations, dimmed inherited values, a PUT with null for "like the server"', async () => {
    let current = libraryMeta();
    const { calls } = mockApi(
      routes({
        'GET /admin/libraries/2/metadata': () => json(current),
        'PUT /admin/libraries/2/metadata': (call) => {
          const body = call.body as LibraryMetadataUpdate;
          current = libraryMeta({ textLangs: body.textLangs ?? null, images: body.images ?? null });
          return json(current);
        },
      }),
    );
    renderApp('/admin/metadata?library=2');
    expect(await screen.findByText(/Serien übernimmt den Standard/)).toBeVisible();
    const user = userEvent.setup();
    expect(
      within(screen.getByRole('radiogroup', { name: 'Geltung' })).getByRole('radio', {
        name: 'Serien',
      }),
    ).toBeChecked();
    // Quality, sources, rating and apply belong to the server: not on a library.
    expect(screen.queryByRole('region', { name: 'Güte' })).toBeNull();
    expect(screen.queryByRole('region', { name: 'Altersfreigabe' })).toBeNull();
    const languages = region('Sprachen');
    expect(within(languages).getByText('wie Server')).toBeVisible();
    const list = within(languages).getByRole('group', { name: 'Metadaten-Sprachen' });
    expect(within(list).getByRole('button', { name: 'Englisch entfernen' })).toBeDisabled();

    await user.click(screen.getByRole('switch', { name: 'Sprachen abweichend' }));
    expect(within(languages).queryByText('wie Server')).toBeNull();
    await user.click(within(list).getByRole('button', { name: 'Englisch entfernen' }));
    expect(within(list).getAllByRole('listitem')).toHaveLength(1);
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/libraries/2/metadata')).toHaveLength(1));
    expect(puts(calls, '/admin/libraries/2/metadata')[0]).toEqual({
      textLangs: ['de'],
      images: null,
    });
    expect(await screen.findByText('Gespeichert für Serien.')).toBeVisible();

    // Switching the deviation off and saving sends null: the library follows the server again.
    await user.click(screen.getByRole('switch', { name: 'Sprachen abweichend' }));
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls, '/admin/libraries/2/metadata')).toHaveLength(2));
    expect(puts(calls, '/admin/libraries/2/metadata')[1]).toEqual({
      textLangs: null,
      images: null,
    });
  });

  it('switches the scope from the server to a library and back without losing the server form', async () => {
    mockApi(routes());
    renderApp('/admin/metadata');
    await screen.findByRole('region', { name: 'Sprachen' });
    const user = userEvent.setup();
    const scope = screen.getByRole('radiogroup', { name: 'Geltung' });
    await user.click(within(scope).getByRole('radio', { name: 'Serien' }));
    expect(await screen.findByText(/Serien übernimmt den Standard/)).toBeVisible();
    await user.click(within(scope).getByRole('radio', { name: 'Standard' }));
    expect(
      await screen.findByText('Gilt für alle Bibliotheken ohne eigene Abweichung.'),
    ).toBeVisible();
    expect(screen.getByRole('region', { name: 'Güte' })).toBeVisible();
  });

  it('"Jetzt anwenden" posts what and where and names the queued run', async () => {
    const { calls } = mockApi(
      routes({
        'POST /admin/metadata/apply': () => json({ jobId: 99, titles: 40 }, 202),
      }),
    );
    renderApp('/admin/metadata');
    const apply = await screen.findByRole('region', { name: 'Auf bestehende Titel anwenden' });
    const user = userEvent.setup();
    await user.click(within(apply).getByRole('radio', { name: 'Nur Texte' }));
    await user.click(within(apply).getByRole('radio', { name: 'Serien' }));
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
