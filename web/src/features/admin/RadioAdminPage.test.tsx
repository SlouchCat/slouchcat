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
type RadioOffer = components['schemas']['RadioOffer'];
type RadioOfferStep = components['schemas']['RadioOfferStep'];
type RadioOfferCountry = components['schemas']['RadioOfferCountry'];
type RadioOfferStation = components['schemas']['RadioOfferStation'];

const config = (overrides: Partial<RadioConfig> = {}): RadioConfig => ({
  bufferMinutes: 30,
  budgetMb: 512,
  maxRelays: 10,
  graceSeconds: 120,
  coverMusicbrainz: true,
  coverItunes: true,
  wikidataEnabled: true,
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
  offered: true,
  failures: 0,
  tags: ['pop', 'swiss'],
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
  sources: {
    name: 'admin',
    country: 'catalog',
    region: 'catalog',
    languages: 'catalog',
    logo: 'catalog',
  },
};

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
  locks: [],
  logoState: 'none',
  aliasCount: 0,
  streams: [
    { ...station.streams[0]!, id: 90, url: 'https://203.0.113.10/live.mp3', preferred: true },
  ],
};

const totals = (overrides: Partial<RadioOffer['totals']> = {}): RadioOffer['totals'] => ({
  offered: 3,
  offeredUnreachable: 1,
  countries: 1,
  marked: 0,
  demarked: 0,
  own: 0,
  withLogo: 2,
  removableUnreachable: 1,
  ...overrides,
});

const switzerland: RadioOfferCountry = {
  code: 'CH',
  continent: 'europe',
  mode: 'all',
  offered: 3,
  total: 3,
  unreachable: 1,
};
const liechtenstein: RadioOfferCountry = {
  code: 'LI',
  continent: 'europe',
  offered: 0,
  total: 2,
  unreachable: 0,
};
const usa: RadioOfferCountry = {
  code: 'US',
  continent: 'northAmerica',
  offered: 0,
  total: 5,
  unreachable: 0,
};

/** The stored offer: Switzerland with «Alle», version 4. */
const offer = (overrides: Partial<RadioOffer> = {}): RadioOffer => ({
  version: 4,
  hideBroken: true,
  totals: totals(),
  countries: [switzerland, liechtenstein, usa],
  ...overrides,
});

const vaduz: RadioOfferStation = {
  id: 21,
  name: 'Radio Vaduz',
  tags: ['pop'],
  broken: false,
  offered: true,
};
const schaan: RadioOfferStation = {
  id: 22,
  name: 'Radio Schaan',
  tags: [],
  broken: true,
  offered: true,
};

/**
 * A stand-in for the server's preview (the test does not know the rule either): choosing
 * Liechtenstein brings its two stations in, a `demark` of Radio Vaduz takes it out again.
 */
function previewOf(body: unknown, version = 4) {
  const request = body as {
    steps: RadioOfferStep[];
    list?: { country: string; offset?: number };
  };
  const chosen = request.steps.some((step) => step.op === 'chooseCountry' && step.country === 'LI');
  const vaduzOut = request.steps.some(
    (step) => step.op === 'demark' && step.stationIds?.includes(21),
  );
  const liOffered = chosen ? (vaduzOut ? 1 : 2) : 0;
  return {
    version,
    hideBroken: true,
    totals: totals({ offered: 3 + liOffered, countries: chosen ? 2 : 1 }),
    diff: { added: liOffered, removed: 0 },
    countries: [
      switzerland,
      chosen ? { ...liechtenstein, mode: 'all', offered: liOffered } : liechtenstein,
      usa,
    ],
    ...(request.list
      ? {
          list: {
            total: request.list.country === 'LI' ? 2 : 0,
            offset: request.list.offset ?? 0,
            items: request.list.country === 'LI' ? [{ ...vaduz, offered: !vaduzOut }, schaan] : [],
          },
        }
      : {}),
  };
}

