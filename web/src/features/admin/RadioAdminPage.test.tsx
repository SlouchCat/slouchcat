import type { components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { adminShell, renderAdminPage } from '@/test/admin';
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';

// Webradio in the administration (concept 16, 11, variant A): the tabs in `?tab=`, the warning
// without RAM buffer, the relays with «Stoppen», the editor in `?item=`. The server answers; the
// page shows and sends.

type RadioConfig = components['schemas']['RadioConfig'];
type AdminRadioStation = components['schemas']['AdminRadioStation'];
type RadioTagRules = components['schemas']['RadioTagRules'];

const config = (overrides: Partial<RadioConfig> = {}): RadioConfig => ({
  bufferMinutes: 30,
  budgetMb: 512,
  maxRelays: 10,
  graceSeconds: 120,
  hideBroken: true,
  coverMusicbrainz: true,
  coverItunes: true,
  countries: null,
  bufferMode: 'tmpfs',
  maxRelaysByBudget: 11,
  ...overrides,
});

const station: AdminRadioStation = {
  id: 7,
  name: 'Radio Swiss Pop',
  country: 'CH',
  region: 'CH-ZH',
  languages: ['de'],
  genres: ['pop'],
  kinds: ['music'],
  moods: [],
  hls: false,
  showsTitles: true,
  recommended: true,
  state: 'ok',
  source: 'catalog',
  hidden: false,
  failures: 0,
  tags: ['pop', 'swiss'],
  facets: [{ facet: 'genre', value: 'pop', manual: false }],
  locks: ['name'],
  logoState: 'catalog',
  aliasCount: 2,
  streams: [
    {
      id: 70,
      url: 'https://stream.example.test/pop.aac',
      codec: 'AAC',
      bitrateKbps: 128,
      hls: false,
      reachable: true,
      preferred: false,
      rank: 0,
    },
  ],
  createdAt: Date.UTC(2026, 10, 1),
  updatedAt: Date.UTC(2026, 10, 6),
};

/** The rules of «Einordnung»: a default, an own rule, an ignored tag; two tags wait. */
const tagRules = (overrides: Partial<RadioTagRules> = {}): RadioTagRules => ({
  rules: [
    { tag: 'pop', facet: 'genre', value: 'pop', source: 'default', stations: 5200 },
    { tag: 'classic hits', facet: 'genre', value: 'oldies', source: 'default', stations: 300 },
    { tag: 'news', facet: 'kind', value: 'news', source: 'admin', stations: 900 },
    { tag: 'fm', facet: 'ignore', value: null, source: 'admin', stations: 4100 },
  ],
  worklist: [
    { tag: '80s', stations: 2105 },
    { tag: 'local', stations: 40 },
  ],
  worklistMinStations: 20,
  openTags: 1,
  coverage: [
    { facet: 'genre', assigned: 35410, total: 38912 },
    { facet: 'kind', assigned: 26000, total: 38912 },
    { facet: 'mood', assigned: 9840, total: 38912 },
    { facet: 'region', assigned: 26100, total: 38912 },
    { facet: 'language', assigned: 37200, total: 38912 },
  ],
  ...overrides,
});

/** An own station (contract 1.19.0): one stream, no catalog tags. */
const own: AdminRadioStation = {
  ...station,
  id: 9,
  name: 'Quartierradio Lorraine',
  source: 'own',
  region: undefined,
  recommended: false,
  homepage: 'https://lorraine.example.test',
  tags: [],
  facets: [],
  locks: [],
  logoState: 'none',
  aliasCount: 0,
  streams: [
    { ...station.streams[0]!, id: 90, url: 'https://203.0.113.10/live.mp3', preferred: true },
  ],
};

function radioRoutes(overrides: Routes = {}): Routes {
  return {
    ...signedIn,
    ...adminShell,
    'GET /admin/radio/config': () => json(config()),
    'GET /admin/radio/catalog': () =>
      json({
        source: 'radio-browser',
        lastSyncAt: Date.UTC(2026, 10, 7, 3, 30),
        syncFailures: 0,
        entriesTotal: 51240,
        entriesReachable: 38912,
        stationsTotal: 40102,
        lastAdded: 214,
        lastRemoved: 88,
      }),
    'GET /admin/radio/relays': () =>
      json({
        relays: [
          {
            stationId: 7,
            name: 'Radio Swiss Pop',
            codec: 'AAC',
            bitrateKbps: 128,
            transcoded: false,
            startedAt: Date.UTC(2026, 10, 7, 8),
            bufferSeconds: 600,
            bufferBytes: 9_600_000,
            state: 'live',
            listeners: [
              {
                listenId: 'L1',
                profileName: 'Lena',
                accountEmail: 'gast@example.test',
                playing: true,
                since: Date.UTC(2026, 10, 7, 8),
              },
            ],
          },
        ],
        load: {
          relays: 1,
          maxRelays: 10,
          bufferBytes: 9_600_000,
          budgetBytes: 512_000_000,
          uploadKbps: 128,
          bufferMode: 'tmpfs',
        },
      }),
    'GET /admin/radio/stations': () => json({ items: [station], total: 1, offset: 0 }),
    'GET /admin/radio/stations/7': () => json(station),
    'GET /admin/radio/stations/9': () => json(own),
    'GET /admin/radio/tag-rules': () => json(tagRules()),
    'GET /radio/facets': () =>
      json({
        country: [{ value: 'CH', count: 1 }],
        region: [],
        language: [],
        genre: [{ value: 'pop', count: 1 }],
        kind: [],
        mood: [],
        quality: [],
        titles: 1,
      }),
    ...overrides,
  };
}

describe('radio administration', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('opens on «Übersicht» with the counters, the catalog and the relays', async () => {
    mockApi(radioRoutes());
    await renderAdminPage('/admin/radio');
    expect(await screen.findByRole('heading', { level: 1, name: 'Radio' })).toBeVisible();
    expect(screen.getByRole('tab', { name: /Übersicht/, selected: true })).toBeVisible();
    const strip = screen.getByRole('group', { name: 'Zusammenfassung' });
    expect(await within(strip).findByText('1 · 1')).toBeVisible();
    expect(
      await screen.findByText(/51.240 Einträge|51’240 Einträge|51'240 Einträge/),
    ).toBeVisible();
    const relays = await screen.findByTestId('radio-relays');
    expect(within(relays).getByRole('button', { name: 'Hören von Lena stoppen' })).toBeVisible();
    // With the RAM disk there is no warning.
    expect(screen.queryByText('Kein RAM-Puffer')).not.toBeInTheDocument();
  });

  it('warns when the relays have no RAM buffer', async () => {
    mockApi(
      radioRoutes({ 'GET /admin/radio/config': () => json(config({ bufferMode: 'memory' })) }),
    );
    await renderAdminPage('/admin/radio');
    expect(await screen.findByText('Kein RAM-Puffer')).toBeVisible();
  });

  it('takes the tab from ?tab= and writes it back', async () => {
    mockApi(radioRoutes());
    const { router } = await renderAdminPage('/admin/radio?tab=einstellungen');
    expect(await screen.findByRole('tab', { name: /Einstellungen/, selected: true })).toBeVisible();
    expect(await screen.findByRole('group', { name: 'Zeitversatz' })).toBeVisible();
    expect(
      screen.getByText(/Das Budget erlaubt beim jetzigen Zeitversatz 11 Sender/),
    ).toBeVisible();

    await userEvent.setup().click(screen.getByRole('tab', { name: /^Sender$/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'sender' }));
    const table = await screen.findByRole('table', { name: 'Sender' });
    expect(within(table).getByRole('rowheader', { name: /Radio Swiss Pop/ })).toBeVisible();
  });

  it('stops a listener after the confirmation', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'DELETE /admin/radio/listeners/L1': () => new Response(null, { status: 204 }),
      }),
    );
    await renderAdminPage('/admin/radio');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Hören von Lena stoppen' }));
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Stoppen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/radio/listeners/L1'),
      ).toBe(true),
    );
  });

  it('opens the editor from ?item= and saves a field with its lock', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'PATCH /admin/radio/stations/7': (call) => json({ ...station, ...(call.body as object) }),
      }),
    );
    // The open editor hides the page (and its sidebar) from assistive technology.
    renderApp('/admin/radio?tab=sender&item=7');
    const dialog = await screen.findByRole('dialog');
    const user = userEvent.setup();
    const region = await within(dialog).findByRole('textbox', { name: 'Region' });
    await user.clear(region);
    await user.type(region, 'ch-be');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => {
      const patch = calls.find((call) => call.method === 'PATCH');
      expect(patch?.body).toEqual({ region: 'CH-BE', locks: ['name', 'region'] });
    });
  });

  it('shows the coverage and the open tags on «Übersicht» and names them in the strip', async () => {
    mockApi(radioRoutes());
    const { router } = await renderAdminPage('/admin/radio');
    const coverage = await screen.findByRole('list', { name: 'Abdeckung je Filter' });
    expect(within(coverage).getAllByRole('listitem')).toHaveLength(5);
    expect(within(coverage).getAllByRole('listitem')[0]).toHaveTextContent(/Genre.*91\s%/);
    expect(screen.getByTestId('radio-open-tags')).toHaveTextContent('2 Schlagworte ohne Zuordnung');
    const strip = screen.getByRole('group', { name: 'Zusammenfassung' });
    expect(within(strip).getByText('Schlagworte ohne Zuordnung')).toBeVisible();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Zur Einordnung' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'einordnung' }));
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: /Einordnung/, selected: true })).toHaveFocus(),
    );
  });

  it('leaves the strip without open tags when none wait', async () => {
    mockApi(radioRoutes({ 'GET /admin/radio/tag-rules': () => json(tagRules({ openTags: 0 })) }));
    await renderAdminPage('/admin/radio');
    await screen.findByTestId('radio-open-tags');
    const strip = screen.getByRole('group', { name: 'Zusammenfassung' });
    expect(within(strip).queryByText('Schlagworte ohne Zuordnung')).toBeNull();
  });

  it('assigns a tag of the worklist and sends the whole list of own rules', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'PUT /admin/radio/tag-rules': () =>
          json(tagRules({ worklist: [{ tag: 'local', stations: 40 }], openTags: 0 })),
      }),
    );
    await renderAdminPage('/admin/radio?tab=einordnung');
    const worklist = await screen.findByTestId('radio-worklist');
    const user = userEvent.setup();
    const assign = within(worklist).getByRole('button', { name: 'Schlagwort 80s zuordnen' });
    expect(assign).toBeDisabled();
    await user.selectOptions(
      within(worklist).getByRole('combobox', { name: 'Merkmal für das Schlagwort 80s' }),
      'genre:oldies',
    );
    await user.click(assign);
    await waitFor(() => {
      const put = calls.find((call) => call.method === 'PUT');
      expect(put?.body).toEqual({
        rules: [
          { tag: 'news', facet: 'kind', value: 'news' },
          { tag: 'fm', facet: 'ignore', value: null },
          { tag: '80s', facet: 'genre', value: 'oldies' },
        ],
      });
    });
    expect(await screen.findByText(/«80s» ist jetzt Genre: Oldies/)).toBeVisible();
    await waitFor(() =>
      expect(within(worklist).queryByRole('rowheader', { name: '80s' })).toBeNull(),
    );
  });

  it('ignores a tag that means nothing (D-1)', async () => {
    const { calls } = mockApi(
      radioRoutes({ 'PUT /admin/radio/tag-rules': () => json(tagRules()) }),
    );
    await renderAdminPage('/admin/radio?tab=einordnung');
    const worklist = await screen.findByTestId('radio-worklist');
    const user = userEvent.setup();
    await user.selectOptions(
      within(worklist).getByRole('combobox', { name: 'Merkmal für das Schlagwort local' }),
      'ignore',
    );
    await user.click(within(worklist).getByRole('button', { name: 'Schlagwort local zuordnen' }));
    await waitFor(() => {
      const put = calls.find((call) => call.method === 'PUT');
      expect((put?.body as { rules: unknown[] }).rules).toContainEqual({
        tag: 'local',
        facet: 'ignore',
        value: null,
      });
    });
  });

  it('overrides a default per facet, «Keins» included, and removes an own rule after asking', async () => {
    const { calls } = mockApi(
      radioRoutes({ 'PUT /admin/radio/tag-rules': () => json(tagRules()) }),
    );
    await renderAdminPage('/admin/radio?tab=einordnung');
    const rules = await screen.findByTestId('radio-rules');
    const user = userEvent.setup();
    // The default rule has no «Entfernen»; changing its value makes it an own rule.
    expect(within(rules).queryByRole('button', { name: /für pop entfernen/ })).toBeNull();
    const apply = within(rules).getByRole('button', {
      name: 'Regel Genre für pop übernehmen',
    });
    expect(apply).toBeDisabled();
    // A choice alone saves nothing (the arrow keys of a closed select pass every value).
    await user.selectOptions(
      within(rules).getByRole('combobox', { name: 'Genre für das Schlagwort pop' }),
      'genre:rock',
    );
    await user.selectOptions(
      within(rules).getByRole('combobox', { name: 'Genre für das Schlagwort pop' }),
      'genre:',
    );
    expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(0);
    await user.click(apply);
    await waitFor(() => {
      const put = calls.findLast((call) => call.method === 'PUT');
      expect((put?.body as { rules: unknown[] }).rules).toContainEqual({
        tag: 'pop',
        facet: 'genre',
        value: null,
      });
    });
    const remove = within(rules).getByRole('button', { name: 'Regel Sendeart für news entfernen' });
    // Every save waits for the one before it.
    await waitFor(() => expect(remove).toBeEnabled());
    await user.click(remove);
    const dialog = await screen.findByRole('alertdialog', { name: 'Regel für «news» entfernen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Entfernen' }));
    await waitFor(() => {
      const put = calls.findLast((call) => call.method === 'PUT');
      expect(put?.body).toEqual({ rules: [{ tag: 'fm', facet: 'ignore', value: null }] });
    });
  });

  it('finds rules by tag or by the name of their value', async () => {
    mockApi(radioRoutes());
    await renderAdminPage('/admin/radio?tab=einordnung');
    const rules = await screen.findByTestId('radio-rules');
    const user = userEvent.setup();
    const search = within(rules).getByRole('textbox', { name: 'Regeln durchsuchen' });
    await user.type(search, 'fm');
    await waitFor(() => expect(within(rules).queryByRole('rowheader', { name: 'pop' })).toBeNull());
    expect(within(rules).getByRole('rowheader', { name: 'fm' })).toBeVisible();
    await user.clear(search);
    // The value as the page names it: genre «oldies» reads «Oldies».
    await user.type(search, 'Oldie');
    await waitFor(() => expect(within(rules).queryByRole('rowheader', { name: 'fm' })).toBeNull());
    expect(within(rules).getByRole('rowheader', { name: 'classic hits' })).toBeVisible();
  });

  it('lists the own stations and refuses a stream address in the local network at its field', async () => {
    const { calls, fetchMock } = mockApi(
      radioRoutes({
        'GET /admin/radio/stations': () => json({ items: [own], total: 1, offset: 0 }),
        'POST /admin/radio/stations': () =>
          problem(422, 'validation_failed', {
            errors: [{ field: 'streamUrl', code: 'local_address' }],
          }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=eigene');
    const table = await screen.findByRole('table', { name: 'Eigene Sender' });
    expect(within(table).getByRole('rowheader', { name: /Quartierradio Lorraine/ })).toBeVisible();
    expect(within(table).getByText('https://203.0.113.10/live.mp3')).toBeVisible();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Eigenen Sender hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Eigenen Sender hinzufügen' });
    await user.type(within(dialog).getByRole('textbox', { name: 'Name' }), 'Heimradio');
    await user.type(
      within(dialog).getByRole('textbox', { name: 'Stream-Adresse' }),
      'http://192.168.1.20/live.mp3',
    );
    await user.type(within(dialog).getByRole('textbox', { name: 'Sprachen' }), 'DE, fr, xyz');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    expect(
      await within(dialog).findByText(
        'Diese Adresse liegt im lokalen Netz. Auch eigene Sender müssen aus dem Internet kommen.',
      ),
    ).toBeVisible();
    const post = calls.find((call) => call.method === 'POST');
    expect(post?.body).toEqual({
      name: 'Heimradio',
      streamUrl: 'http://192.168.1.20/live.mp3',
      languages: ['de', 'fr'],
    });
    // The tab asks for the own stations only.
    const asked = fetchMock.mock.calls
      .map(([request]) => new URL(request.url))
      .filter((url) => url.pathname.endsWith('/admin/radio/stations'));
    expect(asked.some((url) => url.searchParams.get('source') === 'own')).toBe(true);
  });

  it('adds an own station and opens its editor for the logo', async () => {
    mockApi(
      radioRoutes({
        'GET /admin/radio/stations': () => json({ items: [], total: 0, offset: 0 }),
        'POST /admin/radio/stations': () => json(own, 201),
      }),
    );
    const { router } = await renderAdminPage('/admin/radio?tab=eigene');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Eigenen Sender hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Eigenen Sender hinzufügen' });
    await user.type(
      within(dialog).getByRole('textbox', { name: 'Name' }),
      'Quartierradio Lorraine',
    );
    await user.type(
      within(dialog).getByRole('textbox', { name: 'Stream-Adresse' }),
      'https://203.0.113.10/live.mp3',
    );
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'eigene', item: '9' }));
    const editor = await screen.findByRole('dialog', { name: /Quartierradio Lorraine/ });
    expect(await within(editor).findByRole('textbox', { name: 'Stream-Adresse' })).toHaveValue(
      'https://203.0.113.10/live.mp3',
    );
  });

  it('changes the fields of an own station without a lock; what stays the same is not sent', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'GET /admin/radio/stations': () => json({ items: [own], total: 1, offset: 0 }),
        'PATCH /admin/radio/stations/9': (call) => json({ ...own, ...(call.body as object) }),
      }),
    );
    renderApp('/admin/radio?tab=eigene&item=9');
    const editor = await screen.findByRole('dialog');
    const user = userEvent.setup();
    const homepage = await within(editor).findByRole('textbox', { name: 'Webseite' });
    await user.clear(homepage);
    const country = within(editor).getByRole('textbox', { name: 'Land' });
    await user.clear(country);
    // The same country in lower case is no change; the cleared homepage is (null clears it).
    await user.type(country, 'ch');
    const languages = within(editor).getByRole('textbox', { name: 'Sprachen' });
    await user.clear(languages);
    await user.type(languages, 'de, rm');
    await user.click(within(editor).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => {
      const patch = calls.find((call) => call.method === 'PATCH');
      expect(patch?.body).toEqual({ homepage: null, languages: ['de', 'rm'] });
    });
  });

  it('deletes an own station after the confirmation', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'GET /admin/radio/stations': () => json({ items: [own], total: 1, offset: 0 }),
        'DELETE /admin/radio/stations/9': () => new Response(null, { status: 204 }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=eigene');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Quartierradio Lorraine löschen' }));
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Quartierradio Lorraine löschen?',
    });
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/radio/stations/9'),
      ).toBe(true),
    );
    expect(await screen.findByText('Quartierradio Lorraine ist gelöscht.')).toBeVisible();
  });

  it('switches the two cover lookups in «Einstellungen»', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'PATCH /admin/radio/config': (call) => json({ ...config(), ...(call.body as object) }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=einstellungen');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('switch', { name: 'Cover-Suche iTunes' }));
    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await waitFor(() => {
      const patch = calls.find((call) => call.method === 'PATCH');
      expect(patch?.body).toEqual({ coverItunes: false });
    });
    expect(screen.getByRole('switch', { name: 'Cover-Suche MusicBrainz' })).toBeChecked();
  });

  it('saves one rule at a time: the next assign waits and builds on the answer before it', async () => {
    let answerFirst: (response: Response) => void = () => undefined;
    let puts = 0;
    const afterFirst = tagRules({
      rules: [
        ...tagRules().rules,
        { tag: '80s', facet: 'genre', value: 'oldies', source: 'admin', stations: 2105 },
      ],
      worklist: [{ tag: 'local', stations: 40 }],
    });
    const { calls } = mockApi(
      radioRoutes({
        'PUT /admin/radio/tag-rules': () => {
          puts += 1;
          if (puts > 1) return json(afterFirst);
          return new Promise<Response>((resolve) => {
            answerFirst = resolve;
          });
        },
      }),
    );
    await renderAdminPage('/admin/radio?tab=einordnung');
    const worklist = await screen.findByTestId('radio-worklist');
    const user = userEvent.setup();
    await user.selectOptions(
      within(worklist).getByRole('combobox', { name: 'Merkmal für das Schlagwort 80s' }),
      'genre:oldies',
    );
    await user.selectOptions(
      within(worklist).getByRole('combobox', { name: 'Merkmal für das Schlagwort local' }),
      'ignore',
    );
    const second = () =>
      within(worklist).getByRole('button', { name: 'Schlagwort local zuordnen' });
    await user.click(within(worklist).getByRole('button', { name: 'Schlagwort 80s zuordnen' }));
    // While the first is on its way the other saves wait.
    await waitFor(() => expect(second()).toBeDisabled());
    expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(1);
    answerFirst(json(afterFirst));
    // The answer took «80s» off the list; «local» may be saved now.
    await waitFor(() =>
      expect(within(worklist).queryByRole('rowheader', { name: '80s' })).toBeNull(),
    );
    await waitFor(() => expect(second()).toBeEnabled());
    await user.click(second());
    await waitFor(() => expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(2));
    // The second list holds the first's rule: built from the answer, nothing lost.
    expect(calls.filter((call) => call.method === 'PUT')[1]?.body).toEqual({
      rules: [
        { tag: 'news', facet: 'kind', value: 'news' },
        { tag: 'fm', facet: 'ignore', value: null },
        { tag: '80s', facet: 'genre', value: 'oldies' },
        { tag: 'local', facet: 'ignore', value: null },
      ],
    });
  });
});
