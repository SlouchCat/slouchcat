import type { TraktStatus, components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import de from '@/locales/de.json';
import { json, mockApi, problem, renderApp, signedIn, type Call } from '@/test/api';

type SpotlightConfig = components['schemas']['SpotlightConfig'];
type SpotlightEntry = components['schemas']['SpotlightEntry'];
type SpotlightEntryOverride = components['schemas']['SpotlightEntryOverride'];
type SpotlightListKey = components['schemas']['SpotlightListKey'];

/**
 * The key of the list kind the locale names `label`. The single-medium kind is named by its label
 * only, so web/src spells no list key that would hint at a table of kinds (plan step 5, DoD
 * "Medienarten kommen vom Server": the grep for that key stays empty).
 */
function keyOf(label: string): SpotlightListKey {
  const entry = Object.entries(de.admin.spotlight.lists).find(([, text]) => text === label);
  if (!entry) throw new Error(`no list kind is named ${label}`);
  return entry[0] as SpotlightListKey;
}

const all = ['movie', 'show', 'both'] as const;
// The catalogue as the server sends it (plan 5.6); the page must read everything from here.
const config: SpotlightConfig = {
  enabled: true,
  listKey: 'trending',
  media: 'both',
  itemCount: 20,
  minHits: 5,
  intervalHours: 24,
  rotationSeconds: 5,
  listKinds: [
    { key: 'trending', media: [...all], mediaFilters: false, needsUrl: false },
    { key: 'popular', media: [...all], mediaFilters: false, needsUrl: false },
    { key: 'watched_weekly', media: [...all], mediaFilters: false, needsUrl: false },
    { key: 'collected_monthly', media: [...all], mediaFilters: false, needsUrl: false },
    { key: 'anticipated', media: [...all], mediaFilters: false, needsUrl: false },
    { key: keyOf('Kinocharts'), media: ['movie'], mediaFilters: false, needsUrl: false },
    { key: 'url', media: [...all], mediaFilters: true, needsUrl: true },
  ],
};
const status: TraktStatus = {
  configured: true,
  lastSuccess: '2026-10-20T06:00:00Z',
  lastAttempt: '2026-10-20T06:00:00Z',
  listKey: 'trending',
  entries: 40,
};
const dune: SpotlightEntry = {
  media: 'movie',
  tmdbId: 7001,
  position: 1,
  title: 'Dune',
  year: 2021,
  itemId: '01J00000000000000000000001',
  itemTitle: 'Dune: Part One',
  hidden: false,
  orphaned: false,
  posterUrl: '/api/v1/admin/items/01J00000000000000000000001/images/poster/w200?v=dune',
};
const severance: SpotlightEntry = {
  media: 'show',
  tmdbId: 7101,
  position: 2,
  title: 'Severance',
  year: 2022,
  hidden: false,
  orphaned: false,
};
const orphan: SpotlightEntry = {
  media: 'movie',
  tmdbId: 9001,
  position: 0,
  title: 'Old Film',
  hidden: true,
  note: 'Alt',
  orphaned: true,
};

/** The routes of the page with a server that keeps the overrides it is sent. */
function spotlightApi(overrides: Record<string, unknown> = {}) {
  let entries = [dune, severance];
  let orphans = [orphan];
  const apply = (call: Call) => {
    const [, , , , media, id] = call.path.split('/');
    const body = call.body as SpotlightEntryOverride;
    const match = (entry: SpotlightEntry) => entry.media === media && entry.tmdbId === Number(id);
    if (Object.keys(body).length === 0) {
      const removed = orphans.find(match);
      orphans = orphans.filter((entry) => !match(entry));
      return json(removed);
    }
    let saved: SpotlightEntry | undefined;
    entries = entries.map((entry) => {
      if (!match(entry)) return entry;
      const next: SpotlightEntry = { ...entry };
      if (body.hidden !== undefined) next.hidden = body.hidden;
      if (body.pinnedRank === null) delete next.pinnedRank;
      else if (body.pinnedRank !== undefined) next.pinnedRank = body.pinnedRank;
      if (body.note === null) delete next.note;
      else if (body.note !== undefined) next.note = body.note;
      saved = next;
      return next;
    });
    return json(saved);
  };
  return mockApi({
    ...signedIn,
    'GET /admin/trakt': () => json(status),
    'GET /admin/spotlight': () => json(config),
    'GET /admin/spotlight/entries': () => json({ listKey: 'trending', entries, orphans }),
    'PATCH /admin/spotlight/entries/*/*': apply,
    ...overrides,
  });
}

const entriesTable = () => screen.findByRole('table', { name: 'Einträge der Liste' });
const source = () => screen.findByRole('region', { name: 'Quelle' });
const patches = (calls: Call[]) =>
  calls.filter((call) => call.method === 'PATCH').map((call) => [call.path, call.body]);

describe('admin spotlight (M11, ADR 0011, mockup H)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('shows the switch, the source with the state of the fetch, and the entries', async () => {
    spotlightApi();
    renderApp('/admin/spotlight');

    expect(await screen.findByRole('heading', { level: 1, name: 'Spotlight' })).toBeVisible();
    expect(
      screen.getByRole('navigation', { name: 'Verwaltung' }).querySelector('[aria-current="page"]'),
    ).toHaveTextContent('Spotlight');
    expect(await screen.findByRole('switch', { name: 'Auf der Startseite zeigen' })).toBeChecked();

    const card = await source();
    expect(within(card).getByRole('radio', { name: 'Trending bei Trakt' })).toBeChecked();
    expect(within(card).getByRole('radio', { name: 'Filme und Serien' })).toBeChecked();
    // Plan 5.5: the per-profile numbers are named apart from the system-wide counts below.
    const count = within(card).getByLabelText('Höchstens');
    expect(count).toHaveValue(20);
    expect(count).toHaveAccessibleDescription('Titel in der Reihe, je Profil (5–20)');
    const below = within(card).getByLabelText('Ausblenden unter');
    expect(below).toHaveValue(5);
    expect(below).toHaveAccessibleDescription('Treffern in der Bibliothek, je Profil (0–10)');
    expect(within(card).getByLabelText('Abruf alle … Stunden')).toHaveValue(24);

    // The card's foot: fetched, hits of the fetched entries, the key as a link (never its value).
    expect(await within(card).findByText(/^Zuletzt abgerufen /)).toBeVisible();
    expect(await within(card).findByText('1 von 40 Titeln in der Bibliothek')).toBeVisible();
    expect(within(card).getByRole('link', { name: 'Schlüssel gesetzt' })).toHaveAttribute(
      'href',
      '/admin/integrations',
    );
    // Plan 7.1: one hit is below the minimum of five, so the page says why the row is missing.
    expect(
      within(card).getByText(/Nur 1 Treffer in der Bibliothek, weniger als die verlangten 5/),
    ).toBeVisible();
    expect(within(card).getByRole('button', { name: 'Jetzt abrufen' })).toBeVisible();

    const table = await entriesTable();
    const [first, second] = within(table).getAllByRole('row').slice(1);
    expect(within(first!).getByRole('rowheader')).toHaveTextContent('DuneFilm · 2021');
    // Mockup H: the poster in front of the title, decorative (system-wide admin image route).
    const poster = within(first!).getByRole('rowheader').querySelector('img');
    expect(poster).toHaveAttribute('alt', '');
    expect(poster).toHaveAttribute('src', dune.posterUrl);
    expect(within(first!).getByText('Sichtbar')).toBeVisible();
    expect(within(first!).getByRole('button', { name: 'Dune anheften' })).toBeVisible();
    // Plan S-4: an entry without a library title is listed, says so and has no actions.
    expect(within(second!).getByRole('rowheader')).toHaveTextContent('SeveranceSerie · 2022');
    // Without a library hit there is no poster: the placeholder, never a broken image.
    expect(within(second!).getByRole('rowheader').querySelector('img')).toBeNull();
    expect(within(second!).getByRole('rowheader').querySelector('svg')).not.toBeNull();
    expect(within(second!).getByText('Nicht in der Bibliothek')).toBeVisible();
    expect(
      within(second!).getByText('Nicht in der Bibliothek, erscheint nicht auf der Startseite'),
    ).toBeVisible();
    expect(within(second!).queryByRole('button')).toBeNull();
    expect(screen.getByRole('link', { name: 'Zur Seite «Startseite»' })).toHaveAttribute(
      'href',
      '/admin/home-rows',
    );
  });

  it('offers the media the chosen list allows: box office is locked to films, trending has all three', async () => {
    spotlightApi();
    renderApp('/admin/spotlight');
    const user = userEvent.setup();
    const card = await source();
    const media = () => within(card).getByRole('radiogroup', { name: 'Medienart' });

    expect(within(media()).getAllByRole('radio')).toHaveLength(3);
    await user.click(within(card).getByRole('radio', { name: 'Kinocharts' }));
    expect(within(media()).getAllByRole('radio')).toHaveLength(1);
    const films = within(media()).getByRole('radio', { name: 'Nur Filme' });
    expect(films).toBeChecked();
    expect(films).toBeDisabled();
    expect(within(media()).queryByRole('radio', { name: 'Nur Serien' })).toBeNull();

    await user.click(within(card).getByRole('radio', { name: 'Trending bei Trakt' }));
    const radios = within(media()).getAllByRole('radio');
    expect(radios.map((radio) => radio.closest('label')?.textContent)).toEqual([
      'Nur Filme',
      'Nur Serien',
      'Filme und Serien',
    ]);
    for (const radio of radios) expect(radio).toBeEnabled();
  });

  it('the address list asks for its address; a watchlist address comes back as a field error', async () => {
    const { calls } = spotlightApi({
      'PUT /admin/spotlight': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'listUrl', code: 'invalid_value' }],
        }),
    });
    renderApp('/admin/spotlight');
    const user = userEvent.setup();
    const card = await source();

    expect(within(card).queryByRole('textbox', { name: /Adresse der Liste/ })).toBeNull();
    await user.click(within(card).getByRole('radio', { name: 'Öffentliche Liste per Adresse' }));
    const address = within(card).getByRole('textbox', { name: /Adresse der Liste/ });
    expect(address).toBeRequired();
    await user.type(address, 'https://trakt.tv/users/marco/watchlist');
    await user.click(within(card).getByRole('button', { name: 'Speichern' }));

    expect(await within(card).findByText('Ungültiger Wert.')).toBeVisible();
    expect(address).toHaveAttribute('aria-invalid', 'true');
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      enabled: true,
      listKey: 'url',
      listUrl: 'https://trakt.tv/users/marco/watchlist',
      media: 'both',
      itemCount: 20,
      minHits: 5,
      intervalHours: 24,
    });
  });

  it('"Speichern" stores the switch of the page head with the source in one PUT', async () => {
    const { calls } = spotlightApi({
      'PUT /admin/spotlight': (call: Call) =>
        json({ ...config, ...(call.body as object) } satisfies SpotlightConfig),
    });
    renderApp('/admin/spotlight');
    const user = userEvent.setup();
    const card = await source();

    const show = await screen.findByRole('switch', { name: 'Auf der Startseite zeigen' });
    await user.click(show);
    // Nothing is stored before "Speichern".
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
    await user.click(within(card).getByRole('radio', { name: 'Beliebt bei Trakt' }));
    await user.click(within(card).getByRole('radio', { name: 'Nur Serien' }));
    const count = within(card).getByLabelText('Höchstens');
    await user.clear(count);
    await user.type(count, '10');
    await user.click(within(card).getByRole('button', { name: 'Speichern' }));

    expect(await within(card).findByText('Spotlight ist gespeichert.')).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      enabled: false,
      listKey: 'popular',
      media: 'show',
      itemCount: 10,
      minHits: 5,
      intervalHours: 24,
    });
    expect(show).not.toBeChecked();
    // Switched off, the card names that as the reason the row is missing.
    expect(await within(card).findByText(/Spotlight ist ausgeschaltet/)).toBeVisible();
  });

  it('hides and shows, pins and unpins, and annotates an entry with one PATCH each', async () => {
    const { calls } = spotlightApi();
    renderApp('/admin/spotlight');
    const user = userEvent.setup();
    const table = await entriesTable();
    const row = () => within(table).getAllByRole('row')[1]!;

    await user.click(within(table).getByRole('button', { name: 'Dune ausblenden' }));
    expect(await screen.findByText('Dune ist ausgeblendet.')).toBeVisible();
    expect(await within(row()).findByText('Ausgeblendet')).toBeVisible();
    // Poster and title of a hidden entry at half strength.
    expect(
      within(row()).getByRole('rowheader').querySelector('img')?.parentElement?.parentElement,
    ).toHaveClass('dimmed');
    const show = await within(table).findByRole('button', { name: 'Dune einblenden' });
    await user.click(show);
    expect(await screen.findByText('Dune wird wieder gezeigt.')).toBeVisible();
    expect(await within(row()).findByText('Sichtbar')).toBeVisible();

    await user.click(within(table).getByRole('button', { name: 'Dune anheften' }));
    let dialog = await screen.findByRole('dialog', { name: 'Dune anheften' });
    await user.type(within(dialog).getByLabelText(/Platz in der Reihe/), '3');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Dune ist auf Platz 3 angeheftet.')).toBeVisible();
    expect(await within(row()).findByText('Angeheftet auf Platz 3')).toBeVisible();
    await user.click(await within(table).findByRole('button', { name: 'Dune lösen' }));
    expect(await screen.findByText('Dune ist nicht mehr angeheftet.')).toBeVisible();
    await waitFor(() => expect(within(row()).queryByText('Angeheftet auf Platz 3')).toBeNull());

    const note = within(table).getByRole('button', { name: 'Notiz zu Dune' });
    await user.click(note);
    dialog = await screen.findByRole('dialog', { name: 'Notiz zu Dune' });
    await user.type(within(dialog).getByRole('textbox', { name: 'Notiz' }), 'Im Kino gesehen');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(await within(row()).findByText('«Im Kino gesehen»')).toBeVisible();

    // An emptied note removes it.
    await user.click(note);
    dialog = await screen.findByRole('dialog', { name: 'Notiz zu Dune' });
    await user.clear(within(dialog).getByRole('textbox', { name: 'Notiz' }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Die Notiz zu Dune ist entfernt.')).toBeVisible();
    await waitFor(() => expect(within(row()).queryByText('«Im Kino gesehen»')).toBeNull());

    const path = '/admin/spotlight/entries/movie/7001';
    expect(patches(calls)).toEqual([
      [path, { hidden: true }],
      [path, { hidden: false }],
      [path, { pinnedRank: 3 }],
      [path, { pinnedRank: null }],
      [path, { note: 'Im Kino gesehen' }],
      [path, { note: null }],
    ]);
  });

  it('a place another entry holds is refused with a notice in the dialog', async () => {
    spotlightApi({ 'PATCH /admin/spotlight/entries/*/*': () => problem(409, 'rank_taken') });
    renderApp('/admin/spotlight');
    const user = userEvent.setup();
    const table = await entriesTable();

    await user.click(within(table).getByRole('button', { name: 'Dune anheften' }));
    const dialog = await screen.findByRole('dialog', { name: 'Dune anheften' });
    await user.type(within(dialog).getByLabelText(/Platz in der Reihe/), '1');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(dialog).findByText('Diesen Platz hat schon ein anderer Eintrag der Liste.'),
    ).toBeVisible();
  });

  it('lists an orphaned override and removes it with an empty PATCH', async () => {
    const { calls } = spotlightApi();
    renderApp('/admin/spotlight');
    const user = userEvent.setup();

    const orphans = await screen.findByRole('table', { name: 'Verwaiste Anpassungen' });
    expect(within(orphans).getByRole('rowheader')).toHaveTextContent('Old Film');
    expect(within(orphans).getByText('Ausgeblendet · «Alt»')).toBeVisible();
    await user.click(
      within(orphans).getByRole('button', { name: 'Anpassung für Old Film entfernen' }),
    );
    expect(await screen.findByText('Die Anpassung für Old Film ist entfernt.')).toBeVisible();
    await waitFor(() =>
      expect(screen.queryByRole('table', { name: 'Verwaiste Anpassungen' })).toBeNull(),
    );
    expect(patches(calls)).toEqual([['/admin/spotlight/entries/movie/9001', {}]]);
  });

  it.each([
    [202, () => new Response(null, { status: 202 }), 'Die Liste wird abgerufen.'],
    [422, () => problem(422, 'provider_not_configured'), /Dieser Anbieter ist nicht eingerichtet/],
    [429, () => problem(429, 'too_many_requests'), /vor weniger als einer Minute abgerufen/],
  ])('"Jetzt abrufen" answered %i shows its notice', async (_status, reply, text) => {
    const { calls } = spotlightApi({ 'POST /admin/spotlight/refresh': reply });
    renderApp('/admin/spotlight');
    const user = userEvent.setup();
    const card = await source();

    await user.click(within(card).getByRole('button', { name: 'Jetzt abrufen' }));
    expect(await within(card).findByText(text)).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/spotlight/refresh')).toBe(true);
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('without a Trakt key the foot says so and links to the integrations', async () => {
    spotlightApi({
      'GET /admin/trakt': () =>
        json({ configured: false, listKey: 'trending', entries: 0 } satisfies TraktStatus),
    });
    renderApp('/admin/spotlight');
    const card = await source();

    expect(await within(card).findByText('Noch nie abgerufen')).toBeVisible();
    expect(within(card).getByRole('link', { name: 'Kein Schlüssel' })).toHaveAttribute(
      'href',
      '/admin/integrations',
    );
    expect(within(card).getByText(/Ohne Trakt-Schlüssel fehlt die Reihe/)).toBeVisible();
    expect(
      within(card).getByRole('link', { name: 'Schlüssel unter Integrationen' }),
    ).toHaveAttribute('href', '/admin/integrations');
  });

  it('a failed fetch shows its reason instead of the time', async () => {
    spotlightApi({
      'GET /admin/trakt': () =>
        json({ ...status, lastError: 'trakt_unavailable: HTTP 503' } satisfies TraktStatus),
    });
    renderApp('/admin/spotlight');
    const card = await source();

    expect(
      await within(card).findByText('Letzter Abruf fehlgeschlagen: trakt_unavailable: HTTP 503'),
    ).toBeVisible();
    expect(within(card).queryByText(/^Zuletzt abgerufen /)).toBeNull();
    expect(
      within(card).getByText(/Die Reihe fehlt auf der Startseite, bis ein Abruf gelingt/),
    ).toBeVisible();
  });

  it('an empty list is named as the reason the row is missing', async () => {
    spotlightApi({
      'GET /admin/trakt': () => json({ ...status, entries: 0 } satisfies TraktStatus),
      'GET /admin/spotlight/entries': () => json({ listKey: 'trending', entries: [], orphans: [] }),
    });
    renderApp('/admin/spotlight');
    const card = await source();

    expect(await within(card).findByText(/Die Liste ist leer/)).toBeVisible();
    expect(await screen.findByText(/Noch keine Einträge/)).toBeVisible();
  });
});
