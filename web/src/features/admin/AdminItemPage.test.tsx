import type { AdminItem } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

const item: AdminItem = {
  id: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  libraryId: 1,
  metadataState: 'unmatched',
  metadataError: 'no_match',
  anilistId: 1234,
  providers: ['tmdb', 'tvdb', 'anilist'],
  // Out of order on purpose: the page sorts by field, then language.
  fieldSources: [
    { field: 'poster', lang: '', provider: 'tmdb' },
    { field: 'overview', lang: 'en', provider: 'anilist' },
    { field: 'title', lang: 'en', provider: 'tmdb' },
    { field: 'genres', lang: '', provider: 'union' },
    { field: 'overview', lang: 'de', provider: 'tvdb' },
    { field: 'title', lang: 'de', provider: 'tmdb' },
  ],
  lockedFields: ['rating', 'title'],
  ownRating: 16,
  effectiveRating: 16,
  ratingSource: 'manual',
  trailerState: 'none',
  trailers: [
    {
      id: 5,
      lang: 'de',
      youtubeId: 'abcdefghijk',
      embedUrl: 'https://www.youtube-nocookie.com/embed/abcdefghijk',
      source: 'manual',
      official: false,
    },
  ],
  mediaFiles: [
    {
      id: 'mf-1',
      path: 'Alien (1979)/Alien.mkv',
      sizeBytes: 4_200_000_000,
      available: true,
      versionLabel: '1080p',
      probedAt: '2026-10-18T18:00:00Z',
    },
    {
      id: 'mf-2',
      path: 'Alien (1979)/Alien - broken.mkv',
      sizeBytes: 12,
      available: false,
      probedAt: '2026-10-18T18:00:00Z',
      probeError: 'invalid data found when processing input',
    },
  ],
  children: { seasons: 0, episodes: 0, extras: 2 },
};

const base = `/admin/items/${ids.alien}`;

const tabPanel = () => screen.getByRole('tabpanel');
const openTab = async (user: ReturnType<typeof userEvent.setup>, name: RegExp) => {
  await user.click(await screen.findByRole('tab', { name }));
  return tabPanel();
};
/** Opens a disclosure of the tab «Metadaten» and gives its panel. */
const openDisclosure = async (user: ReturnType<typeof userEvent.setup>, name: RegExp) => {
  const trigger = await screen.findByRole('button', { name });
  if (trigger.getAttribute('aria-expanded') !== 'true') await user.click(trigger);
  return screen.getByRole('group', { name });
};