function radioRoutes(overrides: Routes = {}): Routes {
  return {
    ...signedIn,
    ...adminShell,
    'GET /admin/radio/config': () => json(config()),
    'GET /admin/radio/offer': () => json(offer()),
    'POST /admin/radio/offer/preview': (call) => json(previewOf(call.body)),
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
        offerEmpty: false,
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

  it('switches «Wikidata verwenden» in «Einstellungen»', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'PATCH /admin/radio/config': (call) => json({ ...config(), ...(call.body as object) }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=einstellungen');
    const user = userEvent.setup();
    const wikidata = await screen.findByRole('switch', { name: 'Wikidata verwenden' });
    expect(wikidata).toBeChecked();
    await user.click(wikidata);
    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await waitFor(() => {
      const patch = calls.find((call) => call.method === 'PATCH');
      expect(patch?.body).toEqual({ wikidataEnabled: false });
    });
  });

  it('shows the details of a station with the origin of each value and Wikidata (stage B2)', async () => {
    mockApi(
      radioRoutes({
        'GET /admin/radio/stations': () =>
          json({
            items: [
              {
                ...station,
                provider: 'Schweizer Radio und Fernsehen',
                sources: { ...station.sources, provider: 'wikidata' },
                wikidata: {
                  id: 'Q689725',
                  ownerId: 'Q651410',
                  owner: 'Schweizer Radio und Fernsehen',
                  founded: '1983-08',
                  fetchedAt: Date.UTC(2026, 10, 11),
                },
              },
              { ...own, offered: true, sources: { name: 'own', homepage: 'own' } },
            ],
            total: 2,
            offset: 0,
          }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=sender');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Angaben zu Radio Swiss Pop' }));
    const dialog = await screen.findByRole('dialog', { name: 'Angaben zu Radio Swiss Pop' });
    const fact = (term: string) => within(dialog).getByText(term).nextElementSibling;
    expect(fact('Name')).toHaveTextContent('Radio Swiss PopHerkunft: von Hand');
    expect(fact('Land')).toHaveTextContent('Herkunft: Katalog (Radio Browser)');
    expect(fact('Anbieter')).toHaveTextContent('Schweizer Radio und FernsehenHerkunft: Wikidata');
    expect(fact('Gründung')).toHaveTextContent('August 1983Herkunft: Wikidata');
    expect(fact('Wikidata-Kennung')).toHaveTextContent('Q689725');
    expect(within(dialog).getByText('https://stream.example.test/pop.aac')).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Schliessen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

    await user.click(screen.getByRole('button', { name: 'Angaben zu Quartierradio Lorraine' }));
    const second = await screen.findByRole('dialog', { name: 'Angaben zu Quartierradio Lorraine' });
    expect(within(second).getByText('Webseite').nextElementSibling).toHaveTextContent(
      'Herkunft: eigener Sender',
    );
    expect(within(second).queryByText('Gründung')).toBeNull();
    expect(
      within(second).getByText(
        'Wikidata kennt diesen Sender nicht eindeutig: kein Eigentümer, keine Gründung.',
      ),
    ).toBeVisible();
  });

  it('shows the offer in the strip and points to «Angebot» while it is empty', async () => {
    mockApi(
      radioRoutes({
        'GET /admin/radio/offer': () =>
          json(
            offer({
              totals: totals({ offered: 0, offeredUnreachable: 0, countries: 0, withLogo: 0 }),
              countries: [{ ...switzerland, mode: undefined, offered: 0, unreachable: 0 }],
            }),
          ),
      }),
    );
    await renderAdminPage('/admin/radio');
    const strip = screen.getByRole('group', { name: 'Zusammenfassung' });
    expect(await within(strip).findByText('0 · 0')).toBeVisible();
    expect(await screen.findByText('Noch keine Sender im Angebot')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zum Angebot' })).toHaveAttribute(
      'href',
      '/admin/radio?tab=angebot',
    );
  });

  it('sends the steps of «Angebot» to the preview and applies them with the version', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'POST /admin/radio/offer/apply': () =>
          json({ offer: offer({ version: 5 }), queuedLogos: 1, demarkedStationIds: [] }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=angebot');
    const user = userEvent.setup();
    await user.type(await screen.findByRole('textbox', { name: 'Land suchen' }), 'liech');
    const hits = await screen.findByRole('table', { name: 'Gefundene Länder' });
    await user.click(within(hits).getByRole('checkbox', { name: 'Liechtenstein wählen' }));
    const previews = () =>
      calls
        .filter((call) => call.method === 'POST' && call.path === '/admin/radio/offer/preview')
        .map((call) => call.body as { steps: RadioOfferStep[]; list?: { country: string } });
    await waitFor(() =>
      expect(previews()).toContainEqual({ steps: [{ op: 'chooseCountry', country: 'LI' }] }),
    );
    // The bar shows the difference the server computed.
    expect(await screen.findByText('2 Sender kommen dazu.')).toBeVisible();

    // The chosen country opens its station list, read from the preview with the draft.
    const chosen = await screen.findByRole('table', { name: 'Gewählte Länder' });
    await user.click(
      await within(chosen).findByRole('button', { name: 'Sender in Liechtenstein wählen' }),
    );
    const list = await screen.findByRole('table', { name: 'Sender in Liechtenstein' });
    expect(previews().some((body) => body.list?.country === 'LI')).toBe(true);
    expect(within(list).getByText('nicht erreichbar')).toBeVisible();
    const tick = within(list).getByRole('checkbox', { name: 'Radio Vaduz im Angebot' });
    expect(tick).toBeChecked();
    await user.click(tick);
    // The tick follows the input at once and the server's list after the preview.
    expect(tick).not.toBeChecked();
    await waitFor(() =>
      expect(previews()).toContainEqual({
        steps: [
          { op: 'chooseCountry', country: 'LI' },
          { op: 'demark', stationIds: [21] },
        ],
      }),
    );
    expect(await screen.findByText('1 Sender kommt dazu.')).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'Übernehmen' }));
    await waitFor(() => {
      const apply = calls.find((call) => call.path === '/admin/radio/offer/apply');
      expect(apply?.body).toEqual({
        version: 4,
        steps: [
          { op: 'chooseCountry', country: 'LI' },
          { op: 'demark', stationIds: [21] },
        ],
      });
    });
    expect(
      await screen.findByText(
        'Übernommen. Die Radioseite zeigt das neue Angebot sofort. Logos und weitere Angaben lädt der Server jetzt im Hintergrund.',
      ),
    ).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Verwerfen' })).not.toBeInTheDocument();
  });

  it('computes the draft anew on 409 and applies it on the new version (PB-11)', async () => {
    // Another tab stored meanwhile: the offer is at version 5 now.
    let stored = 4;
    let applies = 0;
    const { calls } = mockApi(
      radioRoutes({
        'GET /admin/radio/offer': () => json(offer({ version: stored })),
        'POST /admin/radio/offer/preview': (call) => json(previewOf(call.body, stored)),
        'POST /admin/radio/offer/apply': (call) => {
          applies += 1;
          if (applies === 1) {
            stored = 5;
            return problem(409, 'offer_conflict');
          }
          const body = call.body as { version: number };
          return body.version === stored
            ? json({ offer: offer({ version: 6 }), queuedLogos: 0, demarkedStationIds: [] })
            : problem(409, 'offer_conflict');
        },
      }),
    );
    await renderAdminPage('/admin/radio?tab=angebot');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /^Europa/ }));
    const countries = await screen.findByRole('table', { name: 'Länder in Europa' });
    await user.click(within(countries).getByRole('checkbox', { name: 'Liechtenstein wählen' }));
    expect(await screen.findByText('2 Sender kommen dazu.')).toBeVisible();
    const steps = [{ op: 'chooseCountry', country: 'LI' }];
    const summaries = () =>
      calls.filter(
        (call) =>
          call.path === '/admin/radio/offer/preview' &&
          JSON.stringify(call.body) === JSON.stringify({ steps }),
      ).length;
    const before = summaries();
    await user.click(screen.getByRole('button', { name: 'Übernehmen' }));
    expect(await screen.findByText(/Inzwischen hat sich das Angebot geändert\./)).toBeVisible();
    // The same steps were computed again on the new offer before the note came.
    expect(summaries()).toBeGreaterThanOrEqual(before + 1);
    await user.click(screen.getByRole('button', { name: 'Übernehmen' }));
    await waitFor(() => {
      const bodies = calls
        .filter((call) => call.path === '/admin/radio/offer/apply')
        .map((call) => call.body);
      expect(bodies).toEqual([
        { version: 4, steps },
        { version: 5, steps },
      ]);
    });
    expect(await screen.findByText(/^Übernommen\./)).toBeVisible();
  });

  it('takes the unreachable stations out from «Sender» and undoes it in steps of 5000 (PB-8)', async () => {
    const removed = Array.from({ length: 6001 }, (_, index) => index + 1000);
    const { calls } = mockApi(
      radioRoutes({
        'GET /admin/radio/offer': () =>
          json(offer({ totals: totals({ removableUnreachable: 6001 }) })),
        'POST /admin/radio/offer/apply': (call) => {
          const body = call.body as { steps: RadioOfferStep[] };
          return json({
            offer: offer({ version: 5 }),
            queuedLogos: 0,
            demarkedStationIds: body.steps[0]?.op === 'demarkUnreachable' ? removed : [],
          });
        },
      }),
    );
    await renderAdminPage('/admin/radio?tab=sender');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /Nicht erreichbare entfernen/ }));
    expect(
      await screen.findByText(/6.?001 nicht erreichbare Sender sind aus dem Angebot genommen\./),
    ).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Rückgängig' }));
    expect(await screen.findByText('Die Sender sind wieder im Angebot.')).toBeVisible();
    const bodies = calls
      .filter((call) => call.path === '/admin/radio/offer/apply')
      .map((call) => call.body as { version: number; steps: RadioOfferStep[] });
    expect(bodies[0]).toEqual({ version: 4, steps: [{ op: 'demarkUnreachable' }] });
    expect(bodies[1]?.steps).toEqual([
      { op: 'mark', stationIds: removed.slice(0, 5000) },
      { op: 'mark', stationIds: removed.slice(5000) },
    ]);
  });

  it('takes one station out of the offer from «Sender» and filters by its provider', async () => {
    const { calls, fetchMock } = mockApi(
      radioRoutes({
        'GET /admin/radio/stations': () =>
          json({ items: [{ ...station, provider: 'srg-ssr.ch' }], total: 1, offset: 0 }),
        'POST /admin/radio/offer/apply': () =>
          json({ offer: offer({ version: 5 }), queuedLogos: 0, demarkedStationIds: [] }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=sender');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: 'Radio Swiss Pop aus dem Angebot nehmen' }),
    );
    await waitFor(() => {
      const apply = calls.find((call) => call.path === '/admin/radio/offer/apply');
      expect(apply?.body).toEqual({ version: 4, steps: [{ op: 'demark', stationIds: [7] }] });
    });
    expect(await screen.findByText('Radio Swiss Pop ist nicht mehr im Angebot.')).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'Nach Anbieter srg-ssr.ch filtern' }));
    expect(screen.getByRole('textbox', { name: 'Anbieter' })).toHaveValue('srg-ssr.ch');
    await waitFor(() => {
      const asked = fetchMock.mock.calls
        .map(([request]) => new URL(request.url))
        .filter((url) => url.pathname.endsWith('/admin/radio/stations'));
      expect(asked.some((url) => url.searchParams.get('provider') === 'srg-ssr.ch')).toBe(true);
    });
  });

  it('keeps a change made while «Übernehmen» is on its way', async () => {
    let answer: (response: Response) => void = () => {};
    const { calls } = mockApi(
      radioRoutes({
        'POST /admin/radio/offer/apply': () =>
          new Promise<Response>((resolve) => {
            answer = resolve;
          }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=angebot');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /^Europa/ }));
    const europe = await screen.findByRole('table', { name: 'Länder in Europa' });
    await user.click(within(europe).getByRole('checkbox', { name: 'Liechtenstein wählen' }));
    expect(await screen.findByText('2 Sender kommen dazu.')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Übernehmen' }));
    await waitFor(() =>
      expect(calls.some((call) => call.path === '/admin/radio/offer/apply')).toBe(true),
    );
    // A tick while the apply is on its way: a step of its own after the sent one.
    await user.click(screen.getByRole('button', { name: /^Nord- und Mittelamerika/ }));
    const america = await screen.findByRole('table', { name: 'Länder in Nord- und Mittelamerika' });
    await user.click(within(america).getByRole('checkbox', { name: 'Vereinigte Staaten wählen' }));
    answer(json({ offer: offer({ version: 5 }), queuedLogos: 0, demarkedStationIds: [] }));
    await waitFor(() => {
      const last = calls.filter((call) => call.path === '/admin/radio/offer/preview').at(-1);
      expect(last?.body).toEqual({ steps: [{ op: 'chooseCountry', country: 'US' }] });
    });
    // The draft holds the later change (the stand-in preview knows no United States).
    expect(screen.getByRole('button', { name: 'Verwerfen' })).toBeVisible();
    const applies = calls.filter((call) => call.path === '/admin/radio/offer/apply');
    expect(applies.map((call) => call.body)).toEqual([
      { version: 4, steps: [{ op: 'chooseCountry', country: 'LI' }] },
    ]);
  });

  it('takes back a country ticked and unticked again, and the switch back at its value', async () => {
    const { calls } = mockApi(radioRoutes());
    await renderAdminPage('/admin/radio?tab=angebot');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /^Europa/ }));
    const europe = await screen.findByRole('table', { name: 'Länder in Europa' });
    const liechtenstein = within(europe).getByRole('checkbox', { name: 'Liechtenstein wählen' });
    await user.click(liechtenstein);
    await user.click(liechtenstein);
    const hide = screen.getByRole('switch', { name: 'Nicht erreichbare weglassen' });
    await user.click(hide);
    await user.click(hide);
    // Nothing left: no removeCountry that would drop the stored exceptions, no switch step.
    expect(await screen.findByText('Keine Änderungen am Angebot.')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Verwerfen' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Übernehmen' })).toBeDisabled();
    const bodies = calls
      .filter((call) => call.path === '/admin/radio/offer/preview')
      .map((call) => JSON.stringify(call.body));
    expect(bodies.some((body) => body.includes('removeCountry'))).toBe(false);
    expect(bodies.some((body) => body.includes('setHideBroken'))).toBe(false);

    // The switch off is one step with its value.
    await user.click(hide);
    await waitFor(() =>
      expect(calls.at(-1)?.body).toEqual({ steps: [{ op: 'setHideBroken', value: false }] }),
    );
  });

  it('sends «Alle markieren» and «Gezeigte demarkieren» as steps of the country', async () => {
    // Liechtenstein is stored with «Alle»; the stand-in preview keeps it so.
    const chosenLi: RadioOfferCountry = { ...liechtenstein, mode: 'all', offered: 2 };
    const { calls } = mockApi(
      radioRoutes({
        'GET /admin/radio/offer': () => json(offer({ countries: [switzerland, chosenLi] })),
        'POST /admin/radio/offer/preview': (call) =>
          json({ ...previewOf(call.body), countries: [switzerland, chosenLi] }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=angebot');
    const user = userEvent.setup();
    const chosen = await screen.findByRole('table', { name: 'Gewählte Länder' });
    await user.click(
      within(chosen).getByRole('button', { name: 'Sender in Liechtenstein wählen' }),
    );
    await user.click(await screen.findByRole('button', { name: 'Alle markieren' }));
    await user.type(screen.getByRole('textbox', { name: 'Sender suchen' }), 'vaduz');
    await user.click(await screen.findByRole('button', { name: /^Gezeigte demarkieren/ }));
    await waitFor(() =>
      expect(
        calls.filter((call) => call.path === '/admin/radio/offer/preview').at(-1)?.body,
      ).toMatchObject({
        steps: [
          { op: 'markAll', country: 'LI' },
          { op: 'demarkMatching', country: 'LI', q: 'vaduz' },
        ],
      }),
    );
  });

  it('discards the draft with «Verwerfen» and asks before leaving with one', async () => {
    mockApi(radioRoutes());
    const { router } = await renderAdminPage('/admin/radio?tab=angebot');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /^Europa/ }));
    const europe = await screen.findByRole('table', { name: 'Länder in Europa' });
    await user.click(within(europe).getByRole('checkbox', { name: 'Liechtenstein wählen' }));
    expect(await screen.findByText('2 Sender kommen dazu.')).toBeVisible();

    void router.navigate({ to: '/admin' });
    const ask = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(ask).getByRole('button', { name: 'Auf der Seite bleiben' }));
    expect(router.state.location.pathname).toBe('/admin/radio');
    // The question went and gave the focus back to the tick it came from.
    await waitFor(() =>
      expect(within(europe).getByRole('checkbox', { name: 'Liechtenstein wählen' })).toHaveFocus(),
    );

    await user.click(screen.getByRole('button', { name: 'Verwerfen' }));
    expect(await screen.findByText('Keine Änderungen am Angebot.')).toBeVisible();
    expect(
      within(screen.getByRole('table', { name: 'Länder in Europa' })).getByRole('checkbox', {
        name: 'Liechtenstein wählen',
      }),
    ).not.toBeChecked();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Radio' })).toHaveFocus(),
    );
    void router.navigate({ to: '/admin' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin'));
  });

  it('applies a step of «Sender» once more on the new version after a 409', async () => {
    let stored = 4;
    const { calls } = mockApi(
      radioRoutes({
        'GET /admin/radio/offer': () => json(offer({ version: stored })),
        'POST /admin/radio/offer/apply': (call) => {
          const body = call.body as { version: number };
          if (body.version !== 5) {
            // Another tab stored right before.
            stored = 5;
            return problem(409, 'offer_conflict');
          }
          return json({ offer: offer({ version: 6 }), queuedLogos: 0, demarkedStationIds: [] });
        },
      }),
    );
    await renderAdminPage('/admin/radio?tab=sender');
    await userEvent
      .setup()
      .click(await screen.findByRole('button', { name: 'Radio Swiss Pop aus dem Angebot nehmen' }));
    expect(await screen.findByText('Radio Swiss Pop ist nicht mehr im Angebot.')).toBeVisible();
    const steps = [{ op: 'demark', stationIds: [7] }];
    expect(
      calls.filter((call) => call.path === '/admin/radio/offer/apply').map((call) => call.body),
    ).toEqual([
      { version: 4, steps },
      { version: 5, steps },
    ]);
  });

  it('stores two presses of «Sender» one after the other', async () => {
    const answers: ((response: Response) => void)[] = [];
    const { calls } = mockApi(
      radioRoutes({
        'POST /admin/radio/offer/apply': () =>
          new Promise<Response>((resolve) => {
            answers.push(resolve);
          }),
      }),
    );
    await renderAdminPage('/admin/radio?tab=sender');
    const user = userEvent.setup();
    const out = await screen.findByRole('button', {
      name: 'Radio Swiss Pop aus dem Angebot nehmen',
    });
    await user.click(out);
    await user.click(out);
    const applied = () => calls.filter((call) => call.path === '/admin/radio/offer/apply');
    await waitFor(() => expect(applied()).toHaveLength(1));
    answers[0]?.(json({ offer: offer({ version: 5 }), queuedLogos: 0, demarkedStationIds: [] }));
    await waitFor(() => expect(applied()).toHaveLength(2));
    answers[1]?.(json({ offer: offer({ version: 6 }), queuedLogos: 0, demarkedStationIds: [] }));
    // Each answer says it anew (the message of the second replaces the first).
    await waitFor(() =>
      expect(screen.getByText('Radio Swiss Pop ist nicht mehr im Angebot.')).toBeVisible(),
    );
  });
});