describe('admin item page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows kind, year, state, ids and the tabs with their counts; the link to the detail page', async () => {
    mockApi({ ...signedIn, [`GET ${base}`]: () => json(item) });
    renderApp(base);

    expect(await screen.findByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    // Kind and year below the h1 (TIT-1).
    expect(screen.getByText('1979')).toBeVisible();
    expect(screen.getByText('1979').parentElement).toHaveTextContent(/^Film1979$/);
    expect(screen.getByRole('link', { name: 'Zur Detailseite' })).toHaveAttribute(
      'href',
      `/items/${ids.alien}`,
    );

    const tabs = screen.getByRole('tablist', { name: 'Bereiche des Titels' });
    // The number shows after the label and is spoken with its unit.
    const names = ['Metadaten', 'Bilder 3 Bildarten', 'Dateien 2 Dateien', 'Trailer 1 Trailer'];
    const all = within(tabs).getAllByRole('tab');
    expect(all).toHaveLength(names.length);
    names.forEach((name, index) => expect(all[index]).toHaveAccessibleName(name));
    expect(all[2]).toHaveTextContent(/^Dateien 2/);
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveTextContent('Metadaten');

    const facts = within(tabPanel()).getByRole('region', { name: 'Metadaten' });
    expect(within(facts).getByText('Nicht zugeordnet')).toBeVisible();
    expect(within(facts).getByText('no_match')).toBeVisible();
    expect(within(facts).getByText('rating, title')).toBeVisible();
    expect(within(facts).getByText('Staffeln: 0 · Folgen: 0 · Extras: 2')).toBeVisible();
  });

  it('shows the files with name, version, size and state; a row folds out into path, probe date and error', async () => {
    const user = userEvent.setup();
    mockApi({ ...signedIn, [`GET ${base}`]: () => json(item) });
    renderApp(base);
    const files = within(await openTab(user, /^Dateien/)).getByRole('region', { name: 'Dateien' });
    const rows = within(files).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(2);
    expect(within(rows[0]!).getByText('1080p')).toBeVisible();
    expect(within(rows[1]!).getByText('nicht verfügbar')).toBeVisible();
    expect(within(rows[1]!).getByText('Prüffehler')).toBeVisible();
    // The whole path and the error are in the fold-out, by button.
    expect(screen.queryByText('invalid data found when processing input')).toBeNull();
    // The row names the file and its folder, so equal names stay apart.
    const broken = within(files).getByRole('button', { name: 'Alien - broken.mkv Alien (1979)' });
    expect(broken).toHaveAttribute('aria-expanded', 'false');
    await user.click(broken);
    expect(broken).toHaveAttribute('aria-expanded', 'true');
    expect(within(files).getByText('Alien (1979)/Alien - broken.mkv')).toBeVisible();
    expect(within(files).getByText('invalid data found when processing input')).toBeVisible();
    expect(within(files).getByText(/^Geprüft am /)).toBeVisible();

    // Another tab and back: the row is still open.
    await openTab(user, /^Trailer/);
    await openTab(user, /^Dateien/);
    expect(screen.getByText('invalid data found when processing input')).toBeVisible();
  });

  it('shows the metadata sources, the AniList entry and which provider won each field', async () => {
    const user = userEvent.setup();
    mockApi({ ...signedIn, [`GET ${base}`]: () => json(item) });
    const { unmount } = renderApp(base);

    const facts = await screen.findByRole('region', { name: 'Metadaten' });
    expect(within(facts).getByText('TMDB › TVDB › AniList')).toBeVisible();
    // The id is the link to the AniList entry.
    const anilist = within(facts).getByRole('link', { name: '1234' });
    expect(anilist).toHaveAttribute('href', 'https://anilist.co/anime/1234');
    expect(anilist).toHaveAttribute('target', '_blank');

    // «Herkunft je Feld» is a disclosure with the number of fields; closed at first.
    const trigger = screen.getByRole('button', { name: /^Herkunft je Feld/ });
    expect(trigger).toHaveAccessibleName('Herkunft je Feld 6 Felder');
    // Next to it, closed or open: how many fields each provider won.
    expect(
      screen.getByText('TMDB 3, TVDB 1, AniList 1, Vereinigung aller Quellen 1'),
    ).toBeVisible();
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    const sources = await openDisclosure(user, /^Herkunft je Feld/);
    const rows = [...sources.querySelectorAll('dt')].map((term) => [
      term.textContent,
      term.nextElementSibling?.textContent,
    ]);
    expect(rows).toEqual([
      ['Titel (de)', 'TMDB'],
      ['Titel (en)', 'TMDB'],
      ['Beschreibung (de)', 'TVDB'],
      ['Beschreibung (en)', 'AniList'],
      ['Genres', 'Vereinigung aller Quellen'],
      ['Poster', 'TMDB'],
    ]);
    unmount();

    // No fill job ran yet, and AniList had no entry: no sources disclosure, no link.
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json({ ...item, anilistId: 0, fieldSources: [] }),
    });
    renderApp(base);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    expect(screen.queryByRole('button', { name: /^Herkunft je Feld/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /AniList/ })).not.toBeInTheDocument();
  });

  it('shows the episode range of a double episode', async () => {
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () =>
        json({ ...item, kind: 'episode', seasonNumber: 1, episodeNumber: 3, episodeEnd: 4 }),
    });
    renderApp(base);
    const facts = await screen.findByRole('region', { name: 'Metadaten' });
    expect(within(facts).getByText('3–4')).toBeVisible();
  });

  it('refreshes the title', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(item),
      [`POST ${base}/refresh`]: () => json({ ...item, metadataState: 'pending' }, 202),
    });
    renderApp(base);
    await user.click(await screen.findByRole('button', { name: 'Aktualisieren' }));
    await waitFor(() => expect(calls.some((call) => call.path === `${base}/refresh`)).toBe(true));
    expect(await screen.findByText('Aktualisierung eingereiht.')).toBeVisible();
  });

  it('keeps the rating choice across tabs (S-17), asks before leaving, then saves it', async () => {
    const user = userEvent.setup();
    /** The chip «ungespeichert» in the header of «Freigabe» (the tab's mark is spoken only). */
    const chip = () => screen.getByText('ungespeichert', { selector: '[class*="chip"]' });
    let stored: AdminItem = item;
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(stored),
      [`PUT ${base}/rating`]: () => {
        stored = { ...item, ownRating: undefined, effectiveRating: 18, ratingSource: 'missing' };
        return json({ id: item.id, kind: 'movie', title: 'Alien', effectiveRating: 18 });
      },
    });
    renderApp(base);
    // Closed, «Freigabe» shows its summary in the header.
    expect(await screen.findByText('Wirksam FSK 16 · Eigene FSK 16 · Quelle manual')).toBeVisible();
    let rating = await openDisclosure(user, /^Freigabe/);
    expect(within(rating).getByText('manual')).toBeVisible();
    expect(within(rating).getByRole('radio', { name: 'FSK 16' })).toBeChecked();
    await user.click(within(rating).getByRole('radio', { name: 'Aus Metadaten' }));
    expect(chip()).toBeVisible();
    // The tab «Metadaten» carries a mark while the choice is unsaved, seen from every tab.
    expect(screen.getByRole('tab', { name: 'Metadaten ungespeichert' })).toBeVisible();

    // Another tab and back, then folding «Freigabe» away and out: the choice stays.
    await openTab(user, /^Dateien/);
    expect(screen.getByRole('tab', { name: 'Metadaten ungespeichert' })).toBeVisible();
    await openTab(user, /^Metadaten/);
    expect(chip()).toBeVisible();
    await user.click(screen.getByRole('button', { name: /^Freigabe/ }));
    rating = await openDisclosure(user, /^Freigabe/);
    expect(within(rating).getByRole('radio', { name: 'Aus Metadaten' })).toBeChecked();

    // Leaving the page with the choice asks; staying keeps it.
    await user.click(screen.getByRole('link', { name: 'Zur Detailseite' }));
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    expect(within(rating).getByRole('radio', { name: 'Aus Metadaten' })).toBeChecked();

    await user.click(within(rating).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `${base}/rating`)?.body).toEqual({ rating: null }),
    );
    // The choice starts anew from the stored value (source "missing"); the confirmation stays.
    await waitFor(() => expect(within(rating).getByText('missing')).toBeVisible());
    expect(screen.getByText('Freigabe gespeichert.')).toBeVisible();
    expect(screen.queryByText('ungespeichert')).toBeNull();
    expect(screen.getByRole('tab', { name: 'Metadaten' })).toBeVisible();
    expect(screen.getByText('Wirksam FSK 18 · Eigene – · Quelle missing')).toBeVisible();
    expect(within(rating).getByRole('radio', { name: 'Aus Metadaten' })).toBeChecked();
  });

  it('sets the original language (concept 13 11.7): the select, the draft, the lock and «Aus Metadaten»', async () => {
    const user = userEvent.setup();
    let stored: AdminItem = {
      ...item,
      originalLanguage: 'en',
      fieldSources: [
        ...item.fieldSources,
        { field: 'original_language', lang: '', provider: 'tmdb' },
      ],
    };
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(stored),
      'GET /admin/settings/metadata': () =>
        json({ knownLanguages: ['de', 'en', 'ja', 'cn', 'xx', 'nb', 'mo', 'sh', 'pt-BR'] }),
      [`PUT ${base}/original-language`]: (call) => {
        const language = (call.body as { language: string | null }).language;
        stored =
          language === null
            ? { ...item, originalLanguage: undefined }
            : {
                ...item,
                originalLanguage: language,
                lockedFields: [...item.lockedFields, 'original_language'],
              };
        return json(stored);
      },
    });
    renderApp(base);
    // Closed, the header names the language and the provider that won it.
    expect(await screen.findByText('Englisch · Quelle TMDB')).toBeVisible();
    const panel = await openDisclosure(user, /^Originalsprache/);
    const select = within(panel).getByRole('combobox', { name: 'Originalsprache setzen' });
    expect(select).toHaveValue('metadata');
    // TMDB's cn and xx, nb (no), the withdrawn mo and sh and the regional codes are no choice;
    // «Unbekannt» is.
    await waitFor(() =>
      expect(within(select).getByRole('option', { name: 'Japanisch' })).toBeInTheDocument(),
    );
    const values = within(select)
      .getAllByRole('option')
      .map((option) => (option as HTMLOptionElement).value);
    expect(values).toEqual(['metadata', '', 'de', 'en', 'ja']);

    await user.selectOptions(select, 'ja');
    // The chip «ungespeichert» sits in the header of «Originalsprache».
    expect(screen.getByText('ungespeichert', { selector: '[class*="chip"]' })).toBeVisible();
    expect(screen.getByRole('tab', { name: 'Metadaten ungespeichert' })).toBeVisible();
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `${base}/original-language`)?.body).toEqual({
        language: 'ja',
      }),
    );
    expect(await screen.findByText('Originalsprache gespeichert.')).toBeVisible();
    expect(screen.getByText('Japanisch · Quelle Von Hand (gesperrt)')).toBeVisible();
    expect(select).toHaveValue('ja');
    expect(screen.getByRole('tab', { name: 'Metadaten' })).toBeVisible();

    // «Aus Metadaten» lifts the lock: null, and the title is «noch nicht abgefragt» until the match.
    await user.selectOptions(select, 'metadata');
    await user.click(within(panel).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(
        calls.filter((call) => call.path === `${base}/original-language`).at(-1)?.body,
      ).toEqual({
        language: null,
      }),
    );
    expect(await screen.findByText('Noch nicht abgefragt · Quelle –')).toBeVisible();
    expect(select).toHaveValue('metadata');
  });

  it('shows no original language for an episode', async () => {
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json({ ...item, kind: 'episode', seasonNumber: 1, episodeNumber: 1 }),
    });
    renderApp(base);
    expect(await screen.findByRole('button', { name: /^Freigabe/ })).toBeVisible();
    expect(screen.queryByRole('button', { name: /^Originalsprache/ })).toBeNull();
  });

  it('«Speichern» of an unchanged rating sends nothing and keeps the focus', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({ ...signedIn, [`GET ${base}`]: () => json(item) });
    renderApp(base);
    const rating = await openDisclosure(user, /^Freigabe/);
    const save = within(rating).getByRole('button', { name: 'Speichern' });
    await user.click(save);
    expect(save).toHaveFocus();
    expect(calls.some((call) => call.path === `${base}/rating`)).toBe(false);
  });

  it('lists each trailer with its language and marks the official one (TIT-7)', async () => {
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () =>
        json({
          ...item,
          trailerState: 'found',
          trailers: [
            {
              id: 6,
              lang: 'en',
              youtubeId: 'officialEN1',
              embedUrl: 'https://www.youtube-nocookie.com/embed/officialEN1',
              title: 'Official Trailer',
              source: 'tmdb',
              official: true,
            },
            ...item.trailers,
          ],
        } satisfies AdminItem),
    });
    renderApp(`${base}?tab=trailers`);
    const trailers = await screen.findByRole('region', { name: 'Trailer' });
    const row = (youtubeId: string) =>
      within(within(trailers).getByText(youtubeId).closest('tr') as HTMLElement);
    expect(row('officialEN1').getByText('EN')).toBeVisible();
    expect(row('officialEN1').getByText('Official Trailer')).toBeVisible();
    expect(row('officialEN1').getByText('TMDB')).toBeVisible();
    expect(row('officialEN1').getByText('Offiziell')).toBeVisible();
    expect(row('abcdefghijk').getByText('DE')).toBeVisible();
    expect(row('abcdefghijk').getByText('Manuell')).toBeVisible();
    expect(row('abcdefghijk').queryByText('Offiziell')).not.toBeInTheDocument();
  });

  it('adds a manual trailer from a URL and deletes one after confirmation', async () => {
    const user = userEvent.setup();
    let current: AdminItem = item;
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(current),
      [`POST ${base}/trailers`]: () => json(item.trailers[0], 201),
      [`DELETE ${base}/trailers/5`]: () => {
        current = { ...item, trailers: [] };
        return new Response(null, { status: 204 });
      },
    });
    renderApp(`${base}?tab=trailers`);
    const trailers = await screen.findByRole('region', { name: 'Trailer' });
    expect(within(trailers).getByText('Keiner gefunden')).toBeVisible();
    expect(within(trailers).getByText('abcdefghijk')).toBeVisible();
    expect(within(trailers).getByText('Manuell')).toBeVisible();

    await user.click(within(trailers).getByRole('button', { name: 'Hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Trailer hinzufügen' });
    const source = within(dialog).getByLabelText('YouTube-Link oder Video-ID');
    expect(source).toHaveFocus();
    await user.type(source, 'https://youtu.be/dQw4w9WgXcQ');
    await user.click(within(dialog).getByRole('radio', { name: 'Andere' }));
    await user.type(within(dialog).getByLabelText('Sprachcode'), 'fr');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `${base}/trailers`)?.body).toEqual({
        lang: 'fr',
        url: 'https://youtu.be/dQw4w9WgXcQ',
      }),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(await screen.findByText('Trailer hinzugefügt.')).toBeVisible();

    const remove = within(trailers).getByRole('button', { name: 'Löschen' });
    expect(remove).toHaveAccessibleDescription('abcdefghijk');
    await user.click(remove);
    const confirm = await screen.findByRole('alertdialog', { name: 'Trailer löschen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Löschen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === `${base}/trailers/5`),
      ).toBe(true),
    );
    expect(await screen.findByText('Trailer gelöscht.')).toBeVisible();
    expect(await within(trailers).findByText('Keine Trailer.')).toBeVisible();
    // The row went with its button: the h1 has the focus (ALL-2).
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Alien' })).toHaveFocus(),
    );
  });

  it('sends a bare video id as youtubeId and shows the field error of the server', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(item),
      [`POST ${base}/trailers`]: () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'youtubeId', code: 'invalid_value' }],
        }),
    });
    renderApp(base);
    const trailers = within(await openTab(user, /^Trailer/));
    await user.click(trailers.getByRole('button', { name: 'Hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Trailer hinzufügen' });
    await user.type(within(dialog).getByLabelText('YouTube-Link oder Video-ID'), 'dQw4w9WgXcQ');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `${base}/trailers`)?.body).toEqual({
        lang: 'de',
        youtubeId: 'dQw4w9WgXcQ',
      }),
    );
    expect(await within(dialog).findByText('Ungültiger Wert.')).toBeVisible();
  });

  it('opens the match dialog for films and series only', async () => {
    const user = userEvent.setup();
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(item),
      'GET /admin/metadata/search': () => problem(422, 'provider_not_configured'),
    });
    const { unmount } = renderApp(base);
    await user.click(await screen.findByRole('button', { name: 'Zuordnen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Titel zuordnen' });
    await user.click(within(dialog).getByRole('radio', { name: 'TVDB' }));
    // TVDB knows series only.
    expect(within(dialog).getByRole('radio', { name: 'Serie' })).toBeChecked();
    expect(within(dialog).getByRole('radio', { name: 'Film' })).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Suchen' }));
    expect(await within(dialog).findByText(/Dieser Anbieter ist nicht eingerichtet/)).toBeVisible();
    unmount();

    mockApi({ ...signedIn, [`GET ${base}`]: () => json({ ...item, kind: 'episode' }) });
    renderApp(base);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    expect(screen.queryByRole('button', { name: 'Zuordnen' })).not.toBeInTheDocument();
  });

  it('shows the decided images per kind with source, language and lock (concept 10 7.3)', async () => {
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () =>
        json({
          ...item,
          images: [
            {
              kind: 'poster',
              provider: 'tmdb',
              lang: 'de',
              source: '/p.jpg',
              previewUrl: 'https://image.tmdb.org/t/p/w300/p.jpg',
              chosen: true,
              locked: true,
            },
            { kind: 'backdrop', chosen: false, locked: true },
          ],
        }),
    });
    renderApp(`${base}?tab=images`);
    const images = await screen.findByRole('region', { name: 'Bilder' });
    expect(
      within(images).getByText(
        'Je Art das gewählte Bild mit Quelle und Sprache. Eine eigene Wahl sperrt die Art; «Anwenden» und «Aktualisieren» behalten sie.',
      ),
    ).toBeVisible();
    const rows = within(images).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(3);
    expect(within(rows[0]!).getByRole('rowheader')).toHaveTextContent('Poster');
    expect(within(rows[0]!).getByText('TMDB')).toBeVisible();
    expect(within(rows[0]!).getByText('Deutsch')).toBeVisible();
    expect(within(rows[0]!).getByText('von Hand gewählt')).toBeVisible();
    expect(within(rows[0]!).getByText('gesperrt')).toBeVisible();
    expect(within(rows[1]!).getByText('Sperre ohne Wahl')).toBeVisible();
    expect(within(rows[2]!).getByRole('rowheader')).toHaveTextContent('Logo');
    expect(within(rows[2]!).getByText('Kein Bild')).toBeVisible();
    // «Andere wählen» names its kind; only a locked kind offers to lift the lock.
    expect(
      within(rows[0]!).getByRole('button', { name: 'Andere wählen' }),
    ).toHaveAccessibleDescription('Poster');
    expect(within(rows[0]!).getByRole('button', { name: 'Sperre aufheben' })).toBeVisible();
    expect(within(rows[2]!).queryByRole('button', { name: 'Sperre aufheben' })).toBeNull();
  });

  it('chooses a candidate from the grid and lifts a lock again', async () => {
    let current: AdminItem = { ...item, images: [] };
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(current),
      [`GET ${base}/image-candidates`]: () =>
        json({
          candidates: [
            {
              provider: 'local',
              lang: '',
              width: 600,
              height: 900,
              score: 0,
              voteCount: 0,
              source: 'file:Alien/poster.jpg?1-1',
              previewUrl: '/api/v1/preview',
              chosen: false,
            },
            {
              provider: 'tmdb',
              lang: 'en',
              width: 2000,
              height: 3000,
              score: 5.4,
              voteCount: 12,
              source: '/en.jpg',
              previewUrl: 'https://image.tmdb.org/t/p/w300/en.jpg',
              chosen: true,
            },
          ],
          unavailable: ['tvdb', 'fanart'],
        }),
      [`PUT ${base}/images/poster`]: () => {
        current = {
          ...item,
          images: [
            {
              kind: 'poster',
              provider: 'local',
              lang: '',
              source: 'file:Alien/poster.jpg?1-1',
              chosen: true,
              locked: true,
            },
          ],
        };
        return json(current);
      },
      [`DELETE ${base}/images/poster`]: () => {
        current = { ...item, images: [] };
        return json(current);
      },
    });
    renderApp(base);
    const user = userEvent.setup();
    const images = within(await openTab(user, /^Bilder/)).getByRole('region', { name: 'Bilder' });
    await user.click(within(images).getAllByRole('button', { name: 'Andere wählen' })[0]!);
    const dialog = await screen.findByRole('dialog', { name: 'Poster wählen' });
    expect(
      await within(dialog).findByText(
        'Nicht erreichbar: TVDB, Fanart.tv. Die Liste zeigt die übrigen Quellen.',
      ),
    ).toBeVisible();
    const grid = within(dialog).getByRole('listbox', { name: 'Kandidaten für Poster' });
    const tiles = within(grid).getAllByRole('option');
    expect(tiles).toHaveLength(2);
    expect(tiles[0]).toHaveAccessibleName('Ohne Text, 600 × 900 · Lokale Datei wählen');
    expect(tiles[1]).toHaveAccessibleName('Englisch, 2000 × 3000 · TMDB · Bewertung 5,4 wählen');
    expect(within(tiles[1]!).getByText('aktuell')).toBeVisible();
    await user.click(tiles[0]!);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      provider: 'local',
      source: 'file:Alien/poster.jpg?1-1',
    });
    expect(
      await screen.findByText('Poster gewählt und gesperrt; das Bild wird geholt.'),
    ).toBeVisible();

    await user.click(await within(images).findByRole('button', { name: 'Sperre aufheben' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
    expect(
      await screen.findByText('Sperre für Poster aufgehoben; die automatische Wahl läuft.'),
    ).toBeVisible();
    // The button went with the lock: the h1 has the focus (ALL-2).
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Alien' })).toHaveFocus(),
    );
  });

  it('binds the tab to the address; an unknown tab, or «Bilder» of an episode, falls back', async () => {
    const user = userEvent.setup();
    mockApi({ ...signedIn, [`GET ${base}`]: () => json(item) });
    const { router, unmount } = renderApp(`${base}?tab=files`);
    expect(await screen.findByRole('tab', { selected: true })).toHaveTextContent(/^Dateien/);
    await openTab(user, /^Trailer/);
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'trailers' }));
    await openTab(user, /^Metadaten/);
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    unmount();

    mockApi({ ...signedIn, [`GET ${base}`]: () => json(item) });
    const unknown = renderApp(`${base}?tab=nonsense`);
    await waitFor(() => expect(unknown.router.state.location.search).toEqual({}));
    expect(screen.getByRole('tab', { selected: true })).toHaveTextContent('Metadaten');
    unknown.unmount();

    mockApi({ ...signedIn, [`GET ${base}`]: () => json({ ...item, kind: 'episode' }) });
    const episode = renderApp(`${base}?tab=images`);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    await waitFor(() => expect(episode.router.state.location.search).toEqual({}));
    expect(screen.queryByRole('tab', { name: /^Bilder/ })).toBeNull();
    expect(screen.queryByRole('region', { name: 'Bilder' })).toBeNull();
  });

  it('answers an unknown title and a profile without rights', async () => {
    mockApi({ ...signedIn, [`GET ${base}`]: () => problem(404, 'not_found') });
    const { unmount } = renderApp(base);
    expect(await screen.findByText('Diesen Titel gibt es nicht oder nicht mehr.')).toBeVisible();
    expect(screen.queryByRole('tablist')).toBeNull();
    unmount();

    mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, id: 2, isPrimary: false } }),
    });
    renderApp(base);
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
  });

  it('names the kind when lifting its lock fails', async () => {
    const user = userEvent.setup();
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () =>
        json({
          ...item,
          images: [{ kind: 'backdrop', provider: 'tmdb', lang: '', chosen: true, locked: true }],
        }),
      [`DELETE ${base}/images/backdrop`]: () => problem(500, 'internal'),
    });
    renderApp(`${base}?tab=images`);
    const images = await screen.findByRole('region', { name: 'Bilder' });
    await user.click(within(images).getByRole('button', { name: 'Sperre aufheben' }));
    expect(
      await within(images).findByText('Sperre für Hintergrund nicht aufgehoben.'),
    ).toBeVisible();
  });

  it('keeps the title on the page when reading it again fails', async () => {
    const user = userEvent.setup();
    let reads = 0;
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () => {
        reads += 1;
        return reads === 1 ? json(item) : problem(500, 'internal');
      },
      [`POST ${base}/refresh`]: () => json(item, 202),
    });
    renderApp(base);
    await user.click(await screen.findByRole('button', { name: 'Aktualisieren' }));
    expect(await screen.findByRole('button', { name: 'Erneut versuchen' })).toBeVisible();
    expect(screen.getByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    expect(screen.getByRole('tablist', { name: 'Bereiche des Titels' })).toBeVisible();
  });

  it('offers to try again when the title cannot be read', async () => {
    const user = userEvent.setup();
    let fail = true;
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () => (fail ? problem(500, 'internal') : json(item)),
    });
    renderApp(base);
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
  });
});
